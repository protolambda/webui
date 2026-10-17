package session

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Defaults of [Options].
const (
	// DefaultCookieName is the session cookie's name without its prefix;
	// with the default Secure policy the cookie is "__Host-session".
	DefaultCookieName = "session"
	// DefaultIdleTTL ends sessions after this long without use.
	DefaultIdleTTL = 15 * time.Minute
	// DefaultAbsoluteTTL ends sessions this long after their creation,
	// however active.
	DefaultAbsoluteTTL = 12 * time.Hour
	// DefaultMaxSessions bounds the number of live sessions.
	DefaultMaxSessions = 1024
)

const (
	tokenBytes = 32
	// tokenLen is the length of an encoded session token, the cookie value.
	tokenLen  = 43
	csrfBytes = 32
	csrfLen   = 43
	// idBytes is the number of bytes of the token digest in a log ID.
	idBytes = 6
)

var errCollision = errors.New("session: token collision")

// key is the store's key of a session: the SHA-256 digest of its token.
type key [sha256.Size]byte

// logID returns the public log identifier of a session key.
func logID(k key) string {
	return base64.RawURLEncoding.EncodeToString(k[:idBytes])
}

// parseToken returns the key of an encoded session token, if it is one: the
// length is checked before anything is decoded, and the encoding must be the
// canonical one.
func parseToken(value string) (key, bool) {
	if len(value) != tokenLen {
		return key{}, false
	}
	var raw [tokenBytes]byte
	defer clear(raw[:])
	n, err := base64.RawURLEncoding.Strict().Decode(raw[:], []byte(value))
	if err != nil || n != tokenBytes {
		return key{}, false
	}
	return sha256.Sum256(raw[:]), true
}

// Options configures a [Store]. The zero value gives the secure defaults.
type Options[T any] struct {
	// CookieName is the name of the session cookie, without prefix; ""
	// means [DefaultCookieName]. It must be a valid cookie name, and must
	// not start with "__Host-" or "__Secure-": the prefix follows from
	// Insecure.
	CookieName string
	// Insecure drops the Secure attribute and the __Host- prefix of the
	// cookie. Use it only to develop over plain http on localhost.
	Insecure bool
	// StrictSameSite marks the cookie SameSite=Strict instead of
	// SameSite=Lax; see the package documentation.
	StrictSameSite bool

	// IdleTTL ends sessions after this long without use; zero means
	// [DefaultIdleTTL].
	IdleTTL time.Duration
	// AbsoluteTTL ends sessions this long after their creation; zero means
	// [DefaultAbsoluteTTL].
	AbsoluteTTL time.Duration
	// MaxSessions bounds the live sessions: creating one more ends the
	// least recently used. Zero means [DefaultMaxSessions].
	MaxSessions int

	// Passive reports requests that do not count as use of their session,
	// such as polls and event streams: they find the session, but neither
	// restart its idle timer nor protect it from eviction. Nil means every
	// request counts.
	Passive func(r *http.Request) bool
	// OnEnd, if set, is called once when a session of the store ends,
	// however it ends, with the session's log ID and a pointer to its
	// value; after it returns the value is replaced by the zero T. Use it
	// to clear secrets the value refers to. It runs while the session is
	// locked, after the session left the store, and must neither call
	// methods of the session or its store nor panic.
	OnEnd func(id string, data *T)

	// Now returns the current time; nil means [time.Now].
	Now func() time.Time
	// Rand is the source of session tokens and CSRF tokens; nil means
	// crypto/rand. Reads are serialized, so the source need not be safe
	// for concurrent use. Replace it only in tests.
	Rand io.Reader
}

// Store is the set of live sessions of an application, with values of type
// T. Create it with [NewStore]. It is safe for concurrent use.
type Store[T any] struct {
	cookie   cookiePolicy
	idle     time.Duration
	absolute time.Duration
	max      int
	passive  func(*http.Request) bool
	onEnd    func(string, *T)
	now      func() time.Time
	rand     *lockedReader

	// mu guards sessions, the recency list, and the lastUsed, prev, next
	// and listed fields of every session. It is never acquired while a
	// session's own lock is held.
	mu       sync.Mutex
	sessions map[key]*Session[T]
	// head is the most recently used session, tail the least.
	head, tail *Session[T]
}

// NewStore returns an empty store configured by opts. It fails on invalid
// options: a bad cookie name, or a negative duration or count.
func NewStore[T any](opts Options[T]) (*Store[T], error) {
	cookie, err := newCookiePolicy(opts.CookieName, DefaultCookieName, opts.Insecure, opts.StrictSameSite)
	if err != nil {
		return nil, err
	}
	if opts.IdleTTL < 0 || opts.AbsoluteTTL < 0 || opts.MaxSessions < 0 {
		return nil, errNegative
	}
	st := &Store[T]{
		cookie:   cookie,
		idle:     opts.IdleTTL,
		absolute: opts.AbsoluteTTL,
		max:      opts.MaxSessions,
		passive:  opts.Passive,
		onEnd:    opts.OnEnd,
		now:      opts.Now,
		rand:     newLockedReader(opts.Rand),
		sessions: make(map[key]*Session[T]),
	}
	if st.idle == 0 {
		st.idle = DefaultIdleTTL
	}
	if st.absolute == 0 {
		st.absolute = DefaultAbsoluteTTL
	}
	if st.max == 0 {
		st.max = DefaultMaxSessions
	}
	if st.now == nil {
		st.now = time.Now
	}
	return st, nil
}

// CookieName returns the full name of the session cookie, prefix included.
func (st *Store[T]) CookieName() string {
	return st.cookie.name
}

// New creates a session with the value data, ends the session that r's
// cookie names (if any: the session ID rotates, which defeats session
// fixation), and sets the new session's cookie on w. Call it at login,
// before writing the response body. r may be nil when there is no previous
// session to end.
//
// When the store is full, New first drops expired sessions, then ends the
// least recently used ones. It fails only if the source of randomness
// fails (or repeats a token, which only a broken source does); w is then
// left untouched and no session ended.
func (st *Store[T]) New(w http.ResponseWriter, r *http.Request, data T) (*Session[T], error) {
	var raw [tokenBytes + csrfBytes]byte
	defer clear(raw[:])
	if err := st.rand.read(raw[:]); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:tokenBytes])
	k := key(sha256.Sum256(raw[:tokenBytes]))
	csrf := make([]byte, csrfLen)
	base64.RawURLEncoding.Encode(csrf, raw[tokenBytes:])
	now := st.now()
	s := &Session[T]{
		store:    st,
		key:      k,
		id:       logID(k),
		created:  now,
		lastUsed: now,
		csrf:     csrf,
		data:     data,
		done:     make(chan struct{}),
	}
	prev, hasPrev := parseToken(st.cookie.value(r))

	var ended []*Session[T]
	st.mu.Lock()
	if _, dup := st.sessions[k]; dup {
		st.mu.Unlock()
		clear(csrf)
		return nil, errCollision
	}
	if hasPrev {
		if old := st.sessions[prev]; old != nil {
			st.unlinkLocked(old)
			ended = append(ended, old)
		}
	}
	if len(st.sessions) >= st.max {
		ended = st.sweepLocked(now, ended)
		for len(st.sessions) >= st.max {
			lru := st.tail
			st.unlinkLocked(lru)
			ended = append(ended, lru)
		}
	}
	st.pushFrontLocked(s)
	st.mu.Unlock()

	st.finish(ended)
	http.SetCookie(w, st.cookie.cookie(token, 0))
	return s, nil
}

// Lookup returns the live session that r's cookie names. Unless
// [Options.Passive] reports r as passive, the request counts as use of the
// session. An expired session is ended and not returned. Lookup never
// creates sessions or sets cookies.
func (st *Store[T]) Lookup(r *http.Request) (*Session[T], bool) {
	k, ok := parseToken(st.cookie.value(r))
	if !ok {
		return nil, false
	}
	touch := st.passive == nil || !st.passive(r)
	now := st.now()
	st.mu.Lock()
	s := st.sessions[k]
	if s == nil {
		st.mu.Unlock()
		return nil, false
	}
	if st.expiredLocked(s, now) {
		st.unlinkLocked(s)
		st.mu.Unlock()
		s.finish(st.onEnd)
		return nil, false
	}
	if touch {
		s.lastUsed = now
		st.moveToFrontLocked(s)
	}
	st.mu.Unlock()
	return s, true
}

// Middleware returns a handler that attaches the live session of each
// request's cookie, if any, to the request context ([FromContext]) and
// calls next. Requests without a live session go on unchanged. It never
// sets or clears cookies, and never creates sessions.
func (st *Store[T]) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, ok := st.Lookup(r); ok {
			r = r.WithContext(WithSession(r.Context(), s))
		}
		next.ServeHTTP(w, r)
	})
}

// End ends the session that r's cookie names, if any, and clears the cookie
// on w: the logout. It does not consult [Options.Passive].
func (st *Store[T]) End(w http.ResponseWriter, r *http.Request) {
	if k, ok := parseToken(st.cookie.value(r)); ok {
		st.mu.Lock()
		s := st.sessions[k]
		if s != nil {
			st.unlinkLocked(s)
		}
		st.mu.Unlock()
		if s != nil {
			s.finish(st.onEnd)
		}
	}
	st.ClearCookie(w)
}

// ClearCookie sets a cookie on w that removes the session cookie from the
// browser. It does not end any session.
func (st *Store[T]) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, st.cookie.cookie("", -1))
}

// EndAll ends every session, for example after a change of the
// application's credentials, and returns how many it ended. The store
// stays usable.
func (st *Store[T]) EndAll() int {
	st.mu.Lock()
	ended := make([]*Session[T], 0, len(st.sessions))
	for s := st.head; s != nil; s = s.next {
		ended = append(ended, s)
	}
	for _, s := range ended {
		st.unlinkLocked(s)
	}
	st.mu.Unlock()
	st.finish(ended)
	return len(ended)
}

// EndFunc ends every session whose value match reports true for, for
// example the sessions of one account after its password changed, and
// returns how many it ended. match gets a copy of each value while that
// session is locked; it must not call methods of the session or the store.
func (st *Store[T]) EndFunc(match func(data T) bool) int {
	st.mu.Lock()
	all := make([]*Session[T], 0, len(st.sessions))
	for s := st.head; s != nil; s = s.next {
		all = append(all, s)
	}
	st.mu.Unlock()
	n := 0
	for _, s := range all {
		s.mu.Lock()
		matched := !s.ended && match(s.data)
		s.mu.Unlock()
		if matched && st.end(s) {
			n++
		}
	}
	return n
}

// Sweep ends the expired sessions and returns how many it ended. Expired
// sessions also end when a request presents them; call Sweep periodically
// (every minute, say) so that the others end on time too and their
// [Session.Done] channels close. For example, from a goroutine owned by the
// application:
//
//	ticker := time.NewTicker(time.Minute)
//	defer ticker.Stop()
//	for {
//		select {
//		case <-ctx.Done():
//			return
//		case <-ticker.C:
//			store.Sweep()
//		}
//	}
func (st *Store[T]) Sweep() int {
	now := st.now()
	st.mu.Lock()
	ended := st.sweepLocked(now, nil)
	st.mu.Unlock()
	st.finish(ended)
	return len(ended)
}

// Len returns the number of sessions in the store, including expired
// sessions that were not swept yet.
func (st *Store[T]) Len() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.sessions)
}

// String describes the store without revealing sessions.
func (st *Store[T]) String() string {
	return fmt.Sprintf("session.Store(%s)", st.cookie.name)
}

// GoString is like String.
func (st *Store[T]) GoString() string {
	return st.String()
}

// end removes s from the store, if it is still there, and ends it. It
// reports whether this call ended s.
func (st *Store[T]) end(s *Session[T]) bool {
	st.mu.Lock()
	if s.listed {
		st.unlinkLocked(s)
	}
	st.mu.Unlock()
	return s.finish(st.onEnd)
}

// finish ends sessions that already left the store. The caller must not
// hold st.mu.
func (st *Store[T]) finish(ended []*Session[T]) {
	for _, s := range ended {
		s.finish(st.onEnd)
	}
}

// expiredLocked reports whether s is past its idle or absolute lifetime at
// now. The caller holds st.mu.
func (st *Store[T]) expiredLocked(s *Session[T], now time.Time) bool {
	return now.Sub(s.lastUsed) >= st.idle || now.Sub(s.created) >= st.absolute
}

// sweepLocked removes the expired sessions from the store and appends them
// to ended, for the caller to finish after releasing st.mu.
func (st *Store[T]) sweepLocked(now time.Time, ended []*Session[T]) []*Session[T] {
	for s := st.tail; s != nil; {
		prev := s.prev
		if st.expiredLocked(s, now) {
			st.unlinkLocked(s)
			ended = append(ended, s)
		}
		s = prev
	}
	return ended
}

// pushFrontLocked adds s to the store as its most recently used session.
func (st *Store[T]) pushFrontLocked(s *Session[T]) {
	st.sessions[s.key] = s
	s.listed = true
	s.prev, s.next = nil, st.head
	if st.head != nil {
		st.head.prev = s
	} else {
		st.tail = s
	}
	st.head = s
}

// unlinkLocked removes s from the store.
func (st *Store[T]) unlinkLocked(s *Session[T]) {
	delete(st.sessions, s.key)
	s.listed = false
	if s.prev != nil {
		s.prev.next = s.next
	} else {
		st.head = s.next
	}
	if s.next != nil {
		s.next.prev = s.prev
	} else {
		st.tail = s.prev
	}
	s.prev, s.next = nil, nil
}

// moveToFrontLocked marks s as the most recently used session.
func (st *Store[T]) moveToFrontLocked(s *Session[T]) {
	if st.head == s {
		return
	}
	st.unlinkLocked(s)
	st.pushFrontLocked(s)
}
