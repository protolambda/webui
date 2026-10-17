package session_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/session"
)

// account is the session value of the tests.
type account struct {
	ID     int64
	Secret []byte
}

// clock is a manual clock for the tests.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock {
	return &clock{now: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// seeded returns a deterministic source of randomness.
func seeded(seed byte) *rand.ChaCha8 {
	return rand.NewChaCha8([32]byte{seed})
}

// ended records the sessions an OnEnd hook saw.
type ended struct {
	mu  sync.Mutex
	ids []string
}

func (e *ended) hook(id string, data *account) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ids = append(e.ids, id)
	clear(data.Secret)
}

func (e *ended) list() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.ids...)
}

func newStore(t devtest.T, opts session.Options[account]) *session.Store[account] {
	if opts.Rand == nil {
		opts.Rand = seeded(1)
	}
	st, err := session.NewStore(opts)
	t.Must(be.NoError(err))
	return st
}

// get returns a GET request carrying cookies (name and value only, as a
// browser sends them).
func get(t devtest.T, path string, cookies ...*http.Cookie) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	for _, c := range cookies {
		r.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	return r
}

// login creates a session for r and returns it with its cookie.
func login(t devtest.T, st *session.Store[account], r *http.Request, data account) (*session.Session[account], *http.Cookie) {
	rec := httptest.NewRecorder()
	s, err := st.New(rec, r, data)
	t.Must(be.NoError(err))
	cookies := rec.Result().Cookies()
	t.Must(be.Len(cookies, 1))
	return s, cookies[0]
}

func closed(s interface{ Done() <-chan struct{} }) bool {
	select {
	case <-s.Done():
		return true
	default:
		return false
	}
}

func TestNewSetsSecureCookie(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	t.Must(be.Equal("__Host-session", st.CookieName()))

	rec := httptest.NewRecorder()
	_, err := st.New(rec, nil, account{ID: 1})
	t.Must(be.NoError(err))
	header := rec.Header().Values("Set-Cookie")
	t.Must(be.Len(header, 1))
	name, rest, _ := strings.Cut(header[0], "=")
	value, attrs, _ := strings.Cut(rest, ";")
	t.Must(be.Equal("__Host-session", name))
	t.Must(be.Equal(43, len(value)))
	t.Must(be.Equal(" Path=/; HttpOnly; Secure; SameSite=Lax", attrs))

	c := rec.Result().Cookies()[0]
	t.Must(be.Equal(0, c.MaxAge))
	t.Must(be.True(c.Expires.IsZero()))
	t.Must(be.Equal("", c.Domain))
}

func TestCookieOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		opts  session.Options[account]
		name  string
		attrs string
	}{
		{session.Options[account]{Insecure: true}, "session", " Path=/; HttpOnly; SameSite=Lax"},
		{session.Options[account]{StrictSameSite: true}, "__Host-session", " Path=/; HttpOnly; Secure; SameSite=Strict"},
		{session.Options[account]{CookieName: "app_sid"}, "__Host-app_sid", " Path=/; HttpOnly; Secure; SameSite=Lax"},
		{session.Options[account]{CookieName: "app", Insecure: true, StrictSameSite: true}, "app", " Path=/; HttpOnly; SameSite=Strict"},
	}
	for _, c := range cases {
		st := newStore(t, c.opts)
		t.Must(be.Equal(c.name, st.CookieName()))
		rec := httptest.NewRecorder()
		_, err := st.New(rec, nil, account{})
		t.Must(be.NoError(err))
		name, rest, _ := strings.Cut(rec.Header().Get("Set-Cookie"), "=")
		_, attrs, _ := strings.Cut(rest, ";")
		t.Must(be.Equal(c.name, name))
		t.Must(be.Equal(c.attrs, attrs))

		rec = httptest.NewRecorder()
		st.ClearCookie(rec)
		t.Must(be.Equal(c.name+"=;"+strings.Replace(c.attrs, " Path=/;", " Path=/; Max-Age=0;", 1), rec.Header().Get("Set-Cookie")))
	}
}

func TestNewStoreRejectsBadOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, opts := range []session.Options[account]{
		{CookieName: "__Host-session"},
		{CookieName: "__host-session"},
		{CookieName: "__Secure-session"},
		{CookieName: "bad name"},
		{CookieName: "a;b"},
		{CookieName: "a=b"},
		{IdleTTL: -time.Second},
		{AbsoluteTTL: -time.Second},
		{MaxSessions: -1},
		{OnEnd: func(string, *account) {}, OnEndReason: func(string, session.EndReason, *account) {}},
	} {
		st, err := session.NewStore(opts)
		t.Mustf(be.Error(err), "options %+v", opts)
		t.Must(be.NilPtr(st))
	}
}

func TestLookup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	s, cookie := login(t, st, nil, account{ID: 7})

	got, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))
	data, ok := got.Data()
	t.Must(be.True(ok))
	t.Must(be.Equal(int64(7), data.ID))

	_, ok = st.Lookup(get(t, "/"))
	t.Must(be.False(ok))

	// A canonical token differs from the cookie only in its last character:
	// the strict decoding refuses the non-canonical spelling.
	last := cookie.Value[len(cookie.Value)-1]
	alias := []byte(cookie.Value)
	alias[len(alias)-1] = last ^ 1
	for _, value := range []string{
		"", "x", cookie.Value + "A", cookie.Value[1:], strings.Repeat("A", 43),
		strings.Repeat("!", 43), strings.Repeat("A", 10000), string(alias),
		strings.Replace(cookie.Value, cookie.Value[:1], "+", 1),
	} {
		_, ok := st.Lookup(get(t, "/", &http.Cookie{Name: cookie.Name, Value: value}))
		t.Mustf(be.False(ok), "cookie value %q", value)
	}
	// The same token under another cookie name is not the session's.
	_, ok = st.Lookup(get(t, "/", &http.Cookie{Name: "session", Value: cookie.Value}))
	t.Must(be.False(ok))
}

func TestNewRotatesTheSession(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var e ended
	st := newStore(t, session.Options[account]{OnEnd: e.hook})
	first, firstCookie := login(t, st, nil, account{ID: 1, Secret: []byte("one")})

	second, secondCookie := login(t, st, get(t, "/login", firstCookie), account{ID: 2})
	t.Must(be.True(first.Ended()))
	t.Must(be.True(closed(first)))
	t.Must(be.False(second.Ended()))
	t.Must(be.NotEqual(firstCookie.Value, secondCookie.Value))
	t.Must(be.NotEqual(first.ID(), second.ID()))
	t.Must(be.DeepEqual([]string{first.ID()}, e.list()))
	t.Must(be.Equal(1, st.Len()))

	_, ok := st.Lookup(get(t, "/", firstCookie))
	t.Must(be.False(ok))
	got, ok := st.Lookup(get(t, "/", secondCookie))
	t.Must(be.True(ok))
	t.Must(be.Same(second, got))

	// A cookie that names no session is no obstacle.
	third, _ := login(t, st, get(t, "/login", firstCookie), account{ID: 3})
	t.Must(be.False(third.Ended()))
	t.Must(be.False(second.Ended()))
	t.Must(be.Equal(2, st.Len()))
}

func TestIdleExpiry(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var e ended
	st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: 10 * time.Minute, OnEnd: e.hook})
	s, cookie := login(t, st, nil, account{ID: 1})

	clk.Advance(10*time.Minute - time.Nanosecond)
	_, ok := st.Lookup(get(t, "/", cookie)) // restarts the idle timer
	t.Must(be.True(ok))
	clk.Advance(10*time.Minute - time.Nanosecond)
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.True(ok))

	clk.Advance(10 * time.Minute)
	t.Must(be.False(closed(s))) // not noticed yet
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))
	t.Must(be.True(closed(s)))
	t.Must(be.DeepEqual([]string{s.ID()}, e.list()))
	t.Must(be.Equal(0, st.Len()))
}

func TestAbsoluteExpiry(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: 15 * time.Minute, AbsoluteTTL: time.Hour})
	s, cookie := login(t, st, nil, account{ID: 1})
	t.Must(be.Equal(clk.Now(), s.Created()))
	for range 5 {
		clk.Advance(10 * time.Minute)
		_, ok := st.Lookup(get(t, "/", cookie))
		t.Must(be.True(ok))
	}
	clk.Advance(10*time.Minute - time.Nanosecond)
	_, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.True(ok))
	clk.Advance(time.Nanosecond)
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))
	t.Must(be.True(closed(s)))
}

func TestDefaultLifetimes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	_, cookie := login(t, st, nil, account{})
	clk.Advance(session.DefaultIdleTTL - time.Second)
	_, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.True(ok))
	clk.Advance(session.DefaultIdleTTL)
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))

	_, cookie = login(t, st, nil, account{})
	for elapsed := 10 * time.Minute; elapsed < session.DefaultAbsoluteTTL; elapsed += 10 * time.Minute {
		clk.Advance(10 * time.Minute)
		_, ok = st.Lookup(get(t, "/", cookie))
		t.Must(be.True(ok))
	}
	clk.Advance(10 * time.Minute)
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))
}

func TestPassiveRequestsDoNotCountAsUse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	passive := func(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/live/") }
	st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: 10 * time.Minute, Passive: passive})
	s, cookie := login(t, st, nil, account{ID: 1})

	clk.Advance(6 * time.Minute)
	got, ok := st.Lookup(get(t, "/live/tasks", cookie))
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))
	clk.Advance(4 * time.Minute)
	_, ok = st.Lookup(get(t, "/live/tasks", cookie))
	t.Must(be.False(ok))
	t.Must(be.True(closed(s)))

	// A normal request restarts the timer.
	s, cookie = login(t, st, nil, account{ID: 2})
	clk.Advance(6 * time.Minute)
	_, ok = st.Lookup(get(t, "/tasks", cookie))
	t.Must(be.True(ok))
	clk.Advance(6 * time.Minute)
	_, ok = st.Lookup(get(t, "/live/tasks", cookie))
	t.Must(be.True(ok))
	t.Must(be.False(closed(s)))
}

func TestEvictionEndsTheLeastRecentlyUsed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var e ended
	passive := func(r *http.Request) bool { return r.URL.Path == "/live" }
	st := newStore(t, session.Options[account]{Now: clk.Now, MaxSessions: 3, OnEnd: e.hook, Passive: passive})
	a, aCookie := login(t, st, nil, account{ID: 1})
	clk.Advance(time.Second)
	b, bCookie := login(t, st, nil, account{ID: 2})
	clk.Advance(time.Second)
	c, _ := login(t, st, nil, account{ID: 3})
	clk.Advance(time.Second)
	_, ok := st.Lookup(get(t, "/", aCookie)) // a becomes the most recent
	t.Must(be.True(ok))
	_, ok = st.Lookup(get(t, "/live", bCookie)) // passive: b stays the least recent
	t.Must(be.True(ok))

	d, _ := login(t, st, nil, account{ID: 4})
	t.Must(be.Equal(3, st.Len()))
	t.Must(be.True(closed(b)))
	t.Must(be.False(a.Ended()))
	t.Must(be.False(c.Ended()))
	t.Must(be.False(d.Ended()))
	t.Must(be.DeepEqual([]string{b.ID()}, e.list()))

	// Next in line: c, then a.
	login(t, st, nil, account{ID: 5})
	t.Must(be.True(c.Ended()))
	login(t, st, nil, account{ID: 6})
	t.Must(be.True(a.Ended()))
	t.Must(be.False(d.Ended()))
}

func TestEvictionDropsExpiredSessionsFirst(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now, MaxSessions: 3, IdleTTL: 50 * time.Minute, AbsoluteTTL: time.Hour})
	a, aCookie := login(t, st, nil, account{ID: 1})
	clk.Advance(30 * time.Minute)
	b, _ := login(t, st, nil, account{ID: 2})
	c, _ := login(t, st, nil, account{ID: 3})
	clk.Advance(15 * time.Minute)
	_, ok := st.Lookup(get(t, "/", aCookie)) // a is the most recent, but older than an hour soon
	t.Must(be.True(ok))
	clk.Advance(15 * time.Minute)

	login(t, st, nil, account{ID: 4})
	t.Must(be.True(a.Ended()))
	t.Must(be.False(b.Ended()))
	t.Must(be.False(c.Ended()))
}

func TestDefaultMaxSessions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	first, _ := login(t, st, nil, account{})
	for range session.DefaultMaxSessions - 1 {
		login(t, st, nil, account{})
	}
	t.Must(be.Equal(session.DefaultMaxSessions, st.Len()))
	t.Must(be.False(first.Ended()))
	login(t, st, nil, account{})
	t.Must(be.Equal(session.DefaultMaxSessions, st.Len()))
	t.Must(be.True(first.Ended()))
}

func TestSessionEnd(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var e ended
	st := newStore(t, session.Options[account]{OnEnd: e.hook})
	secret := []byte("hunter2")
	s, cookie := login(t, st, nil, account{ID: 1, Secret: secret})
	ctx := session.WithSession(t.Context(), s)
	csrf := s.CSRF()

	s.End()
	t.Must(be.True(s.Ended()))
	t.Must(be.True(closed(s)))
	t.Must(be.DeepEqual([]byte{0, 0, 0, 0, 0, 0, 0}, secret)) // cleared by the hook
	data, ok := s.Data()
	t.Must(be.False(ok))
	t.Must(be.DeepEqual(account{}, data))
	t.Must(be.ErrorIs(s.Update(func(*account) { panic("not called") }), session.ErrEnded))
	t.Must(be.Equal("", s.CSRF()))
	t.Must(be.ErrorIs(s.CheckCSRF(csrf), session.ErrCSRF))
	_, ok = session.FromContext[account](ctx)
	t.Must(be.False(ok))
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))

	s.End() // idempotent
	t.Must(be.DeepEqual([]string{s.ID()}, e.list()))
	t.Must(be.Equal(0, st.Len()))
}

func TestStoreEndIsTheLogout(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var e ended
	st := newStore(t, session.Options[account]{OnEnd: e.hook})
	secret := []byte("key")
	s, cookie := login(t, st, nil, account{ID: 1, Secret: secret})
	other, _ := login(t, st, nil, account{ID: 2})

	rec := httptest.NewRecorder()
	st.End(rec, get(t, "/logout", cookie))
	t.Must(be.True(closed(s)))
	t.Must(be.Equal(session.EndLogout, s.EndReason()))
	t.Must(be.DeepEqual([]string{s.ID()}, e.list()))
	t.Must(be.DeepEqual([]byte{0, 0, 0}, secret))
	t.Must(be.False(other.Ended()))
	t.Must(be.Equal("__Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax", rec.Header().Get("Set-Cookie")))
	cleared := rec.Result().Cookies()[0]
	t.Must(be.Equal(-1, cleared.MaxAge))

	// Without a session, the cookie is cleared all the same.
	rec = httptest.NewRecorder()
	st.End(rec, get(t, "/logout"))
	t.Must(be.Len(rec.Result().Cookies(), 1))
	t.Must(be.Equal(1, st.Len()))
}

func TestEndSessionIsTheLogoutOfAHeldSession(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var log endLog
	st := newStore(t, session.Options[account]{OnEndReason: log.hook})
	s, cookie := login(t, st, nil, account{ID: 1})
	log.watch(s)
	other, _ := login(t, st, nil, account{ID: 2})
	const clearing = "__Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax"

	rec := httptest.NewRecorder()
	st.EndSession(rec, s)
	t.Must(be.True(s.Ended()))
	t.Must(be.True(closed(s)))
	t.Must(be.Equal(session.EndLogout, s.EndReason()))
	t.Must(be.Equal(1, log.count(s.ID())))
	t.Must(be.Equal(0, log.lateCalls()))
	t.Must(be.DeepEqual([]string{clearing}, rec.Header().Values("Set-Cookie")))
	_, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))
	t.Must(be.False(other.Ended()))
	t.Must(be.Equal(1, st.Len()))

	// Again: nothing more ends, and the cookie is cleared again.
	rec = httptest.NewRecorder()
	st.EndSession(rec, s)
	t.Must(be.Equal(1, log.total()))
	t.Must(be.Equal(session.EndLogout, s.EndReason()))
	t.Must(be.DeepEqual([]string{clearing}, rec.Header().Values("Set-Cookie")))

	// Without a session, only the cookie is cleared.
	rec = httptest.NewRecorder()
	st.EndSession(rec, nil)
	t.Must(be.DeepEqual([]string{clearing}, rec.Header().Values("Set-Cookie")))
	t.Must(be.Equal(1, st.Len()))
	t.Must(be.False(other.Ended()))
}

func TestEndSessionOfADetachedSession(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{Insecure: true, CookieName: "sid"})
	s := session.NewDetached(account{ID: 1}, "test-csrf")
	ctx := session.WithSession(t.Context(), s)

	rec := httptest.NewRecorder()
	st.EndSession(rec, s)
	t.Must(be.True(closed(s)))
	t.Must(be.Equal(session.EndLogout, s.EndReason()))
	t.Must(be.ErrorIs(s.CheckCSRF("test-csrf"), session.ErrCSRF))
	_, ok := session.FromContext[account](ctx)
	t.Must(be.False(ok))
	t.Must(be.DeepEqual([]string{"sid=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax"}, rec.Header().Values("Set-Cookie")))
}

func TestEndSessionFollowsRotations(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var log endLog
	st := newStore(t, session.Options[account]{OnEndReason: log.hook})
	old, cookie := login(t, st, nil, account{ID: 1})
	other, _ := login(t, st, nil, account{ID: 2})

	// A request with the old session is still in flight while two others
	// rotate it.
	rec := httptest.NewRecorder()
	mid, err := st.Rotate(rec, get(t, "/reauth", cookie))
	t.Must(be.NoError(err))
	latest, err := st.Rotate(httptest.NewRecorder(), get(t, "/reauth", rec.Result().Cookies()[0]))
	t.Must(be.NoError(err))
	log.watch(latest)

	st.EndSession(httptest.NewRecorder(), old)
	t.Must(be.True(closed(latest)))
	t.Must(be.Equal(session.EndLogout, latest.EndReason()))
	t.Must(be.Equal(session.EndRotated, old.EndReason()))
	t.Must(be.Equal(session.EndRotated, mid.EndReason()))
	t.Must(be.Equal(1, log.count(latest.ID())))
	t.Must(be.Equal(1, log.total()))
	t.Must(be.Equal(0, log.lateCalls()))
	t.Must(be.False(other.Ended()))
	t.Must(be.Equal(1, st.Len()))
}

func TestEndSessionOfAnotherStore(gt *testing.T) {
	t := devtest.ParallelT(gt)
	users := newStore(t, session.Options[account]{})
	admins := newStore(t, session.Options[account]{CookieName: "admin", Rand: seeded(5)})
	s, _ := login(t, users, nil, account{ID: 1})
	kept, _ := login(t, admins, nil, account{ID: 1})

	// The session ends in its own store; the cookie cleared is admins'.
	rec := httptest.NewRecorder()
	admins.EndSession(rec, s)
	t.Must(be.Equal(session.EndLogout, s.EndReason()))
	t.Must(be.Equal(0, users.Len()))
	t.Must(be.False(kept.Ended()))
	t.Must(be.Equal(1, admins.Len()))
	t.Must(be.DeepEqual([]string{"__Host-admin=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax"}, rec.Header().Values("Set-Cookie")))
}

func TestEndAll(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var e ended
	st := newStore(t, session.Options[account]{OnEnd: e.hook})
	var all []*session.Session[account]
	for i := range 3 {
		s, _ := login(t, st, nil, account{ID: int64(i)})
		all = append(all, s)
	}
	t.Must(be.Equal(3, st.EndAll()))
	for _, s := range all {
		t.Must(be.True(closed(s)))
	}
	t.Must(be.Len(e.list(), 3))
	t.Must(be.Equal(0, st.Len()))
	t.Must(be.Equal(0, st.EndAll()))

	s, cookie := login(t, st, nil, account{ID: 9})
	got, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))
}

func TestEndFunc(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	a1, _ := login(t, st, nil, account{ID: 1})
	b, _ := login(t, st, nil, account{ID: 2})
	a2, _ := login(t, st, nil, account{ID: 1})

	n := st.EndFunc(func(data account) bool { return data.ID == 1 })
	t.Must(be.Equal(2, n))
	t.Must(be.True(closed(a1)))
	t.Must(be.True(closed(a2)))
	t.Must(be.False(b.Ended()))
	t.Must(be.Equal(1, st.Len()))
	t.Must(be.Equal(0, st.EndFunc(func(data account) bool { return data.ID == 1 })))
}

func TestSweep(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var e ended
	st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: 10 * time.Minute, AbsoluteTTL: time.Hour, OnEnd: e.hook})
	old, _ := login(t, st, nil, account{ID: 1})
	clk.Advance(5 * time.Minute)
	young, _ := login(t, st, nil, account{ID: 2})
	t.Must(be.Equal(0, st.Sweep()))

	clk.Advance(5 * time.Minute)
	t.Must(be.Equal(1, st.Sweep()))
	t.Must(be.True(closed(old)))
	t.Must(be.False(closed(young)))
	t.Must(be.DeepEqual([]string{old.ID()}, e.list()))
	t.Must(be.Equal(1, st.Len()))

	clk.Advance(time.Hour)
	t.Must(be.Equal(1, st.Sweep()))
	t.Must(be.True(closed(young)))
	t.Must(be.Equal(0, st.Len()))
}

func TestMiddleware(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	var seen *session.Session[account]
	h := st.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = session.FromContext[account](r.Context())
	}))

	serve := func(r *http.Request) *httptest.ResponseRecorder {
		seen = nil
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}

	// Anonymous requests cost nothing and get no cookie.
	for range 10 {
		rec := serve(get(t, "/"))
		t.Must(be.NilPtr(seen))
		t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	}
	t.Must(be.Equal(0, st.Len()))

	s, cookie := login(t, st, nil, account{ID: 5})
	rec := serve(get(t, "/tasks", cookie))
	t.Must(be.Same(s, seen))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))

	// A bogus or expired cookie: anonymous, and the cookie is left alone.
	rec = serve(get(t, "/", &http.Cookie{Name: cookie.Name, Value: "bogus"}))
	t.Must(be.NilPtr(seen))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	clk.Advance(session.DefaultIdleTTL)
	rec = serve(get(t, "/", cookie))
	t.Must(be.NilPtr(seen))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	t.Must(be.True(s.Ended()))
}

func TestDataAndUpdate(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var last account
	st := newStore(t, session.Options[account]{OnEnd: func(id string, data *account) {
		last = *data
		data.ID = -1 // dropped anyway
	}})
	s, _ := login(t, st, nil, account{ID: 1})

	data, ok := s.Data()
	t.Must(be.True(ok))
	data.ID = 100 // a copy
	t.Must(be.NoError(s.Update(func(a *account) {
		t.Must(be.Equal(int64(1), a.ID))
		a.ID = 2
	})))
	data, _ = s.Data()
	t.Must(be.Equal(int64(2), data.ID))

	s.End()
	t.Must(be.Equal(int64(2), last.ID))
	data, ok = s.Data()
	t.Must(be.False(ok))
	t.Must(be.Equal(int64(0), data.ID))
}

type failingRand struct{ err error }

func (f failingRand) Read([]byte) (int, error) { return 0, f.err }

func TestNewFailsWithoutRandomness(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cause := errors.New("no entropy")
	src := &switchable{r: seeded(3)}
	st := newStore(t, session.Options[account]{Rand: src})
	s, cookie := login(t, st, nil, account{ID: 1})

	src.set(failingRand{cause})
	rec := httptest.NewRecorder()
	got, err := st.New(rec, get(t, "/login", cookie), account{ID: 2})
	t.Must(be.ErrorIs(err, cause))
	t.Must(be.NilPtr(got))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	t.Must(be.False(s.Ended())) // no rotation without a new session
	t.Must(be.Equal(1, st.Len()))
}

// switchable is a source of randomness that a test can replace.
type switchable struct {
	mu sync.Mutex
	r  interface{ Read([]byte) (int, error) }
}

func (s *switchable) set(r interface{ Read([]byte) (int, error) }) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.r = r
}

func (s *switchable) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.r.Read(p)
}

// zeros is a broken source of randomness: always the same bytes.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestNewRefusesATokenCollision(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{Rand: zeros{}})
	s, _ := login(t, st, nil, account{ID: 1})
	rec := httptest.NewRecorder()
	got, err := st.New(rec, nil, account{ID: 2})
	t.Must(be.Error(err))
	t.Must(be.NilPtr(got))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	t.Must(be.False(s.Ended()))
}

func TestDetached(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := session.NewDetached(account{ID: 3}, "test-token")
	ctx := session.WithSession(t.Context(), s)
	got, ok := session.FromContext[account](ctx)
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))
	t.Must(be.Equal("detached", s.ID()))
	t.Must(be.True(s.Created().IsZero()))
	t.Must(be.Equal("test-token", s.CSRF()))
	t.Must(be.NoError(s.CheckCSRF("test-token")))
	t.Must(be.ErrorIs(s.CheckCSRF("test-tokeN"), session.ErrCSRF))

	s.End()
	t.Must(be.True(closed(s)))
	_, ok = session.FromContext[account](ctx)
	t.Must(be.False(ok))

	empty := session.NewDetached(account{}, "")
	t.Must(be.ErrorIs(empty.CheckCSRF(""), session.ErrCSRF))
}

func TestFromContext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	_, ok := session.FromContext[account](context.Background())
	t.Must(be.False(ok))

	s := session.NewDetached(account{ID: 1}, "t")
	ctx := session.WithSession(t.Context(), s)
	// Sessions of another value type are invisible.
	_, ok = session.FromContext[string](ctx)
	t.Must(be.False(ok))
	// A nil session hides the earlier one.
	_, ok = session.FromContext[account](session.WithSession[account](ctx, nil))
	t.Must(be.False(ok))
}

// formatVerbs are the verbs the formatting tests try: those fmt sends to
// String or GoString, and those it would answer by printing the fields.
var formatVerbs = []string{
	"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%o", "%O", "%b",
	"%t", "%e", "%f", "%g", "%c", "%U", "%10v", "%-30s", "%#x",
}

// leaks returns the forms in which the formatting of a value could show
// a secret: as text, as its bytes in decimal, and in hex.
func leaks(secrets ...string) []string {
	var out []string
	for _, secret := range secrets {
		out = append(out, secret, fmt.Sprint([]byte(secret)), hex.EncodeToString([]byte(secret)))
	}
	return out
}

func TestFormattingHidesSecrets(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	s, cookie := login(t, st, nil, account{ID: 424242, Secret: []byte("hunter2")})
	forbidden := append(leaks(s.CSRF(), cookie.Value, "hunter2"), "424242")
	type wrapper struct{ S *session.Session[account] }
	for _, verb := range formatVerbs {
		for _, v := range []any{s, st, wrapper{s}, []*session.Session[account]{s}} {
			out := fmt.Sprintf(verb, v)
			for _, secret := range forbidden {
				t.Mustf(be.False(strings.Contains(out, secret)), "%s: %s", verb, out)
			}
		}
	}
	t.Must(be.Equal("session.Session("+s.ID()+")", fmt.Sprint(s)))
	t.Must(be.Equal("session.Session("+s.ID()+")", fmt.Sprintf("%#v", s)))
	t.Must(be.Equal(`"session.Store(__Host-session)"`, fmt.Sprintf("%q", st)))
	t.Must(be.Equal("session.Store(__Host-session)", fmt.Sprintf("%d", st)))
	t.Must(be.Equal("session.Store(__Host-session)", fmt.Sprint(st)))
	t.Must(be.Equal("<nil>", fmt.Sprint((*session.Session[account])(nil))))
	t.Must(be.Equal(s.String(), s.GoString()))
	t.Must(be.Equal(st.String(), st.GoString()))
}

func TestSessionEndsWithAReason(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var log endLog
	st := newStore(t, session.Options[account]{
		Now: clk.Now, IdleTTL: time.Hour, AbsoluteTTL: 3 * time.Hour, MaxSessions: 3, OnEndReason: log.hook,
	})
	start := func(id int64) (*session.Session[account], *http.Cookie) {
		s, cookie := login(t, st, nil, account{ID: id})
		log.watch(s)
		return s, cookie
	}
	reasons := map[*session.Session[account]]session.EndReason{}

	s, cookie := start(1)
	t.Must(be.Equal(session.EndReason(0), s.EndReason()))
	st.End(httptest.NewRecorder(), get(t, "/logout", cookie))
	reasons[s] = session.EndLogout

	s, _ = start(2)
	s.End()
	reasons[s] = session.EndLogout

	s, _ = start(3)
	t.Must(be.Equal(1, st.EndFunc(func(a account) bool { return a.ID == 3 })))
	reasons[s] = session.EndRevoked

	s, cookie = start(4)
	next, _ := login(t, st, get(t, "/login", cookie), account{ID: 40})
	log.watch(next)
	reasons[s] = session.EndRotated

	s, cookie = start(5)
	clk.Advance(time.Hour)
	_, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))
	reasons[s] = session.EndIdle
	_, ok = st.Lookup(get(t, "/", cookie)) // and stays ended
	t.Must(be.False(ok))

	// next (created two hours ago) passes its absolute lifetime.
	s, cookie = start(6)
	for range 4 {
		clk.Advance(30 * time.Minute)
		_, ok = st.Lookup(get(t, "/", cookie))
		t.Must(be.True(ok))
	}
	t.Must(be.Equal(1, st.Sweep())) // next is past both lifetimes: absolute counts first
	reasons[next] = session.EndAbsolute

	// Fill the store: the least recently used goes.
	a, _ := start(7)
	b, _ := start(8)
	t.Must(be.False(s.Ended()))
	c, _ := start(9)
	reasons[s] = session.EndEvicted
	t.Must(be.True(s.Ended()))

	t.Must(be.Equal(3, st.EndAll()))
	reasons[a] = session.EndRevoked
	reasons[b] = session.EndRevoked
	reasons[c] = session.EndRevoked

	// Ending again changes nothing.
	for s := range reasons {
		s.End()
	}
	t.Must(be.Equal(0, st.EndAll()))
	t.Must(be.Equal(0, st.Sweep()))

	for s, reason := range reasons {
		t.Mustf(be.Equal(reason, s.EndReason()), "session %d", reason)
		t.Must(be.True(closed(s)))
		t.Must(be.Equal(1, log.count(s.ID())))
		t.Must(be.Equal(reason, log.reason(s.ID())))
	}
	t.Must(be.Equal(0, log.lateCalls()))
	t.Must(be.Equal(len(reasons), log.total()))
}

func TestEndReasonString(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for reason, want := range map[session.EndReason]string{
		session.EndLogout:   "logout",
		session.EndRevoked:  "revoked",
		session.EndIdle:     "idle",
		session.EndAbsolute: "absolute",
		session.EndRotated:  "rotated",
		session.EndEvicted:  "evicted",
		0:                   "EndReason(0)",
		200:                 "EndReason(200)",
	} {
		t.Must(be.Equal(want, reason.String()))
	}
}

// endLog records the calls of an OnEndReason hook, and whether Done was
// closed already when the hook ran.
type endLog struct {
	mu      sync.Mutex
	done    map[string]<-chan struct{}
	calls   map[string]int
	reasons map[string]session.EndReason
	values  map[int64]int
	late    int
}

// watch registers the Done channel of s, for the hook to check.
func (l *endLog) watch(s *session.Session[account]) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done == nil {
		l.done = map[string]<-chan struct{}{}
	}
	l.done[s.ID()] = s.Done()
}

func (l *endLog) hook(id string, reason session.EndReason, data *account) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls == nil {
		l.calls = map[string]int{}
		l.reasons = map[string]session.EndReason{}
		l.values = map[int64]int{}
	}
	l.calls[id]++
	l.reasons[id] = reason
	l.values[data.ID]++
	select {
	case <-l.done[id]:
		l.late++
	default:
	}
}

func (l *endLog) count(id string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls[id]
}

func (l *endLog) reason(id string) session.EndReason {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.reasons[id]
}

// valueCount returns how often the hook saw the value with the ID.
func (l *endLog) valueCount(id int64) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.values[id]
}

func (l *endLog) lateCalls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.late
}

func (l *endLog) total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, c := range l.calls {
		n += c
	}
	return n
}

func TestHookPanicStillEndsTheOthers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, end := range []func(*clock, *session.Store[account]) int{
		func(clk *clock, st *session.Store[account]) int { clk.Advance(time.Hour); return st.Sweep() },
		func(_ *clock, st *session.Store[account]) int { return st.EndAll() },
	} {
		clk := newClock()
		var mu sync.Mutex
		calls := 0
		st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: time.Hour, OnEnd: func(id string, data *account) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			clear(data.Secret)
			if calls == 1 {
				panic("hook bug")
			}
		}})
		var all []*session.Session[account]
		var secrets [][]byte
		for i := range 3 {
			secret := []byte("secret")
			s, _ := login(t, st, nil, account{ID: int64(i), Secret: secret})
			all = append(all, s)
			secrets = append(secrets, secret)
		}
		t.Must(be.Panic(func() { end(clk, st) }))
		t.Must(be.Equal(3, calls))
		t.Must(be.Equal(0, st.Len()))
		for i, s := range all {
			t.Must(be.True(s.Ended()))
			t.Must(be.True(closed(s)))
			_, ok := s.Data()
			t.Must(be.False(ok))
			t.Must(be.DeepEqual(make([]byte, 6), secrets[i]))
		}
	}
}

func TestRotate(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var log endLog
	st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: 2 * time.Hour, AbsoluteTTL: time.Hour, OnEndReason: func(id string, reason session.EndReason, data *account) {
		log.hook(id, reason, data)
		clear(data.Secret)
	}})
	secret := []byte("key")
	old, oldCookie := login(t, st, nil, account{ID: 1, Secret: secret})
	other, _ := login(t, st, nil, account{ID: 2})
	oldCSRF := old.CSRF()
	created := old.Created()
	clk.Advance(30 * time.Minute)

	rec := httptest.NewRecorder()
	s, err := st.Rotate(rec, get(t, "/reauth", oldCookie))
	t.Must(be.NoError(err))
	cookies := rec.Result().Cookies()
	t.Must(be.Len(cookies, 1))
	cookie := cookies[0]
	t.Must(be.Equal(oldCookie.Name, cookie.Name))
	t.Must(be.NotEqual(oldCookie.Value, cookie.Value))
	t.Must(be.Equal("; Path=/; HttpOnly; Secure; SameSite=Lax", strings.TrimPrefix(rec.Header().Get("Set-Cookie"), cookie.Name+"="+cookie.Value)))

	// The value moved, and the hook did not run.
	data, ok := s.Data()
	t.Must(be.True(ok))
	t.Must(be.Equal(int64(1), data.ID))
	t.Must(be.DeepEqual([]byte("key"), data.Secret))
	t.Must(be.Equal(0, log.total()))
	t.Must(be.NotEqual(old.ID(), s.ID()))
	t.Must(be.Equal(created, s.Created()))
	t.Must(be.Equal(2, st.Len()))
	t.Must(be.False(other.Ended()))

	// The old session, cookie and CSRF token are gone.
	t.Must(be.True(old.Ended()))
	t.Must(be.True(closed(old)))
	t.Must(be.Equal(session.EndRotated, old.EndReason()))
	_, ok = old.Data()
	t.Must(be.False(ok))
	t.Must(be.ErrorIs(old.CheckCSRF(oldCSRF), session.ErrCSRF))
	_, ok = st.Lookup(get(t, "/", oldCookie))
	t.Must(be.False(ok))
	t.Must(be.NotEqual(oldCSRF, s.CSRF()))
	t.Must(be.ErrorIs(s.CheckCSRF(oldCSRF), session.ErrCSRF))
	t.Must(be.NoError(s.CheckCSRF(s.CSRF())))
	got, ok := st.Lookup(get(t, "/", cookie))
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))

	// Ending the old session again does not touch the new one.
	old.End()
	t.Must(be.False(s.Ended()))

	// The absolute lifetime still counts from the login.
	clk.Advance(30 * time.Minute)
	_, ok = st.Lookup(get(t, "/", cookie))
	t.Must(be.False(ok))
	t.Must(be.Equal(session.EndAbsolute, s.EndReason()))
	t.Must(be.Equal(1, log.count(s.ID())))
	t.Must(be.DeepEqual([]byte{0, 0, 0}, secret))
}

func TestRotateNeedsALiveSession(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var e ended
	st := newStore(t, session.Options[account]{Now: clk.Now, IdleTTL: time.Minute, OnEnd: e.hook})
	s, cookie := login(t, st, nil, account{ID: 1})
	ended1, endedCookie := login(t, st, nil, account{ID: 2})
	ended1.End()

	for _, r := range []*http.Request{
		nil,
		get(t, "/"),
		get(t, "/", &http.Cookie{Name: cookie.Name, Value: "bogus"}),
		get(t, "/", &http.Cookie{Name: cookie.Name, Value: strings.Repeat("A", 43)}),
		get(t, "/", endedCookie),
	} {
		rec := httptest.NewRecorder()
		got, err := st.Rotate(rec, r)
		t.Must(be.ErrorIs(err, session.ErrEnded))
		t.Must(be.NilPtr(got))
		t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	}
	t.Must(be.False(s.Ended()))

	// An expired session ends, as at Lookup.
	clk.Advance(time.Minute)
	rec := httptest.NewRecorder()
	got, err := st.Rotate(rec, get(t, "/", cookie))
	t.Must(be.ErrorIs(err, session.ErrEnded))
	t.Must(be.NilPtr(got))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	t.Must(be.Equal(session.EndIdle, s.EndReason()))
	t.Must(be.DeepEqual([]string{ended1.ID(), s.ID()}, e.list()))
	t.Must(be.Equal(0, st.Len()))
}

func TestRotateFailsWithoutRandomness(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cause := errors.New("no entropy")
	src := &switchable{r: seeded(5)}
	st := newStore(t, session.Options[account]{Rand: src})
	s, cookie := login(t, st, nil, account{ID: 1})

	src.set(failingRand{cause})
	rec := httptest.NewRecorder()
	got, err := st.Rotate(rec, get(t, "/", cookie))
	t.Must(be.ErrorIs(err, cause))
	t.Must(be.NilPtr(got))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	t.Must(be.False(s.Ended()))

	// A broken source that repeats the token is refused too.
	src.set(zeros{})
	_, zeroCookie := login(t, st, nil, account{ID: 2})
	got, err = st.Rotate(rec, get(t, "/", zeroCookie))
	t.Must(be.Error(err))
	t.Must(be.NilPtr(got))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
	_, ok := st.Lookup(get(t, "/", zeroCookie))
	t.Must(be.True(ok))
	t.Must(be.False(s.Ended()))
}

func TestEndFuncChecksRotatedSessions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	_, bCookie := login(t, st, nil, account{ID: 2})
	login(t, st, nil, account{ID: 1}) // the most recent: checked first

	// While EndFunc runs, session 2 rotates (match calls the store here
	// only to make the race deterministic: session 2 is not locked).
	var rotated *session.Session[account]
	n := st.EndFunc(func(a account) bool {
		if a.ID == 1 {
			var err error
			rotated, err = st.Rotate(httptest.NewRecorder(), get(t, "/", bCookie))
			t.Must(be.NoError(err))
		}
		return a.ID == 2
	})
	t.Must(be.Equal(1, n))
	t.Must(be.NonNilPtr(rotated))
	t.Must(be.True(rotated.Ended()))
	t.Must(be.Equal(session.EndRevoked, rotated.EndReason()))
	t.Must(be.Equal(1, st.Len()))
}

func TestOnEndOnceUnderRacingEnders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	var log endLog
	st := newStore(t, session.Options[account]{Now: clk.Now, MaxSessions: 56, OnEndReason: log.hook})
	type started struct {
		s      *session.Session[account]
		cookie *http.Cookie
	}
	var all []started
	for i := range 64 {
		s, cookie := login(t, st, nil, account{ID: int64(i)})
		log.watch(s)
		all = append(all, started{s, cookie})
	}
	// Every goroutine waits for ready, so that they all race.
	ready := make(chan struct{})
	var wg sync.WaitGroup
	race := func(fn func()) {
		wg.Go(func() {
			<-ready
			fn()
		})
	}
	rotate := func(cookie *http.Cookie) {
		rec := httptest.NewRecorder()
		s, err := st.Rotate(rec, get(t, "/", cookie))
		if err != nil {
			return
		}
		log.watch(s)
		// Rotate the new session once more.
		if s, err := st.Rotate(httptest.NewRecorder(), get(t, "/", rec.Result().Cookies()[0])); err == nil {
			log.watch(s)
		}
	}
	for i, x := range all {
		id := int64(i)
		race(func() { rotate(x.cookie) })
		race(func() { rotate(x.cookie) })
		race(func() { x.s.End() })
		race(func() { st.End(httptest.NewRecorder(), get(t, "/logout", x.cookie)) })
		race(func() { st.EndSession(httptest.NewRecorder(), x.s) })
		race(func() { st.EndFunc(func(a account) bool { return a.ID == id }) })
		if i%16 == 15 {
			race(func() { st.EndAll() })
		}
		if i%8 == 7 {
			race(func() { clk.Advance(2 * time.Minute); st.Sweep() })
			race(func() { _, _ = st.New(httptest.NewRecorder(), nil, account{ID: 1000 + id}) })
		}
	}
	close(ready)
	wg.Wait()
	st.EndAll()
	t.Must(be.Equal(0, st.Len()))
	for i, x := range all {
		t.Must(be.True(closed(x.s)))
		t.Mustf(be.Equal(1, log.valueCount(int64(i))), "value %d", i)
	}
	t.Must(be.Equal(0, log.lateCalls()))
}

func TestConcurrentUse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st, err := session.NewStore(session.Options[account]{Now: clk.Now, MaxSessions: 16})
	t.Must(be.NoError(err))
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			var cookie *http.Cookie
			for i := range 200 {
				rec := httptest.NewRecorder()
				var r *http.Request
				if cookie != nil {
					r = httptest.NewRequest(http.MethodPost, "/login", nil)
					r.AddCookie(cookie)
				}
				s, err := st.New(rec, r, account{ID: int64(g)})
				if err != nil {
					panic(err)
				}
				cookie = rec.Result().Cookies()[0]
				lr := httptest.NewRequest(http.MethodGet, "/", nil)
				lr.AddCookie(cookie)
				if got, ok := st.Lookup(lr); ok {
					_ = got.Update(func(a *account) { a.ID++ })
					_, _ = got.Data()
					_ = got.CheckCSRF(got.CSRF())
				}
				switch i % 50 {
				case 5:
					if rotated, err := st.Rotate(httptest.NewRecorder(), lr); err == nil {
						s = rotated
					}
				case 10:
					s.End()
				case 20:
					st.Sweep()
				case 30:
					st.EndFunc(func(a account) bool { return a.ID%7 == 0 })
				case 40:
					clk.Advance(time.Minute)
				case 49:
					if g == 0 {
						st.EndAll()
					}
				}
				_ = st.Len()
			}
		})
	}
	wg.Wait()
	t.Must(be.LessOrEq(st.Len(), 16))
}
