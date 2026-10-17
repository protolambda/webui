package session_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
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
	store, err := session.NewStore(session.Options[User]{
		Rand: rand.NewChaCha8([32]byte{1}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Rand: rand.NewChaCha8([32]byte{2}),
	})
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
			// The Errors policy of package web answers these errors so.
			status := http.StatusBadRequest
			if errors.Is(err, session.ErrCSRF) {
				status = http.StatusForbidden
			}
			http.Error(w, http.StatusText(status), status)
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
	submit := func(token string, cookies ...*http.Cookie) {
		body := url.Values{session.CSRFField: {token}, "message": {"hello"}}.Encode()
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
	submit(preauthToken, preauthCookie)
	submit("forged", preauthCookie)
	submit(preauthToken) // without the cookie

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
	submit(s.CSRF(), sessionCookie)
	submit(preauthToken, sessionCookie, preauthCookie)
	// Output:
	// __Host-preauth
	// 200 thanks for "hello"
	// 403 Forbidden
	// 403 Forbidden
	// true 0
	// 200 thanks for "hello"
	// 403 Forbidden
}

// A fragment that updates a page carries the page's token, and sets no
// cookie. Only when the page's cookie is gone does it fall back to a new
// one, and then it must renew the page's other forms as well.
func ExampleForms_FragmentToken() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now:  func() time.Time { return now }, // a manual clock for the example
		Rand: rand.NewChaCha8([32]byte{}),     // deterministic for the example; omit in applications
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
