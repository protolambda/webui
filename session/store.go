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
	// DefaultMaxSessions bounds the number of live sessions; see
	// [Options.MaxSessions].
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

var (
	errCollision = errors.New("session: token collision")
	errTwoHooks  = errors.New("session: both OnEnd and OnEndReason set")
)

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
	//
	// Both lifetimes are measured on the monotonic and on the wall clock,
	// and whichever advanced more counts: time the computer spends asleep
	// counts too (on some systems the monotonic clock stops then), and
	// setting the wall clock back does not lengthen a session (setting it
	// forward can end sessions early).
	IdleTTL time.Duration
	// AbsoluteTTL ends sessions this long after their creation, however
	// active; zero means [DefaultAbsoluteTTL].
	AbsoluteTTL time.Duration
	// MaxSessions bounds the live sessions: creating one more ends the
	// least recently used, whoever it belongs to. Zero means
	// [DefaultMaxSessions].
	//
	// Anonymous requests cannot fill the store, but logins can: a client
	// that logs in MaxSessions times without sending its session cookie,
	// even to a single account, ends every other session. Size the bound
	// above the expected number of concurrent sessions, throttle
	// successful logins too (per account and per client), and bound the
	// sessions of each account, for example by ending its earlier
	// sessions with [Store.EndFunc] before [Store.New] (see the example of
	// EndFunc). [EndEvicted] in the OnEndReason hook reveals evictions.
	MaxSessions int

	// Passive reports requests that do not count as use of their session,
	// such as polls and event streams: they find the session, but neither
	// restart its idle timer nor protect it from eviction. Nil means every
	// request counts.
	Passive func(r *http.Request) bool
	// OnEnd, if set, is called once when a session of the store ends,
	// however it ends, with the session's log ID and a pointer to its
	// value; after it returns the value is replaced by the zero T. Use it
	// to clear secrets the value refers to. This includes the previous
	// session that [Store.New] ends, even when the new session's value was
	// copied from it; [Store.Rotate] moves a value to a new session without
	// calling OnEnd, which then runs when that session ends. It runs while
	// the session is locked, after the session left the store, and must
	// neither call methods of the session or its store nor panic (a panic
	// still ends the session and the others ended with it, and then goes
	// on to the caller).
	OnEnd func(id string, data *T)
	// OnEndReason is like OnEnd, and also gets the reason the session
	// ended, for audit logs. Set at most one of OnEnd and OnEndReason.
	OnEndReason func(id string, reason EndReason, data *T)

	// Now returns the current time; nil means [time.Now].
	Now func() time.Time
	// Rand is the source of session tokens and CSRF tokens; nil means
	// crypto/rand. Reads are serialized, so the source need not be safe
	// for concurrent use. Replace it only in tests.
	Rand io.Reader
}

// Store is the set of live sessions of an application, with values of type
// T. Create it with [NewStore]. It is safe for concurrent use. Formatting a
// store with any verb prints only its cookie name.
type Store[T any] struct {
	cookie   cookiePolicy
	idle     time.Duration
	absolute time.Duration
	max      int
	passive  func(*http.Request) bool
	onEnd    func(string, EndReason, *T)
	now      func() time.Time
	rand     *lockedReader

	// mu guards sessions, the recency list, and the lastUsed, prev, next
	// and listed fields of every session. It is never acquired while a
	// session's own lock is held; Rotate acquires a session's lock while
	// holding it. A session in the list has not ended: every operation
	// unlinks a session before it ends it.
	mu       sync.Mutex
	sessions map[key]*Session[T]
	// head is the most recently used session, tail the least.
	head, tail *Session[T]
}

// NewStore returns an empty store configured by opts. It fails on invalid
// options: a bad cookie name, a negative duration or count, or both OnEnd
// and OnEndReason.
func NewStore[T any](opts Options[T]) (*Store[T], error) {
	cookie, err := newCookiePolicy(opts.CookieName, DefaultCookieName, opts.Insecure, opts.StrictSameSite)
	if err != nil {
		return nil, err
	}
	if opts.IdleTTL < 0 || opts.AbsoluteTTL < 0 || opts.MaxSessions < 0 {
		return nil, errNegative
	}
	onEnd := opts.OnEndReason
	if opts.OnEnd != nil {
		if onEnd != nil {
			return nil, errTwoHooks
		}
		hook := opts.OnEnd
		onEnd = func(id string, _ EndReason, data *T) { hook(id, data) }
	}
	st := &Store[T]{
		cookie:   cookie,
		idle:     opts.IdleTTL,
		absolute: opts.AbsoluteTTL,
		max:      opts.MaxSessions,
		passive:  opts.Passive,
		onEnd:    onEnd,
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
// cookie names (if any, with the reason [EndRotated] unless it had expired:
// the session ID rotates, which defeats session fixation), and sets the new
// session's cookie on w. Call it at login, before writing the response body. r may
// be nil when there is no previous session to end. For a change of
// privileges within a session, see [Store.Rotate].
//
// When the store is full, New first drops expired sessions, then ends the
// least recently used ones ([EndEvicted]) of any account; see
// [Options.MaxSessions]. It fails only if the source of randomness fails
// (or repeats a token, which only a broken source does); w is then left
// untouched and no session ended.
//
// New takes data as the new session's value and copies nothing. The
// previous session ends as usual, and the OnEnd hook runs on its value: a
// value copied from the previous session with [Session.Data] shares the
// contents of its pointers, slices and maps, which a hook that clears
// secrets then clears for the new session too. Carry a value over with
// [Store.Rotate] instead.
func (st *Store[T]) New(w http.ResponseWriter, r *http.Request, data T) (*Session[T], error) {
	token, k, csrf, err := st.newTokens()
	if err != nil {
		return nil, err
	}
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

	var ended []ending[T]
	st.mu.Lock()
	if _, dup := st.sessions[k]; dup {
		st.mu.Unlock()
		clear(csrf)
		return nil, errCollision
	}
	if hasPrev {
		if old := st.sessions[prev]; old != nil {
			reason := st.expiryLocked(old, now)
			if reason == 0 {
				reason = EndRotated
			}
			st.unlinkLocked(old)
			ended = append(ended, ending[T]{old, reason})
		}
	}
	if len(st.sessions) >= st.max {
		ended = st.sweepLocked(now, ended)
		for len(st.sessions) >= st.max {
			lru := st.tail
			st.unlinkLocked(lru)
			ended = append(ended, ending[T]{lru, EndEvicted})
		}
	}
	st.pushFrontLocked(s)
	st.mu.Unlock()

	st.finish(ended)
	http.SetCookie(w, st.cookie.cookie(token, 0))
	return s, nil
}

// Rotate moves the value of the live session that r's cookie names to a
// new session, with a new session token and a new CSRF token, and sets the
// new session's cookie on w. Call it when the privileges of a session
// change without a new login, such as after a re-authentication or a
// change of role, before writing the response body: the old token and CSRF
// token stop working at once, so one that leaked before the change does
// not gain the new privileges.
//
// The old session ends with the reason [EndRotated] and its Done channel
// closes, but the OnEnd hook does not run for it: the value lives on in the
// returned session, and the hook runs when that one ends. Requests in
// flight with the old session see it ended, and forms rendered with the
// old CSRF token are refused. The new session keeps the creation time of
// the old one, so rotation does not extend the absolute lifetime. A
// [Store.EndFunc] running at the same time checks the new session in place
// of the old one.
//
// Rotate returns [ErrEnded] if r names no live session (an expired one
// ends, as at [Store.Lookup]), and fails like [Store.New] if the source of
// randomness fails; w is then left untouched.
func (st *Store[T]) Rotate(w http.ResponseWriter, r *http.Request) (*Session[T], error) {
	prev, ok := parseToken(st.cookie.value(r))
	if !ok {
		return nil, ErrEnded
	}
	token, k, csrf, err := st.newTokens()
	if err != nil {
		return nil, err
	}
	now := st.now()

	st.mu.Lock()
	if _, dup := st.sessions[k]; dup {
		st.mu.Unlock()
		clear(csrf)
		return nil, errCollision
	}
	old := st.sessions[prev]
	if old == nil {
		st.mu.Unlock()
		clear(csrf)
		return nil, ErrEnded
	}
	if reason := st.expiryLocked(old, now); reason != 0 {
		st.unlinkLocked(old)
		st.mu.Unlock()
		clear(csrf)
		old.finish(st.onEnd, reason)
		return nil, ErrEnded
	}
	s := &Session[T]{
		store:    st,
		key:      k,
		id:       logID(k),
		created:  old.created,
		lastUsed: now,
		csrf:     csrf,
		done:     make(chan struct{}),
	}
	// The value moves while the store is locked, so that every other
	// operation of the store sees either the old session or the new one.
	// old is listed, so it has not ended.
	old.mu.Lock()
	s.data = old.data
	var zero T
	old.data = zero
	old.ended = true
	old.reason = EndRotated
	old.rotated = s
	clear(old.csrf)
	old.csrf = nil
	close(old.done)
	old.mu.Unlock()
	st.unlinkLocked(old)
	st.pushFrontLocked(s)
	st.mu.Unlock()

	http.SetCookie(w, st.cookie.cookie(token, 0))
	return s, nil
}

// newTokens draws a session token and a CSRF token, and returns the
// encoded session token with its key, and the encoded CSRF token.
func (st *Store[T]) newTokens() (token string, k key, csrf []byte, err error) {
	var raw [tokenBytes + csrfBytes]byte
	defer clear(raw[:])
	if err := st.rand.read(raw[:]); err != nil {
		return "", key{}, nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw[:tokenBytes])
	k = sha256.Sum256(raw[:tokenBytes])
	csrf = make([]byte, csrfLen)
	base64.RawURLEncoding.Encode(csrf, raw[tokenBytes:])
	return token, k, csrf, nil
}

// Lookup returns the live session that r's cookie names. Unless
// [Options.Passive] reports r as passive, the request counts as use of the
// session. An expired session is ended ([EndIdle] or [EndAbsolute]) and not
// returned. Lookup never creates sessions or sets cookies.
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
	if reason := st.expiryLocked(s, now); reason != 0 {
		st.unlinkLocked(s)
		st.mu.Unlock()
		s.finish(st.onEnd, reason)
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
// sets or clears cookies, and never creates sessions. The context slot is
// that of T, shared by every Store[T]: see [FromContext] before mounting
// the middlewares of two stores of the same T on one route.
func (st *Store[T]) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, ok := st.Lookup(r); ok {
			r = r.WithContext(WithSession(r.Context(), s))
		}
		next.ServeHTTP(w, r)
	})
}

// End ends the session that r's cookie names, if any, with the reason
// [EndLogout], and clears the cookie on w: the logout by cookie. It does not
// consult [Options.Passive] or the request context: a handler that holds
// its session from [FromContext] (and a handler test, whose session comes
// from [NewDetached] without a cookie) logs out with [Store.EndSession].
func (st *Store[T]) End(w http.ResponseWriter, r *http.Request) {
	if k, ok := parseToken(st.cookie.value(r)); ok {
		st.mu.Lock()
		s := st.sessions[k]
		if s != nil {
			st.unlinkLocked(s)
		}
		st.mu.Unlock()
		if s != nil {
			s.finish(st.onEnd, EndLogout)
		}
	}
	st.ClearCookie(w)
}

// EndSession ends s with the reason [EndLogout] and clears the session
// cookie on w: the logout of a handler that holds its session. The recipe
// (see the example): take the session from [FromContext], check the logout
// form's CSRF token against it, call EndSession before writing the
// response body, then redirect. Without a session there is nothing to end
// or check: clear the cookie and redirect. A handler test attaches a
// [NewDetached] session with [WithSession] and sees the same: the session
// ends, and w gets the cookie that clears it.
//
// If [Store.Rotate] moved the value of s to a new session meanwhile (a
// concurrent request of the same browser), EndSession ends that one too,
// so that the logout wins. s normally is a session of st or a detached
// one; a session of another store ends in that store, and st's cookie is
// cleared. A nil s only clears the cookie. Ending an ended session again
// changes nothing but sets the clearing cookie again. EndSession does not
// consult [Options.Passive].
func (st *Store[T]) EndSession(w http.ResponseWriter, s *Session[T]) {
	for s != nil {
		s.End()
		s = s.successor()
	}
	st.ClearCookie(w)
}

// ClearCookie sets a cookie on w that removes the session cookie from the
// browser. It does not end any session.
func (st *Store[T]) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, st.cookie.cookie("", -1))
}

// EndAll ends every session with the reason [EndRevoked], for example
// after a change of the application's credentials, and returns how many it
// ended. The store stays usable.
func (st *Store[T]) EndAll() int {
	st.mu.Lock()
	ended := make([]ending[T], 0, len(st.sessions))
	for s := st.head; s != nil; s = s.next {
		ended = append(ended, ending[T]{s, EndRevoked})
	}
	for _, e := range ended {
		st.unlinkLocked(e.s)
	}
	st.mu.Unlock()
	st.finish(ended)
	return len(ended)
}

// EndFunc ends every session whose value match reports true for, with the
// reason [EndRevoked], for example the sessions of one account after its
// password changed, and returns how many it ended. Sessions that
// [Store.Rotate] moves meanwhile are checked under their new token.
// Sessions that [Store.New] creates meanwhile may not be checked.
//
// match gets a copy of each value while that session is locked; it must
// not call methods of the session or the store. If match panics, the panic
// goes on to the caller, the sessions ended so far stay ended, and the
// others stay live and usable.
func (st *Store[T]) EndFunc(match func(data T) bool) int {
	st.mu.Lock()
	all := make([]*Session[T], 0, len(st.sessions))
	for s := st.head; s != nil; s = s.next {
		all = append(all, s)
	}
	st.mu.Unlock()
	n := 0
	for _, s := range all {
		for s != nil {
			matched, rotated := s.matches(match)
			if matched && st.end(s, EndRevoked) {
				n++
			}
			s = rotated
		}
	}
	return n
}

// Sweep ends the expired sessions ([EndIdle] or [EndAbsolute]) and returns
// how many it ended. Expired sessions also end when a request presents
// them; call Sweep periodically (every minute, say) so that the others end
// on time too and their [Session.Done] channels close. For example, from a
// goroutine owned by the application:
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

// Format prints String for every verb, formatted as a string for %v, %s,
// %q, %x and %X.
func (st *Store[T]) Format(f fmt.State, verb rune) {
	formatRedacted(f, verb, st.String())
}

// ending is a session that left the store, to end for reason once st.mu is
// released.
type ending[T any] struct {
	s      *Session[T]
	reason EndReason
}

// end removes s from the store, if it is still there, and ends it for
// reason. It reports whether this call ended s.
func (st *Store[T]) end(s *Session[T], reason EndReason) bool {
	st.mu.Lock()
	if s.listed {
		st.unlinkLocked(s)
	}
	st.mu.Unlock()
	return s.finish(st.onEnd, reason)
}

// finish ends sessions that already left the store. The caller must not
// hold st.mu. If an OnEnd hook panics, the remaining sessions still end
// before the panic goes on: nothing could reach them afterwards.
func (st *Store[T]) finish(ended []ending[T]) {
	i := 0
	defer func() {
		if i < len(ended) {
			// The hook of ended[i] panicked; Session.finish ended that
			// session all the same.
			st.finish(ended[i+1:])
		}
	}()
	for ; i < len(ended); i++ {
		ended[i].s.finish(st.onEnd, ended[i].reason)
	}
}

// expiryLocked returns why s has expired at now, or zero if it has not.
// The caller holds st.mu.
func (st *Store[T]) expiryLocked(s *Session[T], now time.Time) EndReason {
	switch {
	case elapsed(s.created, now) >= st.absolute:
		return EndAbsolute
	case elapsed(s.lastUsed, now) >= st.idle:
		return EndIdle
	}
	return 0
}

// elapsed returns the time from t to now: the greater of the difference of
// their monotonic clock readings (which [time.Time.Sub] uses when both have
// one) and that of their wall clock readings. Either alone can understate
// it: on some systems the monotonic clock stops while the computer sleeps,
// and the wall clock can be set back.
func elapsed(t, now time.Time) time.Duration {
	return max(now.Sub(t), now.Round(0).Sub(t.Round(0)))
}

// sweepLocked removes the expired sessions from the store and appends them
// to ended, for the caller to finish after releasing st.mu.
func (st *Store[T]) sweepLocked(now time.Time, ended []ending[T]) []ending[T] {
	for s := st.tail; s != nil; {
		prev := s.prev
		if reason := st.expiryLocked(s, now); reason != 0 {
			st.unlinkLocked(s)
			ended = append(ended, ending[T]{s, reason})
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
