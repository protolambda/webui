package session_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/session"
)

// attached returns r as the handlers behind st's middleware see it.
func attached(st *session.Store[account], r *http.Request) *http.Request {
	var seen *http.Request
	st.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r
	})).ServeHTTP(httptest.NewRecorder(), r)
	return seen
}

// submit returns a POST request with an urlencoded form, carrying cookies
// (name and value only, as a browser sends them).
func submit(t devtest.T, values url.Values, cookies ...*http.Cookie) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", form.URLEncoded)
	for _, c := range cookies {
		r.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	return r
}

// setCookies returns the cookies set on rec.
func setCookies(rec *httptest.ResponseRecorder) []*http.Cookie {
	return rec.Result().Cookies()
}

func TestPostForm(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var out strings.Builder
	node := session.PostForm("/tasks", "tok-1", attr.ID("add"), attr.Class("mb-3"))(
		text.Span()(text.Text("fields")),
	)
	t.Must(be.NoError(core.Render(t.Context(), node, &out)))
	t.Must(be.Equal(`<form method="post" action="/tasks" id="add" class="mb-3">`+
		`<input type="hidden" name="csrf" value="tok-1"/><span>fields</span></form>`, out.String()))

	// Without children, the form holds the token alone.
	out.Reset()
	t.Must(be.NoError(core.Render(t.Context(), session.PostForm("/logout", "tok-2")(), &out)))
	t.Must(be.Equal(`<form method="post" action="/logout"><input type="hidden" name="csrf" value="tok-2"/></form>`, out.String()))

	// The action and the token are escaped like any attribute value.
	out.Reset()
	t.Must(be.NoError(core.Render(t.Context(), session.PostForm(`/x"><b>`, `"><b>x</b>`)(), &out)))
	t.Must(be.False(strings.Contains(out.String(), "<b>")))

	page := ct.View(session.PostForm("/tasks", "tok")(text.Span()()))
	t.Must(page.Find(ct.Tag("form"), ct.Attr("method", "post"), ct.Attr("action", "/tasks")))
	t.Must(page.Find(ct.Tag("input"), ct.Attr("type", "hidden"), ct.Attr("name", session.CSRFField), ct.Attr("value", "tok")))

	// A scope renders the same form every time it is called.
	scope := session.PostForm("/a", "t")
	var first, second strings.Builder
	t.Must(be.NoError(core.Render(t.Context(), scope(text.Text("1")), &first)))
	t.Must(be.NoError(core.Render(t.Context(), scope(text.Text("2")), &second)))
	t.Must(be.Equal(`<form method="post" action="/a"><input type="hidden" name="csrf" value="t"/>1</form>`, first.String()))
	t.Must(be.Equal(`<form method="post" action="/a"><input type="hidden" name="csrf" value="t"/>2</form>`, second.String()))
}

func TestFormsSignedIn(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	p := newPreAuth(t, session.PreAuthOptions{})
	forms := session.Forms[account]{Store: st, PreAuth: p}
	s, cookie := login(t, st, nil, account{ID: 1})

	r := attached(st, get(t, "/notes", cookie))
	got, ok := forms.Session(r)
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))

	// Pages and fragments carry the session's token and set no cookie.
	rec := httptest.NewRecorder()
	token, err := forms.PageToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal(s.CSRF(), token))
	t.Must(be.Len(setCookies(rec), 0))

	rec = httptest.NewRecorder()
	token, renewed, err := forms.FragmentToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal(s.CSRF(), token))
	t.Must(be.False(renewed))
	t.Must(be.Len(setCookies(rec), 0))

	// The session's token is accepted.
	v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{
		session.CSRFField: {s.CSRF()},
		"note":            {"buy milk"},
	}, cookie)))
	t.Must(be.NoError(err))
	t.Must(be.Equal("buy milk", v.String("note")))
	v.Zero()
	t.Must(be.NoError(forms.Check(r, s.CSRF())))

	// A wrong, empty or missing token is refused, without values.
	for _, values := range []url.Values{
		{session.CSRFField: {"forged"}, "note": {"x"}},
		{session.CSRFField: {""}, "note": {"x"}},
		{"note": {"x"}},
	} {
		v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, values, cookie)))
		t.Must(be.ErrorIs(err, session.ErrCSRF))
		t.Must(be.NilPtr(v))
	}

	// A valid pre-authentication pair does not stand in for the session's
	// token: a form rendered before the login is refused.
	preToken, preCookie := issue(t, p, nil)
	t.Must(be.NoError(p.Verify(submit(t, nil, preCookie), preToken)))
	_, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {preToken}}, cookie, preCookie)))
	t.Must(be.ErrorIs(err, session.ErrCSRF))

	// Once the session ended, its token is refused too.
	csrf := s.CSRF()
	s.End()
	_, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {csrf}}, cookie)))
	t.Must(be.ErrorIs(err, session.ErrCSRF))
}

func TestFormsAnonymous(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	forms := session.Forms[account]{Store: st, PreAuth: p}

	// The first page issues a pre-authentication cookie.
	r := attached(st, get(t, "/signup"))
	_, ok := forms.Session(r)
	t.Must(be.False(ok))
	rec := httptest.NewRecorder()
	token, err := forms.PageToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 1))
	cookie := setCookies(rec)[0]
	t.Must(be.Equal(p.CookieName(), cookie.Name))

	// Pages and fragments of that cookie share its token and set nothing.
	r = attached(st, get(t, "/signup", cookie))
	rec = httptest.NewRecorder()
	again, err := forms.PageToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal(token, again))
	t.Must(be.Len(setCookies(rec), 0))

	rec = httptest.NewRecorder()
	fragment, renewed, err := forms.FragmentToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal(token, fragment))
	t.Must(be.False(renewed))
	t.Must(be.Len(setCookies(rec), 0))

	// The pair is accepted; without the cookie, or with another token, the
	// form is refused.
	v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {token}, "name": {"ada"}}, cookie)))
	t.Must(be.NoError(err))
	t.Must(be.Equal("ada", v.String("name")))
	v.Zero()
	_, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {token}})))
	t.Must(be.ErrorIs(err, session.ErrCSRF))
	_, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {"forged"}}, cookie)))
	t.Must(be.ErrorIs(err, session.ErrCSRF))

	// Past half of the cookie's lifetime, a fragment still reads the
	// page's token and renews nothing; a whole page renews the cookie.
	clk.Advance(session.DefaultPreAuthTTL/2 + time.Minute)
	rec = httptest.NewRecorder()
	fragment, renewed, err = forms.FragmentToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal(token, fragment))
	t.Must(be.False(renewed))
	t.Must(be.Len(setCookies(rec), 0))
	rec = httptest.NewRecorder()
	fresh, err := forms.PageToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.NotEqual(token, fresh))
	t.Must(be.Len(setCookies(rec), 1))

	// Once the cookie expired, the fragment falls back to a new cookie and
	// reports it, so that it can renew the page's forms.
	clk.Advance(session.DefaultPreAuthTTL / 2)
	rec = httptest.NewRecorder()
	fragment, renewed, err = forms.FragmentToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.True(renewed))
	t.Must(be.NotEqual(token, fragment))
	t.Must(be.Len(setCookies(rec), 1))
	renewedCookie := setCookies(rec)[0]
	_, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {token}}, cookie)))
	t.Must(be.ErrorIs(err, session.ErrCSRF))
	v, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {fragment}}, renewedCookie)))
	t.Must(be.NoError(err))
	v.Zero()

	// A session's token from a visitor without the session is refused.
	s, _ := login(t, st, nil, account{ID: 2})
	_, err = forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {s.CSRF()}}, renewedCookie)))
	t.Must(be.ErrorIs(err, session.ErrCSRF))
}

func TestFormsWithoutPreAuth(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	forms := session.Forms[account]{Store: st}

	// Visitors without a session get the empty token and no cookie.
	r := attached(st, get(t, "/"))
	rec := httptest.NewRecorder()
	token, err := forms.PageToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal("", token))
	t.Must(be.Len(setCookies(rec), 0))
	token, renewed, err := forms.FragmentToken(rec, r)
	t.Must(be.NoError(err))
	t.Must(be.Equal("", token))
	t.Must(be.False(renewed))
	t.Must(be.Len(setCookies(rec), 0))

	// Every form of theirs is refused, the empty token included.
	for _, token := range []string{"", "anything"} {
		v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {token}})))
		t.Must(be.ErrorIs(err, session.ErrCSRF))
		t.Must(be.NilPtr(v))
		t.Must(be.ErrorIs(forms.Check(r, token), session.ErrCSRF))
	}

	// Visitors with a session are served as usual.
	s, cookie := login(t, st, nil, account{ID: 1})
	v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {s.CSRF()}}, cookie)))
	t.Must(be.NoError(err))
	v.Zero()
}

func TestFormsSession(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	s, cookie := login(t, st, nil, account{ID: 1})
	withStore := session.Forms[account]{Store: st}
	contextOnly := session.Forms[account]{}

	// Without the middleware, the store finds the cookie's session; the
	// zero Forms finds none.
	got, ok := withStore.Session(get(t, "/", cookie))
	t.Must(be.True(ok))
	t.Must(be.Same(s, got))
	_, ok = contextOnly.Session(get(t, "/", cookie))
	t.Must(be.False(ok))

	// A session attached to the context wins over the cookie, as with
	// FromContext.
	detached := session.NewDetached(account{ID: 9}, "test-csrf")
	r := get(t, "/", cookie)
	r = r.WithContext(session.WithSession(r.Context(), detached))
	for _, forms := range []session.Forms[account]{withStore, contextOnly} {
		got, ok = forms.Session(r)
		t.Must(be.True(ok))
		t.Must(be.Same(detached, got))
	}
	v, err := contextOnly.Read(httptest.NewRecorder(), withSession(submit(t, url.Values{session.CSRFField: {"test-csrf"}}), detached))
	t.Must(be.NoError(err))
	v.Zero()

	// A nil session attached with WithSession hides the cookie's session.
	r = get(t, "/", cookie)
	r = r.WithContext(session.WithSession[account](r.Context(), nil))
	_, ok = withStore.Session(r)
	t.Must(be.False(ok))

	// An ended session attached to the context means no session: the
	// cookie is not consulted.
	gone := session.NewDetached(account{ID: 8}, "gone-csrf")
	gone.End()
	r = get(t, "/", cookie)
	r = r.WithContext(session.WithSession(r.Context(), gone))
	_, ok = withStore.Session(r)
	t.Must(be.False(ok))
	_, err = withStore.Read(httptest.NewRecorder(), withSession(submit(t, url.Values{session.CSRFField: {s.CSRF()}}, cookie), gone))
	t.Must(be.ErrorIs(err, session.ErrCSRF))

	// Behind the middleware, Session finds what FromContext finds.
	r = attached(st, get(t, "/", cookie))
	fromContext, _ := session.FromContext[account](r.Context())
	got, _ = withStore.Session(r)
	t.Must(be.Same(fromContext, got))
	r = attached(st, get(t, "/"))
	_, ok = withStore.Session(r)
	t.Must(be.False(ok))
}

// withSession returns r with s attached to its context.
func withSession(r *http.Request, s *session.Session[account]) *http.Request {
	return r.WithContext(session.WithSession(r.Context(), s))
}

func TestFormsReadBounds(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := session.NewDetached(account{ID: 1}, "test-csrf")
	forms := session.Forms[account]{Limits: form.Limits{Body: 64}}

	ok := withSession(submit(t, url.Values{session.CSRFField: {"test-csrf"}, "note": {"short"}}), s)
	v, err := forms.Read(httptest.NewRecorder(), ok)
	t.Must(be.NoError(err))
	t.Must(be.Equal("short", v.String("note")))
	v.Zero()

	// The limits apply, and a valid token does not lift them.
	long := withSession(submit(t, url.Values{session.CSRFField: {"test-csrf"}, "note": {strings.Repeat("x", 100)}}), s)
	v, err = forms.Read(httptest.NewRecorder(), long)
	t.Must(be.ErrorIs(err, form.ErrTooLarge))
	t.Must(be.NilPtr(v))

	plain := withSession(submit(t, url.Values{session.CSRFField: {"test-csrf"}}), s)
	plain.Header.Set("Content-Type", "text/plain")
	_, err = forms.Read(httptest.NewRecorder(), plain)
	t.Must(be.ErrorIs(err, form.ErrUnsupportedType))

	malformed := withSession(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", strings.NewReader("csrf=%zz")), s)
	malformed.Header.Set("Content-Type", form.URLEncoded)
	_, err = forms.Read(httptest.NewRecorder(), malformed)
	t.Must(be.ErrorIs(err, form.ErrMalformed))
	t.Must(be.False(errors.Is(err, session.ErrCSRF)))

	// The token in the query string does not count: forms carry it in
	// their body.
	query := withSession(submit(t, url.Values{"note": {"x"}}), s)
	query.URL.RawQuery = session.CSRFField + "=test-csrf"
	_, err = forms.Read(httptest.NewRecorder(), query)
	t.Must(be.ErrorIs(err, session.ErrCSRF))

	// Only the first CSRF field counts.
	twice := withSession(submit(t, url.Values{session.CSRFField: {"forged", "test-csrf"}}), s)
	_, err = forms.Read(httptest.NewRecorder(), twice)
	t.Must(be.ErrorIs(err, session.ErrCSRF))
}

func TestFormsFailWithoutRandomness(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cause := errors.New("no entropy")
	src := &switchable{r: seeded(4)}
	p := newPreAuth(t, session.PreAuthOptions{Rand: src})
	forms := session.Forms[account]{PreAuth: p}
	token, cookie := issue(t, p, nil)

	src.set(failingRand{cause})
	rec := httptest.NewRecorder()
	got, err := forms.PageToken(rec, get(t, "/signup"))
	t.Must(be.ErrorIs(err, cause))
	t.Must(be.Equal("", got))
	t.Must(be.Len(setCookies(rec), 0))

	got, renewed, err := forms.FragmentToken(rec, get(t, "/signup/check"))
	t.Must(be.ErrorIs(err, cause))
	t.Must(be.Equal("", got))
	t.Must(be.False(renewed))
	t.Must(be.Len(setCookies(rec), 0))

	// A fragment of a page with a valid cookie needs no randomness.
	got, renewed, err = forms.FragmentToken(rec, get(t, "/signup/check", cookie))
	t.Must(be.NoError(err))
	t.Must(be.Equal(token, got))
	t.Must(be.False(renewed))
}

func TestFormsConcurrentUse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	p := newPreAuth(t, session.PreAuthOptions{})
	forms := session.Forms[account]{Store: st, PreAuth: p}
	s, cookie := login(t, st, nil, account{ID: 1})
	token, preCookie := issue(t, p, nil)

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := range 16 {
		wg.Go(func() {
			r := submit(t, url.Values{session.CSRFField: {token}}, preCookie)
			if i%2 == 0 {
				r = attached(st, submit(t, url.Values{session.CSRFField: {s.CSRF()}}, cookie))
			}
			if _, err := forms.PageToken(httptest.NewRecorder(), r); err != nil {
				errs <- err
				return
			}
			v, err := forms.Read(httptest.NewRecorder(), r)
			if err != nil {
				errs <- err
				return
			}
			v.Zero()
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Must(be.NoError(err))
	}
}
