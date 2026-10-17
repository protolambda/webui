package session_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
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

func newPreAuth(t devtest.T, opts session.PreAuthOptions) *session.PreAuth {
	if opts.Rand == nil {
		opts.Rand = seeded(2)
	}
	p, err := session.NewPreAuth(opts)
	t.Must(be.NoError(err))
	return p
}

// issue issues a pre-auth form for r and returns its token and cookie (nil
// if none was set).
func issue(t devtest.T, p *session.PreAuth, r *http.Request) (string, *http.Cookie) {
	rec := httptest.NewRecorder()
	token, err := p.Issue(rec, r)
	t.Must(be.NoError(err))
	cookies := rec.Result().Cookies()
	t.Must(be.LessOrEq(len(cookies), 1))
	if len(cookies) == 0 {
		return token, nil
	}
	return token, cookies[0]
}

func post(t devtest.T, cookies ...*http.Cookie) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
	for _, c := range cookies {
		r.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	return r
}

func TestPreAuthIssueAndVerify(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p := newPreAuth(t, session.PreAuthOptions{})
	t.Must(be.Equal("__Host-preauth", p.CookieName()))

	rec := httptest.NewRecorder()
	token, err := p.Issue(rec, nil)
	t.Must(be.NoError(err))
	t.Must(be.Equal(43, len(token)))
	header := rec.Header().Get("Set-Cookie")
	name, rest, _ := strings.Cut(header, "=")
	value, attrs, _ := strings.Cut(rest, ";")
	t.Must(be.Equal("__Host-preauth", name))
	t.Must(be.Equal(96, len(value)))
	t.Must(be.Equal(" Path=/; Max-Age=600; HttpOnly; Secure; SameSite=Lax", attrs))
	cookie := rec.Result().Cookies()[0]

	t.Must(be.NoError(p.Verify(post(t, cookie), token)))
	// Stateless: the same pair verifies again.
	t.Must(be.NoError(p.Verify(post(t, cookie), token)))

	// Forms are independent: each visit gets its own pair.
	other, otherCookie := issue(t, p, nil)
	t.Must(be.NotEqual(token, other))
	t.Must(be.NoError(p.Verify(post(t, otherCookie), other)))
	t.Must(be.ErrorIs(p.Verify(post(t, cookie), other), session.ErrCSRF))
	t.Must(be.ErrorIs(p.Verify(post(t, otherCookie), token), session.ErrCSRF))

	rec = httptest.NewRecorder()
	p.ClearCookie(rec)
	t.Must(be.Equal("__Host-preauth=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax", rec.Header().Get("Set-Cookie")))
}

func TestPreAuthCookieOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p := newPreAuth(t, session.PreAuthOptions{CookieName: "login", Insecure: true, StrictSameSite: true, TTL: 90 * time.Second})
	t.Must(be.Equal("login", p.CookieName()))
	rec := httptest.NewRecorder()
	_, err := p.Issue(rec, nil)
	t.Must(be.NoError(err))
	_, rest, _ := strings.Cut(rec.Header().Get("Set-Cookie"), ";")
	t.Must(be.Equal(" Path=/; Max-Age=90; HttpOnly; SameSite=Strict", rest))
}

func TestPreAuthExpiry(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	token, cookie := issue(t, p, nil)

	clk.Advance(session.DefaultPreAuthTTL - time.Second)
	t.Must(be.NoError(p.Verify(post(t, cookie), token)))
	clk.Advance(time.Second)
	err := p.Verify(post(t, cookie), token)
	t.Must(be.ErrorIs(err, session.ErrCSRF))
	t.Must(be.ErrorContains(err, "expired"))
}

func TestPreAuthClockSkew(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now, Skew: 30 * time.Second})
	token, cookie := issue(t, p, nil)

	clk.Advance(-30 * time.Second) // the wall clock stepped back
	t.Must(be.NoError(p.Verify(post(t, cookie), token)))
	clk.Advance(-time.Second)
	err := p.Verify(post(t, cookie), token)
	t.Must(be.ErrorIs(err, session.ErrCSRF))
	t.Must(be.ErrorContains(err, "future"))

	// The default skew is a minute.
	q := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	token, cookie = issue(t, q, nil)
	clk.Advance(-session.DefaultPreAuthSkew)
	t.Must(be.NoError(q.Verify(post(t, cookie), token)))
	clk.Advance(-time.Second)
	t.Must(be.ErrorIs(q.Verify(post(t, cookie), token), session.ErrCSRF))
}

// flip returns s with the character at i replaced by another base64url
// character.
func flip(s string, i int) string {
	b := []byte(s)
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

func TestPreAuthRefusesTampering(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p := newPreAuth(t, session.PreAuthOptions{})
	token, cookie := issue(t, p, nil)

	for i := range len(cookie.Value) {
		forged := &http.Cookie{Name: cookie.Name, Value: flip(cookie.Value, i)}
		t.Mustf(be.ErrorIs(p.Verify(post(t, forged), token), session.ErrCSRF), "cookie byte %d", i)
	}
	for i := range len(token) {
		t.Mustf(be.ErrorIs(p.Verify(post(t, cookie), flip(token, i)), session.ErrCSRF), "token byte %d", i)
	}

	// The cookie's tag and the form token are separate MACs: a cookie
	// carrying its own form token as tag is a forgery.
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	t.Must(be.NoError(err))
	formMAC, err := base64.RawURLEncoding.DecodeString(token)
	t.Must(be.NoError(err))
	spliced := append(raw[:40:40], formMAC...)
	forged := &http.Cookie{Name: cookie.Name, Value: base64.RawURLEncoding.EncodeToString(spliced)}
	t.Must(be.ErrorIs(p.Verify(post(t, forged), token), session.ErrCSRF))

	// Another key (another process, or a restart) does not accept the form.
	q := newPreAuth(t, session.PreAuthOptions{Rand: seeded(9)})
	err = q.Verify(post(t, cookie), token)
	t.Must(be.ErrorIs(err, session.ErrCSRF))
	t.Must(be.ErrorContains(err, "forged"))
}

func TestPreAuthChecksLengthsFirst(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p := newPreAuth(t, session.PreAuthOptions{})
	token, cookie := issue(t, p, nil)

	for _, value := range []string{
		cookie.Value[1:], cookie.Value + "A", strings.Repeat("A", 100_000),
		strings.Repeat("!", 96), cookie.Value[:95] + "=", token,
	} {
		err := p.Verify(post(t, &http.Cookie{Name: cookie.Name, Value: value}), token)
		t.Must(be.ErrorIs(err, session.ErrCSRF))
		t.Mustf(be.ErrorContains(err, "malformed"), "value of length %d", len(value))
	}
	for _, bad := range []string{"", token[1:], token + "A", strings.Repeat(token, 100), cookie.Value} {
		t.Must(be.ErrorIs(p.Verify(post(t, cookie), bad), session.ErrCSRF))
	}

	err := p.Verify(post(t), token)
	t.Must(be.ErrorIs(err, session.ErrCSRF))
	t.Must(be.ErrorContains(err, "no pre-auth cookie"))
	// A cookie of another name does not count.
	t.Must(be.ErrorIs(p.Verify(post(t, &http.Cookie{Name: "preauth", Value: cookie.Value}), token), session.ErrCSRF))
}

func TestPreAuthErrorsDoNotQuoteInput(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	token, cookie := issue(t, p, nil)
	var errs []error
	errs = append(errs, p.Verify(post(t, cookie), flip(token, 3)))
	errs = append(errs, p.Verify(post(t, &http.Cookie{Name: cookie.Name, Value: flip(cookie.Value, 50)}), token))
	clk.Advance(time.Hour)
	errs = append(errs, p.Verify(post(t, cookie), token))
	for _, err := range errs {
		t.Must(be.Error(err))
		t.Must(be.False(strings.Contains(err.Error(), token)))
		t.Must(be.False(strings.Contains(err.Error(), cookie.Value[:20])))
	}
}

func TestPreAuthIssueReusesAFreshCookie(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	token, cookie := issue(t, p, nil)

	// A second tab within the first half of the cookie's life: same form.
	clk.Advance(session.DefaultPreAuthTTL/2 - time.Second)
	again, set := issue(t, p, post(t, cookie))
	t.Must(be.Equal(token, again))
	t.Must(be.NilPtr(set))

	// Later, a new cookie, so the new form has a full lifetime.
	clk.Advance(time.Second)
	fresh, set := issue(t, p, post(t, cookie))
	t.Must(be.NonNilPtr(set))
	t.Must(be.Equal(cookie.Name, set.Name))
	t.Must(be.NotEqual(token, fresh))
	t.Must(be.NoError(p.Verify(post(t, set), fresh)))
	// The browser replaced the cookie: the earlier tabs' forms are refused.
	t.Must(be.ErrorIs(p.Verify(post(t, set), token), session.ErrCSRF))

	// An invalid cookie is replaced.
	_, set = issue(t, p, post(t, &http.Cookie{Name: cookie.Name, Value: "junk"}))
	t.Must(be.NonNilPtr(set))
}

func TestNewPreAuthRejectsBadOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, opts := range []session.PreAuthOptions{
		{CookieName: "__Host-preauth"},
		{CookieName: "bad name"},
		{TTL: -time.Minute},
		{Skew: -time.Minute},
		{TTL: 500 * time.Millisecond},
		{Rand: failingRand{errors.New("no entropy")}},
	} {
		p, err := session.NewPreAuth(opts)
		t.Mustf(be.Error(err), "options %+v", opts)
		t.Must(be.NilPtr(p))
	}
}

func TestPreAuthIssueFailsWithoutRandomness(gt *testing.T) {
	t := devtest.ParallelT(gt)
	src := &switchable{r: seeded(4)}
	p := newPreAuth(t, session.PreAuthOptions{Rand: src})
	cause := errors.New("no entropy")
	src.set(failingRand{cause})
	rec := httptest.NewRecorder()
	token, err := p.Issue(rec, nil)
	t.Must(be.ErrorIs(err, cause))
	t.Must(be.Equal("", token))
	t.Must(be.Len(rec.Header().Values("Set-Cookie"), 0))
}

func TestPreAuthFormattingHidesTheKey(gt *testing.T) {
	t := devtest.ParallelT(gt)
	key := bytes.Repeat([]byte{0xAB}, 32)
	p := newPreAuth(t, session.PreAuthOptions{Rand: bytes.NewReader(append(key, make([]byte, 1024)...))})
	type wrapper struct{ P *session.PreAuth }
	for _, verb := range formatVerbs {
		for _, v := range []any{p, wrapper{p}} {
			out := fmt.Sprintf(verb, v)
			t.Mustf(be.False(strings.Contains(out, "171")), "%s: %s", verb, out)
			t.Mustf(be.False(strings.Contains(out, "253")), "%s: %s", verb, out) // octal
			t.Mustf(be.False(strings.Contains(out, "10101011")), "%s: %s", verb, out)
			t.Mustf(be.False(strings.Contains(strings.ToLower(out), "abab")), "%s: %s", verb, out)
		}
	}
	t.Must(be.Equal("session.PreAuth(__Host-preauth)", fmt.Sprint(p)))
	t.Must(be.Equal("session.PreAuth(__Host-preauth)", fmt.Sprintf("%d", p)))
	t.Must(be.Equal(p.String(), p.GoString()))
}

func TestPreAuthConcurrentUse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p := newPreAuth(t, session.PreAuthOptions{}) // a deterministic source, serialized by PreAuth
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			for range 100 {
				rec := httptest.NewRecorder()
				token, err := p.Issue(rec, nil)
				if err == nil {
					r := httptest.NewRequest(http.MethodPost, "/login", nil)
					r.AddCookie(rec.Result().Cookies()[0])
					err = p.Verify(r, token)
				}
				if err != nil {
					failures <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Must(be.NoError(err))
	}
}
