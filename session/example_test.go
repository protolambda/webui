package session_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/protolambda/chord/core"

	"github.com/protolambda/webui/session"
)

// User is the value an application keeps per session.
type User struct {
	Name string
}

// A login creates a session and sets its cookie; the middleware attaches
// the session to later requests; the logout ends it and clears the cookie.
func ExampleStore() {
	store, err := session.NewStore(session.Options[User]{
		Rand: rand.NewChaCha8([32]byte{}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}

	// POST /login, after the credentials checked out.
	rec := httptest.NewRecorder()
	if _, err := store.New(rec, httptest.NewRequest(http.MethodPost, "/login", nil), User{Name: "ada"}); err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]
	fmt.Println(cookie.Name, cookie.Path, cookie.HttpOnly, cookie.Secure, cookie.SameSite == http.SameSiteLaxMode, cookie.MaxAge)

	app := store.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[User](r.Context())
		if !ok {
			fmt.Println("anonymous")
			return
		}
		user, _ := s.Data()
		fmt.Println("signed in as", user.Name)
	}))

	// GET /tasks with and without the cookie.
	r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	r.AddCookie(cookie)
	app.ServeHTTP(httptest.NewRecorder(), r)
	app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tasks", nil))

	// POST /logout.
	r = httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.AddCookie(cookie)
	rec = httptest.NewRecorder()
	store.End(rec, r)
	fmt.Println(rec.Header().Get("Set-Cookie"))
	app.ServeHTTP(httptest.NewRecorder(), r)
	// Output:
	// __Host-session / true true true 0
	// signed in as ada
	// anonymous
	// __Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax
	// anonymous
}

// Every state-changing form carries the session's CSRF token, which the
// handler checks before acting.
func ExampleSession_CheckCSRF() {
	s := session.NewDetached(User{Name: "ada"}, "k3Xo-token")
	core.Dump(session.CSRFInput(s.CSRF()))

	submitted := url.Values{session.CSRFField: {"k3Xo-token"}}
	fmt.Println(s.CheckCSRF(submitted.Get(session.CSRFField)))
	fmt.Println(s.CheckCSRF("forged"))
	fmt.Println(s.CheckCSRF(""))
	// Output:
	// <input type="hidden" name="csrf" value="k3Xo-token"/>
	// <nil>
	// session: invalid CSRF token
	// session: invalid CSRF token
}

func ExampleCSRFInput() {
	core.Dump(session.CSRFInput("dG9rZW4"))
	// Output: <input type="hidden" name="csrf" value="dG9rZW4"/>
}

func ExampleCheckCSRF() {
	fmt.Println(session.CheckCSRF("dG9rZW4", "dG9rZW4"))
	fmt.Println(errors.Is(session.CheckCSRF("dG9rZW4", "dG9rZW5"), session.ErrCSRF))
	// An empty expected token never matches, not even an empty field.
	fmt.Println(errors.Is(session.CheckCSRF("", ""), session.ErrCSRF))
	// Output:
	// <nil>
	// true
	// true
}

// Handler tests skip the login: a detached session attached to the
// request context stands in for the middleware's lookup.
func ExampleNewDetached() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[User](r.Context())
		if !ok {
			http.Error(w, "sign in first", http.StatusUnauthorized)
			return
		}
		if err := s.CheckCSRF(r.PostFormValue(session.CSRFField)); err != nil {
			http.Error(w, "invalid form", http.StatusForbidden)
			return
		}
		user, _ := s.Data()
		fmt.Fprintf(w, "saved for %s", user.Name)
	})

	s := session.NewDetached(User{Name: "ada"}, "test-csrf")
	for _, token := range []string{"test-csrf", "wrong"} {
		r := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(session.CSRFField+"="+token))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r = r.WithContext(session.WithSession(r.Context(), s))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		fmt.Println(rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// 200 saved for ada
	// 403 invalid form
}

// A login form is protected before any session exists: GET /login issues
// the form token and its cookie, POST /login verifies them.
func ExamplePreAuth() {
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Rand: rand.NewChaCha8([32]byte{}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}

	// GET /login
	rec := httptest.NewRecorder()
	token, err := preauth.Issue(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]
	fmt.Println(cookie.Name, cookie.MaxAge, len(token))

	// POST /login, with the form's token, then with a forged one.
	for _, submitted := range []string{token, "forged"} {
		r := httptest.NewRequest(http.MethodPost, "/login", nil)
		r.AddCookie(cookie)
		fmt.Println(preauth.Verify(r, submitted))
	}
	// POST /login from a browser without the cookie.
	fmt.Println(preauth.Verify(httptest.NewRequest(http.MethodPost, "/login", nil), token))
	// Output:
	// __Host-preauth 600 43
	// <nil>
	// session: invalid CSRF token: pre-auth token does not match its cookie
	// session: invalid CSRF token: no pre-auth cookie
}

// OnEnd clears secrets of a session when it ends, however it ends.
func ExampleOptions_onEnd() {
	type Secrets struct{ Key []byte }
	store, err := session.NewStore(session.Options[Secrets]{
		OnEnd: func(id string, v *Secrets) { clear(v.Key) },
		Rand:  rand.NewChaCha8([32]byte{}),
	})
	if err != nil {
		panic(err)
	}
	key := []byte{1, 2, 3, 4}
	s, err := store.New(httptest.NewRecorder(), nil, Secrets{Key: key})
	if err != nil {
		panic(err)
	}
	s.End()
	<-s.Done()
	fmt.Println(key)
	// Output: [0 0 0 0]
}

// After a re-authentication, Rotate gives the session a new token and CSRF
// token and keeps its value: the OnEnd hook does not clear it.
func ExampleStore_Rotate() {
	type Account struct {
		Name     string
		Key      []byte
		Elevated bool
	}
	store, err := session.NewStore(session.Options[Account]{
		OnEnd: func(id string, v *Account) { clear(v.Key) },
		Rand:  rand.NewChaCha8([32]byte{}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}
	rec := httptest.NewRecorder()
	old, err := store.New(rec, nil, Account{Name: "ada", Key: []byte{1, 2, 3}})
	if err != nil {
		panic(err)
	}
	oldCookie := rec.Result().Cookies()[0]

	// POST /confirm-password, after the password checked out.
	r := httptest.NewRequest(http.MethodPost, "/confirm-password", nil)
	r.AddCookie(oldCookie)
	rec = httptest.NewRecorder()
	s, err := store.Rotate(rec, r)
	if err != nil {
		panic(err)
	}
	if err := s.Update(func(v *Account) { v.Elevated = true }); err != nil {
		panic(err)
	}
	newCookie := rec.Result().Cookies()[0]

	account, _ := s.Data()
	fmt.Println(account.Name, account.Key, account.Elevated)
	fmt.Println(old.Ended(), old.EndReason(), newCookie.Value != oldCookie.Value)
	_, ok := store.Lookup(r)
	fmt.Println("old cookie works:", ok)
	// Output:
	// ada [1 2 3] true
	// true rotated true
	// old cookie works: false
}

// Ending the earlier sessions of an account before its login keeps one
// session per account, so that logging in again and again does not push
// the sessions of others out of the store.
func ExampleStore_EndFunc() {
	type Account struct{ ID int }
	store, err := session.NewStore(session.Options[Account]{
		Rand: rand.NewChaCha8([32]byte{}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}
	login := func(id int) *session.Session[Account] {
		store.EndFunc(func(a Account) bool { return a.ID == id })
		s, err := store.New(httptest.NewRecorder(), nil, Account{ID: id})
		if err != nil {
			panic(err)
		}
		return s
	}

	other := login(2)
	first := login(1)
	for range 100 {
		login(1)
	}
	fmt.Println(store.Len(), first.Ended(), first.EndReason(), other.Ended())

	// After a password change: end every session of the account.
	fmt.Println(store.EndFunc(func(a Account) bool { return a.ID == 1 }), store.Len())
	// Output:
	// 2 true revoked false
	// 1 1
}

// The login: the form is protected by PreAuth until the credentials check
// out, then Store.New starts the session and the pre-auth cookie goes. The
// next page's forms carry the session's CSRF token.
func ExamplePreAuth_login() {
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Rand: rand.NewChaCha8([32]byte{1}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}
	store, err := session.NewStore(session.Options[User]{
		Rand: rand.NewChaCha8([32]byte{2}),
	})
	if err != nil {
		panic(err)
	}

	// GET /login renders the form with the pre-auth token.
	rec := httptest.NewRecorder()
	token, err := preauth.Issue(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if err != nil {
		panic(err)
	}
	preauthCookie := rec.Result().Cookies()[0]

	// POST /login with the form's token and valid credentials.
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(session.CSRFField+"="+token))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(preauthCookie)
	if err := preauth.Verify(r, r.PostFormValue(session.CSRFField)); err != nil {
		fmt.Println("forged:", err)
		return
	}
	rec = httptest.NewRecorder()
	s, err := store.New(rec, r, User{Name: "ada"})
	if err != nil {
		panic(err)
	}
	preauth.ClearCookie(rec)
	for _, c := range rec.Result().Cookies() {
		fmt.Println(c.Name, c.MaxAge)
	}

	// GET /tasks: the forms carry the session's token, which POST checks.
	fmt.Println(len(s.CSRF()), s.CheckCSRF(s.CSRF()), s.CheckCSRF(token) != nil)
	// Output:
	// __Host-session 0
	// __Host-preauth -1
	// 43 <nil> true
}

// OnEndReason logs why sessions end; evictions reveal a store too small for
// its load, or logins that push other sessions out.
func ExampleOptions_onEndReason() {
	store, err := session.NewStore(session.Options[User]{
		MaxSessions: 2,
		OnEndReason: func(id string, reason session.EndReason, v *User) {
			fmt.Println("session of", v.Name, "ended:", reason)
		},
		Rand: rand.NewChaCha8([32]byte{}), // deterministic for the example; omit in applications
	})
	if err != nil {
		panic(err)
	}
	login := func(name string) *session.Session[User] {
		s, err := store.New(httptest.NewRecorder(), nil, User{Name: name})
		if err != nil {
			panic(err)
		}
		return s
	}
	ada := login("ada")
	login("grace")
	login("edsger") // the store is full: ada's session goes
	<-ada.Done()
	fmt.Println(ada.EndReason())
	store.EndAll()
	// Output:
	// session of ada ended: evicted
	// evicted
	// session of edsger ended: revoked
	// session of grace ended: revoked
}
