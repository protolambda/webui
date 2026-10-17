package session

import (
	"context"
	"crypto/subtle"
	"errors"
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
// token is empty, and every check against it fails.
type Session[T any] struct {
	store   *Store[T] // nil for detached sessions
	key     key
	id      string
	created time.Time

	// Guarded by store.mu.
	lastUsed   time.Time
	prev, next *Session[T]
	listed     bool

	mu    sync.Mutex
	ended bool
	csrf  []byte
	data  T
	done  chan struct{}
}

// NewDetached returns a live session that belongs to no store, with the
// value data and the CSRF token csrf, for handler tests that skip the login
// (attach it with [WithSession]). Its log ID is "detached" and its creation
// time is the zero time. Ending it closes its Done channel; no OnEnd hook
// runs. An empty csrf makes every CSRF check fail.
func NewDetached[T any](data T, csrf string) *Session[T] {
	return &Session[T]{
		id:   detachedID,
		csrf: []byte(csrf),
		data: data,
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
// streams, should stop when it closes. Expiry closes it only once noticed:
// see [Store.Sweep].
func (s *Session[T]) Done() <-chan struct{} {
	return s.done
}

// Ended reports whether the session has ended.
func (s *Session[T]) Ended() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ended
}

// End ends the session and removes it from its store. It is idempotent.
// The cookie stays in the browser until cleared ([Store.End] or
// [Store.ClearCookie]), but names no session any more.
func (s *Session[T]) End() {
	if s.store != nil {
		s.store.end(s)
		return
	}
	s.finish(nil)
}

// CSRF returns the session's CSRF token, to render into its forms with
// [CSRFInput]; "" once the session has ended.
func (s *Session[T]) CSRF() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.csrf)
}

// CheckCSRF checks a submitted CSRF token against the session's, in
// constant time for tokens of the right length. It returns [ErrCSRF] if they
// differ, if token is empty, or if the session has ended.
func (s *Session[T]) CheckCSRF(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended || len(s.csrf) == 0 || subtle.ConstantTimeCompare(s.csrf, []byte(token)) != 1 {
		return ErrCSRF
	}
	return nil
}

// Data returns a copy of the session's value, and false (with the zero T)
// once the session has ended. The copy is shallow: when T holds pointers,
// slices or maps, their contents are shared with the session and need
// [Session.Update] for safe access.
func (s *Session[T]) Data() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		var zero T
		return zero, false
	}
	return s.data, true
}

// Update calls fn with a pointer to the session's value, while no other
// Data, Update or OnEnd call of the session runs. fn must not keep the
// pointer, nor call methods of the session or its store. Once the session
// has ended, Update does not call fn and returns [ErrEnded].
func (s *Session[T]) Update(fn func(data *T)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return ErrEnded
	}
	fn(&s.data)
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

// finish ends s, which already left its store: it runs onEnd, drops the
// value and the CSRF token, and closes Done. It reports whether this call
// ended s.
func (s *Session[T]) finish(onEnd func(string, *T)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return false
	}
	s.ended = true
	defer func() {
		var zero T
		s.data = zero
		clear(s.csrf)
		s.csrf = nil
		close(s.done)
	}()
	if onEnd != nil {
		onEnd(s.id, &s.data)
	}
	return true
}

// contextKey is the context key of sessions with values of type T.
type contextKey[T any] struct{}

// WithSession returns a copy of ctx that carries s, for [FromContext].
// [Store.Middleware] uses it for every request with a live session; tests
// use it to attach a session (from [NewDetached], say) without cookies. A
// nil s hides a session attached to ctx earlier.
func WithSession[T any](ctx context.Context, s *Session[T]) context.Context {
	return context.WithValue(ctx, contextKey[T]{}, s)
}

// FromContext returns the session with values of type T attached to ctx,
// if there is one and it has not ended.
func FromContext[T any](ctx context.Context) (*Session[T], bool) {
	s, _ := ctx.Value(contextKey[T]{}).(*Session[T])
	if s == nil || s.Ended() {
		return nil, false
	}
	return s, true
}
