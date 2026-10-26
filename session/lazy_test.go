package session_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
)

// csrfValues returns the values of the CSRF fields in body, in order.
func csrfValues(body string) []string {
	const field = `name="` + session.CSRFField + `" value="`
	var values []string
	for {
		_, rest, ok := strings.Cut(body, field)
		if !ok {
			return values
		}
		var value string
		value, body, _ = strings.Cut(rest, `"`)
		values = append(values, value)
	}
}

// notesView renders forms deep in its tree, without a token: a list with a
// delete form per note and, below it, the add form.
func notesView(notes ...string) elem.Node {
	items := make([]elem.Node, 0, len(notes))
	for _, note := range notes {
		items = append(items, list.LI()(text.Text(note), session.PostFormFromContext("/notes/delete")(text.Text("delete"))))
	}
	return section.Main()(div.Div(attr.ID("notes"))(
		list.UL()(items...),
		div.Div()(session.PostFormFromContext("/notes", attr.ID("add-note"))(text.Text("add"))),
	))
}

// render serves r with web.Render: the node that wrap returns for the
// response.
func render(r *http.Request, wrap func(w http.ResponseWriter, r *http.Request) elem.Node) (*httptest.ResponseRecorder, error) {
	rec := httptest.NewRecorder()
	return rec, web.Render(rec, r, http.StatusOK, wrap(rec, r))
}

func TestWithPageToken(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	forms := session.Forms[account]{Store: st, PreAuth: p}
	page := func(view elem.Node) func(w http.ResponseWriter, r *http.Request) elem.Node {
		return func(w http.ResponseWriter, r *http.Request) elem.Node { return forms.WithPageToken(w, r, view) }
	}

	// A visitor without a session gets a pre-authentication cookie, once,
	// and every form of the page its token, which Read accepts.
	rec, err := render(attached(st, get(t, "/notes")), page(notesView("milk", "eggs")))
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([]string{p.CookieName()}, cookieNames(rec)))
	cookie := setCookies(rec)[0]
	tokens := csrfValues(rec.Body.String())
	t.Must(be.Len(tokens, 3))
	token, ok := p.Token(get(t, "/notes", cookie))
	t.Must(be.True(ok))
	t.Must(be.DeepEqual([]string{token, token, token}, tokens))
	v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {tokens[1]}, "note": {"bread"}}, cookie)))
	t.Must(be.NoError(err))
	t.Must(be.Equal("bread", v.String("note")))
	v.Zero()

	// With that cookie, the page sets nothing and keeps the token.
	rec, err = render(attached(st, get(t, "/notes", cookie)), page(notesView("milk")))
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.DeepEqual([]string{token, token}, csrfValues(rec.Body.String())))

	// A page without forms takes no token, so it sets no cookie.
	rec, err = render(attached(st, get(t, "/about")), page(text.P()(text.Text("About"))))
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.Equal("<p>About</p>", rec.Body.String()))

	// A signed-in visitor's forms carry the session's token.
	s, sessionCookie := login(t, st, nil, account{ID: 1})
	rec, err = render(attached(st, get(t, "/notes", sessionCookie)), page(notesView("milk")))
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.DeepEqual([]string{s.CSRF(), s.CSRF()}, csrfValues(rec.Body.String())))
}

// The page variant renews a pre-authentication cookie past half of its
// lifetime; the fragment variant keeps the page's cookie and token, and
// renews only when the cookie is gone, which IfRenewed reports.
func TestWithFragmentToken(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	forms := session.Forms[account]{Store: st, PreAuth: p}
	// The fragment swaps in the list, and with a new cookie also the add
	// form, out of band.
	fragment := func(w http.ResponseWriter, r *http.Request) elem.Node {
		return forms.WithFragmentToken(w, r, elem.Bundle{
			session.PostFormFromContext("/notes/delete")(text.Text("delete")),
			session.IfRenewed(session.PostFormFromContext("/notes", attr.ID("add-note"))(text.Text("add"))),
		})
	}
	page := func(w http.ResponseWriter, r *http.Request) elem.Node {
		return forms.WithPageToken(w, r, notesView("milk"))
	}

	token, cookie := issue(t, p, nil)
	clk.Advance(session.DefaultPreAuthTTL/2 + time.Minute)
	rec, err := render(attached(st, get(t, "/notes/list", cookie)), fragment)
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.DeepEqual([]string{token}, csrfValues(rec.Body.String())))
	rec, err = render(attached(st, get(t, "/notes", cookie)), page)
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([]string{p.CookieName()}, cookieNames(rec)))
	t.Must(be.NotEqual(token, csrfValues(rec.Body.String())[0]))

	// Once the page's cookie expired, the fragment issues a new one, and
	// renders the add form too; its forms are accepted with the new cookie.
	clk.Advance(session.DefaultPreAuthTTL / 2)
	rec, err = render(attached(st, get(t, "/notes/list", cookie)), fragment)
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([]string{p.CookieName()}, cookieNames(rec)))
	fresh := csrfValues(rec.Body.String())
	t.Must(be.Len(fresh, 2))
	t.Must(be.Equal(fresh[0], fresh[1]))
	t.Must(be.True(strings.Contains(rec.Body.String(), `id="add-note"`)))
	v, err := forms.Read(httptest.NewRecorder(), attached(st, submit(t, url.Values{session.CSRFField: {fresh[1]}}, setCookies(rec)[0])))
	t.Must(be.NoError(err))
	v.Zero()

	// IfRenewed takes the token itself when no form did before it.
	rec, err = render(attached(st, get(t, "/notes/list")), func(w http.ResponseWriter, r *http.Request) elem.Node {
		return forms.WithFragmentToken(w, r, session.IfRenewed(text.Text("renewed")))
	})
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 1))
	t.Must(be.Equal("renewed", rec.Body.String()))

	// A signed-in visitor's fragment renews nothing.
	s, sessionCookie := login(t, st, nil, account{ID: 1})
	rec, err = render(attached(st, get(t, "/notes/list", sessionCookie)), fragment)
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.DeepEqual([]string{s.CSRF()}, csrfValues(rec.Body.String())))

	// IfRenewed renders nothing below a page, and takes no token there.
	rec, err = render(attached(st, get(t, "/notes")), func(w http.ResponseWriter, r *http.Request) elem.Node {
		return forms.WithPageToken(w, r, session.IfRenewed(text.Text("renewed")))
	})
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.Equal("", rec.Body.String()))

	// Without a token, as for a visitor without a session and no PreAuth,
	// nothing was renewed: IfRenewed renders nothing, and the forms still
	// fail the render.
	withoutPreAuth := session.Forms[account]{}
	rec, err = render(get(t, "/notes/list"), func(w http.ResponseWriter, r *http.Request) elem.Node {
		return withoutPreAuth.WithFragmentToken(w, r, session.IfRenewed(text.Text("renewed")))
	})
	t.Must(be.NoError(err))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.Equal("", rec.Body.String()))
	rec, err = render(get(t, "/notes/list"), func(w http.ResponseWriter, r *http.Request) elem.Node {
		return withoutPreAuth.WithFragmentToken(w, r, elem.Bundle{
			session.IfRenewed(text.Text("renewed")),
			session.PostFormFromContext("/notes")(text.Text("add")),
		})
	})
	t.Must(be.ErrorIs(err, session.ErrNoToken))
	t.Must(be.Equal(0, rec.Body.Len()))
}

// The stream variant sets nothing, and takes the token at every render of
// its node: an update after the cookie expired fails with ErrNoToken.
func TestWithToken(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	forms := session.Forms[account]{Store: st, PreAuth: p}

	token, cookie := issue(t, p, nil)
	r := attached(st, get(t, "/notes/events", cookie))
	update := forms.WithToken(r, notesView("milk"))
	var out strings.Builder
	t.Must(be.NoError(core.Render(r.Context(), update, &out)))
	t.Must(be.DeepEqual([]string{token, token}, csrfValues(out.String())))

	clk.Advance(session.DefaultPreAuthTTL)
	out.Reset()
	t.Must(be.ErrorIs(core.Render(r.Context(), update, &out), session.ErrNoToken))

	s, sessionCookie := login(t, st, nil, account{ID: 1})
	r = attached(st, get(t, "/notes/events", sessionCookie))
	out.Reset()
	t.Must(be.NoError(core.Render(r.Context(), forms.WithToken(r, notesView()), &out)))
	t.Must(be.DeepEqual([]string{s.CSRF()}, csrfValues(out.String())))
}

// Token nodes nest: a node below another one takes the outer node's token,
// so that a response sets one cookie at most and all of its forms carry
// one token, the one that the browser's cookie then accepts, on a first
// visit and on renewal alike. IfRenewed follows the outer node.
func TestNestedTokenNodes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clk := newClock()
	st := newStore(t, session.Options[account]{Now: clk.Now})
	p := newPreAuth(t, session.PreAuthOptions{Now: clk.Now})
	forms := session.Forms[account]{Store: st, PreAuth: p}

	type wrapper = func(w http.ResponseWriter, r *http.Request, node elem.Node) elem.Node
	kinds := []struct {
		name string
		wrap wrapper
	}{
		{"page", forms.WithPageToken},
		{"fragment", forms.WithFragmentToken},
		{"stream", func(_ http.ResponseWriter, r *http.Request, node elem.Node) elem.Node {
			return forms.WithToken(r, node)
		}},
	}
	// want is what the outer node of a kind does in a state of the cookie.
	type want struct{ cookie, renewed, fails bool }
	wants := map[string][3]want{
		// no cookie, a cookie past half of its lifetime, an expired cookie
		"page":     {{cookie: true}, {cookie: true}, {cookie: true}},
		"fragment": {{cookie: true, renewed: true}, {}, {cookie: true, renewed: true}},
		"stream":   {{fails: true}, {}, {fails: true}},
	}
	_, expired := issue(t, p, nil)
	clk.Advance(session.DefaultPreAuthTTL / 2)
	_, pastHalf := issue(t, p, nil)
	clk.Advance(session.DefaultPreAuthTTL/2 + time.Minute)
	states := []struct {
		name   string
		cookie *http.Cookie
	}{{"no cookie", nil}, {"past half", pastHalf}, {"expired", expired}}

	part := elem.Bundle{session.PostFormFromContext("/notes")(text.Text("add")), session.IfRenewed(text.Text("renewed"))}
	for _, outer := range kinds {
		for _, inner := range kinds {
			for i, state := range states {
				name := inner.name + " in " + outer.name + ", " + state.name
				want := wants[outer.name][i]
				var cookies []*http.Cookie
				if state.cookie != nil {
					cookies = append(cookies, state.cookie)
				}
				rec, err := render(attached(st, get(t, "/notes", cookies...)), func(w http.ResponseWriter, r *http.Request) elem.Node {
					return outer.wrap(w, r, elem.Bundle{part, inner.wrap(w, r, part)})
				})
				if want.fails {
					t.Mustf(be.ErrorIs(err, session.ErrNoToken), "%s", name)
					t.Mustf(be.Len(setCookies(rec), 0), "%s", name)
					t.Mustf(be.Equal(0, rec.Body.Len()), "%s", name)
					continue
				}
				t.Mustf(be.NoError(err), "%s", name)
				set := setCookies(rec)
				if want.cookie {
					t.Mustf(be.Len(set, 1), "%s", name)
					cookies = set
				} else {
					t.Mustf(be.Len(set, 0), "%s", name)
				}
				tokens := csrfValues(rec.Body.String())
				t.Mustf(be.Len(tokens, 2), "%s", name)
				t.Mustf(be.Equal(tokens[0], tokens[1]), "%s", name)
				token, ok := p.Token(get(t, "/notes", cookies...))
				t.Mustf(be.True(ok), "%s", name)
				t.Mustf(be.Equal(token, tokens[0]), "%s", name)
				renewed := 0
				if want.renewed {
					renewed = 2
				}
				t.Mustf(be.Equal(renewed, strings.Count(rec.Body.String(), "renewed")), "%s", name)
			}
		}
	}
}

// Forms rendered without a token fail the render, and web.Render then
// writes nothing: outside every token node, without a token, and when the
// token's method fails.
func TestContextTokensFailClosed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := notesView("milk")

	for _, node := range []elem.Node{
		view,
		session.CSRFInputFromContext(),
		session.IfRenewed(text.Text("renewed")),
	} {
		rec, err := render(get(t, "/notes"), func(http.ResponseWriter, *http.Request) elem.Node { return node })
		t.Must(be.ErrorContains(err, "no CSRF token in the render context"))
		t.Must(be.False(errors.Is(err, session.ErrNoToken)))
		t.Must(be.Equal(0, rec.Body.Len()))
		t.Must(be.Equal("", rec.Header().Get("Content-Type")))
	}
	_, err := session.TokenFromContext(context.Background())
	t.Must(be.Error(err))

	// No PreAuth for a visitor without a session, or a session without a
	// token: no token, rather than the empty one.
	withoutPreAuth := session.Forms[account]{}
	empty := session.NewDetached(account{ID: 1}, "")
	for _, r := range []*http.Request{get(t, "/notes"), withSession(get(t, "/notes"), empty)} {
		for _, wrap := range []func(w http.ResponseWriter, r *http.Request) elem.Node{
			func(w http.ResponseWriter, r *http.Request) elem.Node {
				return withoutPreAuth.WithPageToken(w, r, view)
			},
			func(w http.ResponseWriter, r *http.Request) elem.Node {
				return withoutPreAuth.WithFragmentToken(w, r, view)
			},
			func(_ http.ResponseWriter, r *http.Request) elem.Node { return withoutPreAuth.WithToken(r, view) },
		} {
			rec, err := render(r, wrap)
			t.Must(be.ErrorIs(err, session.ErrNoToken))
			t.Must(be.Equal(0, rec.Body.Len()))
		}
	}

	// A failing source of randomness fails the render, and sets no cookie.
	cause := errors.New("no entropy")
	src := &switchable{r: seeded(6)}
	p := newPreAuth(t, session.PreAuthOptions{Rand: src})
	src.set(failingRand{cause})
	forms := session.Forms[account]{PreAuth: p}
	rec, err := render(get(t, "/notes"), func(w http.ResponseWriter, r *http.Request) elem.Node {
		return forms.WithPageToken(w, r, view)
	})
	t.Must(be.ErrorIs(err, cause))
	t.Must(be.Len(setCookies(rec), 0))
	t.Must(be.Equal(0, rec.Body.Len()))

	// A token node of nothing fails as a nil node does.
	_, err = render(get(t, "/notes"), func(w http.ResponseWriter, r *http.Request) elem.Node {
		return forms.WithPageToken(w, r, nil)
	})
	t.Must(be.ErrorIs(err, elem.ErrNilNode))
}

// PostFormFromContext renders what PostForm renders with the token.
func TestPostFormFromContext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := session.NewDetached(account{ID: 1}, "tok-1")
	forms := session.Forms[account]{}
	r := withSession(get(t, "/"), s)
	var lazy, eager strings.Builder
	t.Must(be.NoError(core.Render(r.Context(), forms.WithToken(r, session.PostFormFromContext("/tasks", attr.ID("add"))(text.Text("fields"))), &lazy)))
	t.Must(be.NoError(core.Render(r.Context(), session.PostForm("/tasks", "tok-1", attr.ID("add"))(text.Text("fields")), &eager)))
	t.Must(be.Equal(eager.String(), lazy.String()))
	t.Must(be.Equal(`<form method="post" action="/tasks" id="add"><input type="hidden" name="csrf" value="tok-1"/>fields</form>`, lazy.String()))

	// A node of one's own reads the token with TokenFromContext.
	custom := elem.Fn(func(ctx context.Context) (elem.Node, error) {
		token, err := session.TokenFromContext(ctx)
		if err != nil {
			return nil, err
		}
		return text.Text(token), nil
	})
	var out strings.Builder
	t.Must(be.NoError(core.Render(r.Context(), forms.WithToken(r, custom), &out)))
	t.Must(be.Equal("tok-1", out.String()))
}
