package session_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/web"
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

// Views render their forms with PostFormFromContext and take no token: the
// handler wraps what it renders with the kind of answer, a page here, and
// the forms take the page's token when they render. A page without forms
// sets no cookie, and a form outside every token node fails the render.
func ExampleForms_WithPageToken() {
	preauth, err := session.NewPreAuth(session.PreAuthOptions{})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{PreAuth: preauth}

	// feedback is a view deep in a page's tree.
	feedback := session.PostFormFromContext("/feedback")(
		input.Input(input.Type(input.Text), input.Name("message")),
	)
	serve := func(main elem.Node) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		if err := web.Render(rec, r, http.StatusOK, forms.WithPageToken(rec, r, main)); err != nil {
			panic(err)
		}
		fmt.Println("cookies:", len(rec.Result().Cookies()), "forms:", strings.Count(rec.Body.String(), `name="csrf"`))
	}
	serve(text.P()(text.Text("About us")))
	serve(elem.Bundle{text.P()(text.Text("Tell us what you think.")), feedback})

	err = web.Render(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), http.StatusOK, feedback)
	fmt.Println("without a token node:", err != nil)
	// Output:
	// cookies: 0 forms: 0
	// cookies: 1 forms: 1
	// without a token node: true
}

// A fragment carries the token of the page it updates and sets no cookie.
// Once the page's cookie is gone, its first form issues a new one, which
// refuses the page's other forms: IfRenewed sends fresh copies of them
// along, out of band.
func ExampleForms_WithFragmentToken() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{PreAuth: preauth}

	// The page of /tasks has a delete form in its list, and an add form.
	deleteForm := session.PostFormFromContext("/tasks/1/delete")(
		button.Button(button.Type(button.TypeSubmit))(text.Text("Delete")),
	)
	addForm := func(oob bool) elem.Node {
		return session.PostFormFromContext("/tasks", attr.ID("add-task"), attr.If(oob, hx2.SwapOOB("true")))(
			button.Button(button.Type(button.TypeSubmit))(text.Text("Add")),
		)
	}
	r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rec := httptest.NewRecorder()
	if err := web.Render(rec, r, http.StatusOK, forms.WithPageToken(rec, r, elem.Bundle{deleteForm, addForm(false)})); err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]

	// An htmx request swaps in the list.
	list := func() {
		r := httptest.NewRequest(http.MethodGet, "/tasks/list", nil)
		r.AddCookie(cookie)
		rec := httptest.NewRecorder()
		fragment := elem.Bundle{deleteForm, session.IfRenewed(addForm(true))}
		if err := web.Render(rec, r, http.StatusOK, forms.WithFragmentToken(rec, r, fragment)); err != nil {
			panic(err)
		}
		fmt.Println("cookies:", len(rec.Result().Cookies()), "forms:", strings.Count(rec.Body.String(), `name="csrf"`))
	}
	now = now.Add(6 * time.Minute) // past half of the cookie's lifetime
	list()
	now = now.Add(5 * time.Minute) // past its end
	list()
	// Output:
	// cookies: 0 forms: 1
	// cookies: 1 forms: 2
}

// The updates of an event stream render their forms within WithToken. Once
// the pre-authentication cookie that the stream opened with expires, the
// forms fail the render with ErrNoToken: the Update of the sse Loop checks
// for it and ends the stream itself, with an end event that asks for a
// reload. Returning the error instead would end the stream as a failure
// (sse.ReasonFailed), and Run would return the error to be logged.
func ExampleForms_WithToken() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{PreAuth: preauth}

	// GET /board renders the whole page, whose forms set the cookie.
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
	vote := forms.WithToken(r, session.PostFormFromContext("/board/vote")(
		button.Button(button.Type(button.TypeSubmit))(text.Text("Vote")),
	))
	changes := make(chan struct{}, 1)
	stream := httptest.NewRecorder()
	err = sse.Loop{
		Timing: sse.Timing{Interval: time.Millisecond},
		Update: func(ctx context.Context, s *sse.Stream) error {
			err := s.Send(ctx, "vote", vote)
			if errors.Is(err, session.ErrNoToken) {
				return s.End(ctx, text.P()(
					text.Text("This form expired. "), text.A(text.Href("/board"))(text.Text("Reload")),
				))
			}
			now = now.Add(6 * time.Minute) // stands in for the time until the next change
			changes <- struct{}{}
			return err
		},
		End:     func(sse.Reason) elem.Node { return text.P()(text.Text("Live updates stopped.")) },
		Changes: changes,
	}.Run(context.Background(), sse.New(stream))
	fmt.Println(err)
	fmt.Print(strings.ReplaceAll(stream.Body.String(), pageToken, "..."))
	// Output:
	// <nil>
	// event: vote
	// data: <form method="post" action="/board/vote"><input type="hidden" name="csrf" value="..."/><button type="submit">Vote</button></form>
	//
	// event: vote
	// data: <form method="post" action="/board/vote"><input type="hidden" name="csrf" value="..."/><button type="submit">Vote</button></form>
	//
	// event: end
	// data: <p>This form expired. <a href="/board">Reload</a></p>
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
