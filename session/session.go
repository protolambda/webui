package session

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrEnded reports an operation on a session that has ended.
var ErrEnded = errors.New("session: ended")

// detachedID is the log ID of sessions made by [NewDetached].
const detachedID = "detached"

// Session is one browser session, with a value of type T. Sessions come
// from [Store.New], [Store.Lookup], [FromContext], or [NewDetached] in
// tests. A Session is safe for concurrent use; its value is guarded by the
// session (see [Session.Data] and [Session.Update]).
//
// Once ended, a session stays ended: its value is the zero T, its CSRF
// token is empty, and every check against it fails. Formatting a session
// with any verb prints only its log ID, never its value or CSRF token.
type Session[T any] struct {
	store   *Store[T] // nil for detached sessions
	key     key
	id      string
	created time.Time

	// Guarded by store.mu.
	lastUsed   time.Time
	prev, next *Session[T]
	listed     bool

	mu     sync.Mutex
	ended  bool
	reason EndReason
	// rotated is the session that [Store.Rotate] moved the value to.
	rotated *Session[T]
	// sec holds the CSRF token and the value; guarded by mu.
	sec  *hidden[secrets[T]]
	done chan struct{}
}

// secrets holds the parts of a session that must never be printed. They
// sit behind a hidden handle so that a Session formatted by value (a copy,
// which go vet reports as well, since a session holds a mutex) cannot
// expose them, including through fmt's unsupported-verb diagnostics.
type secrets[T any] struct {
	csrf []byte
	data T
}

// NewDetached returns a live session that belongs to no store, with the
// value data and the CSRF token csrf, for handler tests that skip the login
// (attach it with [WithSession]). Its log ID is "detached" and its creation
// time is the zero time. Ending it closes its Done channel; no OnEnd hook
// runs. An empty csrf makes every CSRF check fail.
func NewDetached[T any](data T, csrf string) *Session[T] {
	return &Session[T]{
		id:   detachedID,
		sec:  hide(secrets[T]{csrf: []byte(csrf), data: data}),
		done: make(chan struct{}),
	}
}

// ID returns the public identifier of the session, for logs and audit
// records. It is derived from the token by a one-way function and cannot be
// used to act as the session.
func (s *Session[T]) ID() string {
	return s.id
}

// Created returns the time the session was created: the login.
func (s *Session[T]) Created() time.Time {
	return s.created
}

// Done returns a channel that is closed when the session ends, after the
// OnEnd hook ran. Long-running requests of the session, such as event
// streams, should stop when it closes, and may tell the user why
// ([Session.EndReason]). Expiry closes it only once noticed: see
// [Store.Sweep].
func (s *Session[T]) Done() <-chan struct{} {
	return s.done
}

// EndReason returns why the session ended, or zero while it is live. It is
// set before the OnEnd hook runs and Done closes. When several operations
// end the session at once, the reason is that of the one that ended it.
func (s *Session[T]) EndReason() EndReason {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// Ended reports whether the session has ended.
func (s *Session[T]) Ended() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

// End ends the session and removes it from its store, with the reason
// [EndLogout]. It is idempotent. The cookie stays in the browser until
// cleared, but names no session any more: a logout calls
// [Store.EndSession], which also clears it. Once [Store.Rotate] moved the
// value to a new session, End of the old session does nothing: end the new
// one, or call [Store.EndSession], which follows the move.
func (s *Session[T]) End() {
	if s.store != nil {
		s.store.end(s, EndLogout)
		return
	}
	s.finish(nil, EndLogout)
}

// CSRF returns the session's CSRF token, to render into its forms with
// [CSRFInput]; "" once the session has ended.
func (s *Session[T]) CSRF() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.sec.value.csrf)
}

// CheckCSRF checks a submitted CSRF token against the session's, in
// constant time for tokens of the right length. It returns [ErrCSRF] if they
// differ, if token is empty, or if the session has ended.
func (s *Session[T]) CheckCSRF(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended || len(s.sec.value.csrf) == 0 || subtle.ConstantTimeCompare(s.sec.value.csrf, []byte(token)) != 1 {
		return ErrCSRF
	}
	return nil
}

// Data returns a copy of the session's value, and false (with the zero T)
// once the session has ended. The copy is shallow: when T holds pointers,
// slices or maps, their contents are shared with the session and need
// [Session.Update] for safe access, and an OnEnd hook that clears them
// clears them for the copy too (see [Store.New] and [Store.Rotate]).
func (s *Session[T]) Data() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		var zero T
		return zero, false
	}
	return s.sec.value.data, true
}

// Update calls fn with a pointer to the session's value, while no other
// Data, Update or OnEnd call of the session runs. fn must not keep the
// pointer, nor call methods of the session or its store, and should be
// short: [Store.Rotate] of the session waits for it while holding the
// store's lock. Once the session has ended, Update does not call fn and
// returns [ErrEnded].
func (s *Session[T]) Update(fn func(data *T)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return ErrEnded
	}
	fn(&s.sec.value.data)
	return nil
}

// String describes the session by its log ID, without its token or value.
func (s *Session[T]) String() string {
	return "session.Session(" + s.id + ")"
}

// GoString is like String, so that %#v does not print the value or the
// CSRF token either.
func (s *Session[T]) GoString() string {
	return s.String()
}

// Format prints String for every verb (fmt would print the value and the
// CSRF token for verbs such as %d), formatted as a string for %v, %s, %q,
// %x and %X.
func (s *Session[T]) Format(f fmt.State, verb rune) {
	formatRedacted(f, verb, s.String())
}

// finish ends s, which already left its store, for reason: it runs onEnd,
// drops the value and the CSRF token, and closes Done, even when onEnd
// panics. It reports whether this call ended s.
func (s *Session[T]) finish(onEnd func(string, EndReason, *T), reason EndReason) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return false
	}
	s.ended = true
	s.reason = reason
	defer func() {
		var zero T
		s.sec.value.data = zero
		clear(s.sec.value.csrf)
		s.sec.value.csrf = nil
		close(s.done)
	}()
	if onEnd != nil {
		onEnd(s.id, reason, &s.sec.value.data)
	}
	return true
}

// successor returns the session that [Store.Rotate] moved the value of s
// to, or nil if it did not.
func (s *Session[T]) successor() *Session[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotated
}

// matches reports whether s is live and match reports true for its value.
// For a session that [Store.Rotate] ended, it returns the session that
// holds the value now, for the caller to check in its place. The deferred
// unlock keeps a panicking match from leaving s locked.
func (s *Session[T]) matches(match func(T) bool) (bool, *Session[T]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return false, s.rotated
	}
	return match(s.sec.value.data), nil
}

// contextKey is the context key of sessions with values of type T.
type contextKey[T any] struct{}

// WithSession returns a copy of ctx that carries s, for [FromContext].
// [Store.Middleware] uses it for every request with a live session; tests
// use it to attach a session (from [NewDetached], say) without cookies. A
// nil s hides a session attached to ctx earlier. The context has one slot
// per value type T, shared by every Store[T]: see [FromContext].
func WithSession[T any](ctx context.Context, s *Session[T]) context.Context {
	return context.WithValue(ctx, contextKey[T]{}, s)
}

// FromContext returns the session with values of type T attached to ctx,
// if there is one and it has not ended.
//
// A context holds at most one session per value type T, whichever store it
// came from: the latest [WithSession] for T wins, including the one of the
// Middleware of another Store[T], and a Middleware that finds no session
// leaves the slot as it was. So with two stores of the same T on one route
// (users and administrators, say), FromContext can return a session of the
// other store. Give each store its own value type (type adminAccount
// Account), which keeps their sessions apart at compile time, or call that
// store's [Store.Lookup] instead.
func FromContext[T any](ctx context.Context) (*Session[T], bool) {
	s, _ := ctx.Value(contextKey[T]{}).(*Session[T])
	if s == nil || s.Ended() {
		return nil, false
	}
	return s, true
}
