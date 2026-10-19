package session_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/session"
)

// PostForm renders the CSRF field first, then the form's own fields.
func ExamplePostForm() {
	s := session.NewDetached(User{Name: "ada"}, "k3Xo-token")
	core.Dump(session.PostForm("/tasks", s.CSRF(), attr.Class("d-flex gap-2"))(
		input.Input(input.Type(input.Text), input.Name("title"), input.Placeholder("Title")),
		button.Button(button.Type(button.TypeSubmit))(text.Text("Add")),
	))
	// Output:
	// <form method="post" action="/tasks" class="d-flex gap-2"><input type="hidden" name="csrf" value="k3Xo-token"/><input type="text" name="title" placeholder="Title"/><button type="submit">Add</button></form>
}

// One Forms serves the handlers of an application with signed-in and
// anonymous visitors: pages ask it for the token of their forms, and POST
// handlers read their form through it, which checks the token against the
// session of the request or, without one, its pre-authentication cookie.
func ExampleForms() {
	store, err := session.NewStore(session.Options[User]{})
	if err != nil {
		panic(err)
	}
	preauth, err := session.NewPreAuth(session.PreAuthOptions{})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{Store: store, PreAuth: preauth, Limits: form.Limits{Body: 64 << 10}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		token, err := forms.PageToken(w, r) // before anything is written
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		var page strings.Builder
		if err := core.Render(r.Context(), session.PostForm("/feedback", token)(
			input.Input(input.Type(input.Text), input.Name("message")),
		), &page); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, page.String())
	})
	mux.HandleFunc("POST /feedback", func(w http.ResponseWriter, r *http.Request) {
		f, err := forms.Read(w, r)
		if err != nil {
			// The statuses the Errors policy of package web gives these
			// errors; with that policy, return err unchanged instead.
			http.Error(w, http.StatusText(formStatus(err)), formStatus(err))
			return
		}
		defer f.Zero()
		fmt.Fprintf(w, "thanks for %q", f.String("message"))
	})
	app := store.Middleware(mux) // innermost, right around the router

	// A browser: it sends cookies and submits the form of a page.
	serve := func(r *http.Request, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, r)
		return rec
	}
	tokenOf := func(page string) string {
		_, rest, _ := strings.Cut(page, `name="csrf" value="`)
		token, _, _ := strings.Cut(rest, `"`)
		return token
	}
	submit := func(message, token string, cookies ...*http.Cookie) {
		body := url.Values{session.CSRFField: {token}, "message": {message}}.Encode()
		r := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(body))
		r.Header.Set("Content-Type", form.URLEncoded)
		rec := serve(r, cookies...)
		fmt.Println(rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	// An anonymous visitor gets a pre-authentication token and its cookie.
	rec := serve(httptest.NewRequest(http.MethodGet, "/", nil))
	preauthCookie := rec.Result().Cookies()[0]
	preauthToken := tokenOf(rec.Body.String())
	fmt.Println(preauthCookie.Name)
	submit("hello", preauthToken, preauthCookie)
	submit("hello", "forged", preauthCookie)
	submit("hello", preauthToken) // without the cookie
	submit(strings.Repeat("x", 70<<10), preauthToken, preauthCookie)

	// After a login, the page's forms carry the session's token, and the
	// forms rendered before the login are refused.
	rec = httptest.NewRecorder()
	s, err := store.New(rec, nil, User{Name: "ada"})
	if err != nil {
		panic(err)
	}
	sessionCookie := rec.Result().Cookies()[0]
	rec = serve(httptest.NewRequest(http.MethodGet, "/", nil), sessionCookie)
	fmt.Println(tokenOf(rec.Body.String()) == s.CSRF(), len(rec.Result().Cookies()))
	submit("hello", s.CSRF(), sessionCookie)
	submit("hello", preauthToken, sessionCookie, preauthCookie)
	// Output:
	// __Host-preauth
	// 200 thanks for "hello"
	// 403 Forbidden
	// 403 Forbidden
	// 413 Request Entity Too Large
	// true 0
	// 200 thanks for "hello"
	// 403 Forbidden
}

// formStatus is the status that the Errors policy of package web answers
// the errors of Forms.Read and form.Read with.
func formStatus(err error) int {
	switch {
	case errors.Is(err, session.ErrCSRF):
		return http.StatusForbidden
	case errors.Is(err, form.ErrTooLarge): // also matches *http.MaxBytesError
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, form.ErrUnsupportedType):
		return http.StatusUnsupportedMediaType
	default: // form.ErrMalformed, ErrTooManyFields, ErrFile
		return http.StatusBadRequest
	}
}

// A fragment that updates a page carries the page's token, and sets no
// cookie. Only when the page's cookie is gone does it fall back to a new
// one, and then it must renew the page's other forms as well.
func ExampleForms_FragmentToken() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{PreAuth: preauth}

	// GET /tasks renders the whole page, with an add form: PageToken.
	rec := httptest.NewRecorder()
	pageToken, err := forms.PageToken(rec, httptest.NewRequest(http.MethodGet, "/tasks", nil))
	if err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]

	// Six minutes later, an htmx request swaps in the list, with a delete
	// form per task: FragmentToken, the page's own token.
	now = now.Add(6 * time.Minute)
	r := httptest.NewRequest(http.MethodGet, "/tasks/list", nil)
	r.AddCookie(cookie)
	rec = httptest.NewRecorder()
	token, renewed, err := forms.FragmentToken(rec, r)
	if err != nil {
		panic(err)
	}
	fmt.Println(token == pageToken, renewed, len(rec.Result().Cookies()))

	// Once the cookie expired, the fragment gets a new cookie and token:
	// it also sends a fresh add form, out of band, as the new cookie
	// refuses the page's old one.
	now = now.Add(5 * time.Minute)
	rec = httptest.NewRecorder()
	token, renewed, err = forms.FragmentToken(rec, r)
	if err != nil {
		panic(err)
	}
	fmt.Println(token == pageToken, renewed, len(rec.Result().Cookies()))
	// Output:
	// true false 0
	// false true 1
}

// A form that should come back with what was typed when its token fails,
// such as a feedback form of an anonymous visitor whose cookie expired, is
// read with form.Read and checked with Check: on ErrCSRF the handler acts
// on nothing and answers with the form again, a fresh token and the typed
// message. That is safe behind mw.SameOrigin, which refuses cross-site
// posts.
func ExampleForms_Check() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{PreAuth: preauth}
	limits := form.Limits{Body: 64 << 10}

	feedback := func(w http.ResponseWriter, r *http.Request, status int, message string) {
		token, err := forms.PageToken(w, r) // before anything is written
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		var page strings.Builder
		if err := core.Render(r.Context(), session.PostForm("/feedback", token)(
			input.Input(input.Type(input.Text), input.Name("message"), input.Value(message)),
		), &page); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(status)
		fmt.Fprint(w, page.String())
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /feedback", func(w http.ResponseWriter, r *http.Request) {
		feedback(w, r, http.StatusOK, "")
	})
	mux.HandleFunc("POST /feedback", func(w http.ResponseWriter, r *http.Request) {
		f, err := form.Read(w, r, limits)
		if err != nil {
			http.Error(w, http.StatusText(formStatus(err)), formStatus(err))
			return
		}
		defer f.Zero()
		if err := forms.Check(r, f.String(session.CSRFField)); err != nil {
			// The form again, with what was typed (never a password).
			feedback(w, r, http.StatusForbidden, f.String("message"))
			return
		}
		fmt.Fprintf(w, "thanks for %q", f.String("message"))
	})

	tokenOf := func(page string) string {
		_, rest, _ := strings.Cut(page, `name="csrf" value="`)
		token, _, _ := strings.Cut(rest, `"`)
		return token
	}
	submit := func(token string, cookie *http.Cookie) *httptest.ResponseRecorder {
		body := url.Values{session.CSRFField: {token}, "message": {"the export is slow"}}.Encode()
		r := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(body))
		r.Header.Set("Content-Type", form.URLEncoded)
		r.AddCookie(cookie)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		return rec
	}

	// The visitor opens the form, and submits it after the TTL.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/feedback", nil))
	cookie := rec.Result().Cookies()[0]
	now = now.Add(session.DefaultPreAuthTTL + time.Minute)
	rec = submit(tokenOf(rec.Body.String()), cookie)
	fmt.Println(rec.Code, strings.Contains(rec.Body.String(), `value="the export is slow"`))

	// The form came back with a fresh token and its cookie, and goes
	// through.
	rec = submit(tokenOf(rec.Body.String()), rec.Result().Cookies()[0])
	fmt.Println(rec.Code, rec.Body.String())
	// Output:
	// 403 true
	// 200 thanks for "the export is slow"
}

// The updates of an event stream (a live region) render their forms with
// Token, which never sets a cookie: the stream's response has started, so
// a cookie set on it would be lost. The stream's request keeps the cookies
// it opened with, so once the page's pre-authentication cookie expires,
// the stream ends with a prompt to reload (in package sse, the Update of a
// Loop calls the stream's End).
func ExampleForms_Token() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{PreAuth: preauth}

	// GET /board renders the whole page: PageToken sets the cookie.
	rec := httptest.NewRecorder()
	pageToken, err := forms.PageToken(rec, httptest.NewRequest(http.MethodGet, "/board", nil))
	if err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]

	// The page's live region opens GET /board/events, whose updates each
	// render a vote form.
	r := httptest.NewRequest(http.MethodGet, "/board/events", nil)
	r.AddCookie(cookie)
	update := func() {
		token, ok := forms.Token(r)
		if !ok {
			fmt.Println("end event: the form expired, reload the page")
			return
		}
		fmt.Println("update with the page's token:", token == pageToken)
	}
	update()
	now = now.Add(6 * time.Minute)
	update()
	now = now.Add(5 * time.Minute)
	update()
	// Output:
	// update with the page's token: true
	// update with the page's token: true
	// end event: the form expired, reload the page
}
