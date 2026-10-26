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

	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/session"
)

// User is the value an application keeps per session.
type User struct {
	Name string
}

// A login creates a session and sets its cookie; the middleware attaches
// the session to later requests; the logout checks the form's CSRF token,
// ends the session and clears the cookie.
func ExampleStore() {
	store, err := session.NewStore(session.Options[User]{})
	if err != nil {
		panic(err)
	}

	// POST /login, after the credentials checked out.
	rec := httptest.NewRecorder()
	s, err := store.New(rec, httptest.NewRequest(http.MethodPost, "/login", nil), User{Name: "ada"})
	if err != nil {
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

	// POST /logout, from a form with the session's CSRF token.
	forms := session.Forms[User]{Store: store}
	logout := store.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[User](r.Context())
		if !ok {
			store.ClearCookie(w) // nothing to end
			return
		}
		f, err := forms.Read(w, r) // checks the token against s
		if err != nil {
			http.Error(w, "invalid form", http.StatusForbidden)
			return
		}
		f.Zero()
		store.EndSession(w, s)
	}))
	r = httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(session.CSRFField+"="+s.CSRF()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	rec = httptest.NewRecorder()
	logout.ServeHTTP(rec, r)
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
	var forms session.Forms[User] // in the application: with its Store and PreAuth
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[User](r.Context())
		if !ok {
			http.Error(w, "sign in first", http.StatusUnauthorized)
			return
		}
		f, err := forms.Read(w, r)
		if err != nil {
			http.Error(w, "invalid form", http.StatusForbidden)
			return
		}
		defer f.Zero()
		user, _ := s.Data()
		fmt.Fprintf(w, "saved %q for %s", f.String("note"), user.Name)
	})

	s := session.NewDetached(User{Name: "ada"}, "test-csrf")
	for _, token := range []string{"test-csrf", "wrong"} {
		r := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(session.CSRFField+"="+token+"&note=milk"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r = r.WithContext(session.WithSession(r.Context(), s))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		fmt.Println(rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// 200 saved "milk" for ada
	// 403 invalid form
}

// A login form is protected before any session exists: GET /login issues
// the form token and its cookie, POST /login verifies them.
func ExamplePreAuth() {
	preauth, err := session.NewPreAuth(session.PreAuthOptions{})
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

	// r still holds the old session, which has ended: a handler that
	// answers with a page rather than a redirect attaches the new one, so
	// that the page (and its forms, through Forms) is for the new session.
	r = r.WithContext(session.WithSession(r.Context(), s))
	current, _ := session.FromContext[Account](r.Context())
	fmt.Println("page for the new session:", current == s)
	// Output:
	// ada [1 2 3] true
	// true rotated true
	// old cookie works: false
	// page for the new session: true
}

// After a password change, EndFunc ends every session of the account.
func ExampleStore_EndFunc() {
	type Account struct{ ID int }
	store, err := session.NewStore(session.Options[Account]{})
	if err != nil {
		panic(err)
	}
	login := func(id int) *session.Session[Account] {
		s, err := store.New(httptest.NewRecorder(), nil, Account{ID: id})
		if err != nil {
			panic(err)
		}
		return s
	}
	first := login(1)
	login(1)
	other := login(2)

	n := store.EndFunc(func(a Account) bool { return a.ID == 1 })
	fmt.Println(n, store.Len(), first.Ended(), first.EndReason(), other.Ended())
	// Output: 2 1 true revoked false
}

// Each reads the live sessions and changes nothing: here it counts the
// sessions of each account, for a page that shows where an account is
// signed in.
func ExampleStore_Each() {
	type Account struct{ ID int }
	store, err := session.NewStore(session.Options[Account]{})
	if err != nil {
		panic(err)
	}
	for _, id := range []int{1, 1, 2} {
		if _, err := store.New(httptest.NewRecorder(), nil, Account{ID: id}); err != nil {
			panic(err)
		}
	}

	sessions := make(map[int]int)
	store.Each(func(a Account) { sessions[a.ID]++ })
	fmt.Println(sessions[1], sessions[2], store.Len())
	// Output: 2 1 3
}

// A login with Replace ends the earlier sessions of the account in the
// same step, which keeps one session per account: logging in again and
// again does not push the sessions of others out of the store, also when
// logins of one account run at the same time.
func ExampleStore_Replace() {
	type Account struct{ ID int }
	store, err := session.NewStore(session.Options[Account]{})
	if err != nil {
		panic(err)
	}
	login := func(id int) *session.Session[Account] {
		s, err := store.Replace(httptest.NewRecorder(), nil, Account{ID: id}, func(a Account) bool { return a.ID == id })
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
	// Output: 2 true revoked false
}

// A login throttle bounds the attempts per key, successful ones included:
// the login handler asks it for the account name and for the client's
// address before it checks anything, and answers a refusal with 429 and
// Retry-After.
func ExampleThrottle() {
	now := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	logins, err := session.NewThrottle(session.ThrottleOptions{
		Limit:  3,
		Window: time.Minute,
		Now:    func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}
	for attempt := 1; attempt <= 4; attempt++ {
		now = now.Add(5 * time.Second)
		retryAfter, ok := logins.Allow("name:ada")
		fmt.Println(attempt, ok, retryAfter)
	}
	_, ok := logins.Allow("name:bo") // another key has its own count
	fmt.Println("bo:", ok)

	now = now.Add(time.Minute)
	_, ok = logins.Allow("name:ada")
	fmt.Println("ada, a minute later:", ok)
	// Output:
	// 1 true 0s
	// 2 true 0s
	// 3 true 0s
	// 4 false 45s
	// bo: true
	// ada, a minute later: true
}

// The login: the form is protected by PreAuth until the credentials check
// out, then Store.New starts the session and the pre-auth cookie goes. The
// next page's forms carry the session's CSRF token.
func ExamplePreAuth_login() {
	preauth, err := session.NewPreAuth(session.PreAuthOptions{})
	if err != nil {
		panic(err)
	}
	store, err := session.NewStore(session.Options[User]{})
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
	rec = httptest.NewRecorder()
	f, err := form.Read(rec, r, form.Limits{})
	if err != nil {
		fmt.Println("invalid form:", err)
		return
	}
	defer f.Zero()
	if err := preauth.Verify(r, f.String(session.CSRFField)); err != nil {
		fmt.Println("forged:", err)
		return
	}
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

// A login form that expired is answered with the form again, with a fresh
// token, instead of an error page: the visitor keeps the typed name and
// the page to return to. The handler signs no one in, so this is safe
// behind mw.SameOrigin, which refuses cross-site posts.
func ExamplePreAuth_expired() {
	now := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now },
	})
	if err != nil {
		panic(err)
	}

	// GET /login?next=/notes renders the form.
	rec := httptest.NewRecorder()
	token, err := preauth.Issue(rec, httptest.NewRequest(http.MethodGet, "/login?next=/notes", nil))
	if err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]

	// The visitor submits it after the TTL.
	now = now.Add(session.DefaultPreAuthTTL + time.Minute)
	body := url.Values{session.CSRFField: {token}, "name": {"ada"}, "password": {"correct horse"}, "next": {"/notes"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(cookie)
	rec = httptest.NewRecorder()
	f, err := form.Read(rec, r, form.Limits{})
	if err != nil {
		fmt.Println("invalid form:", err)
		return
	}
	defer f.Zero()
	if verr := preauth.Verify(r, f.String(session.CSRFField)); verr != nil {
		// Render the login page again (with status 403): a fresh token,
		// the typed name and next, a message, and an empty password field.
		fresh, err := preauth.Issue(rec, r)
		if err != nil {
			panic(err)
		}
		fmt.Println("expired:", errors.Is(verr, session.ErrCSRF))
		fmt.Println("form again with name", f.String("name"), "and next", f.String("next"))

		// The fresh form verifies with the cookie set along with it.
		again := httptest.NewRequest(http.MethodPost, "/login", nil)
		again.AddCookie(rec.Result().Cookies()[0])
		fmt.Println("fresh form:", preauth.Verify(again, fresh))
		return
	}
	fmt.Println("signed in")
	// Output:
	// expired: true
	// form again with name ada and next /notes
	// fresh form: <nil>
}

// OnEndReason logs why sessions end; evictions reveal a store too small for
// its load, or logins that push other sessions out.
func ExampleOptions_onEndReason() {
	store, err := session.NewStore(session.Options[User]{
		MaxSessions: 2,
		OnEndReason: func(id string, reason session.EndReason, v *User) {
			fmt.Println("session of", v.Name, "ended:", reason)
		},
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

// The logout handler ends the session it holds, from the context, and
// clears the cookie. The same handler works behind the store's middleware
// and in a test with a detached session.
func ExampleStore_EndSession() {
	store, err := session.NewStore(session.Options[User]{})
	if err != nil {
		panic(err)
	}
	forms := session.Forms[User]{Store: store}
	logout := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[User](r.Context())
		if !ok {
			store.ClearCookie(w) // nothing to end
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		f, err := forms.Read(w, r) // the form's CSRF token against s
		if err != nil {
			http.Error(w, "invalid form", http.StatusForbidden)
			return
		}
		f.Zero()
		store.EndSession(w, s)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	postLogout := func(csrf string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(session.CSRFField+"="+csrf))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return r
	}

	// In the application: the middleware finds the session of the cookie.
	rec := httptest.NewRecorder()
	s, err := store.New(rec, nil, User{Name: "ada"})
	if err != nil {
		panic(err)
	}
	r := postLogout(s.CSRF())
	r.AddCookie(rec.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	store.Middleware(logout).ServeHTTP(rec, r)
	fmt.Println(rec.Code, rec.Header().Get("Set-Cookie"))
	fmt.Println(s.Ended(), s.EndReason(), store.Len())

	// In a handler test: a detached session, without a cookie.
	s = session.NewDetached(User{Name: "ada"}, "test-csrf")
	r = postLogout("test-csrf")
	r = r.WithContext(session.WithSession(r.Context(), s))
	rec = httptest.NewRecorder()
	logout.ServeHTTP(rec, r)
	fmt.Println(rec.Code, rec.Header().Get("Set-Cookie"))
	fmt.Println(s.Ended(), s.EndReason())
	// Output:
	// 303 __Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax
	// true logout 0
	// 303 __Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax
	// true logout
}

// Fragments that update a page read the page's form token with Token,
// which never sets a cookie: past half of the cookie's lifetime, Issue
// would set a new one, and the browser would then refuse the forms already
// on the page.
func ExamplePreAuth_Token() {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	preauth, err := session.NewPreAuth(session.PreAuthOptions{
		Now: func() time.Time { return now }, // a manual clock for the example
	})
	if err != nil {
		panic(err)
	}

	// GET /signup renders the whole page, with its forms: Issue.
	rec := httptest.NewRecorder()
	pageToken, err := preauth.Issue(rec, httptest.NewRequest(http.MethodGet, "/signup", nil))
	if err != nil {
		panic(err)
	}
	cookie := rec.Result().Cookies()[0]

	// Six minutes later, GET /signup/check answers an htmx request with a
	// fragment that holds a form: Token, the page's own token.
	now = now.Add(6 * time.Minute)
	r := httptest.NewRequest(http.MethodGet, "/signup/check", nil)
	r.AddCookie(cookie)
	token, ok := preauth.Token(r)
	fmt.Println(ok, token == pageToken)

	// Issuing instead would renew the cookie under the page's forms.
	rec = httptest.NewRecorder()
	renewedToken, renewed, err := preauth.IssueRenewed(rec, r)
	if err != nil {
		panic(err)
	}
	fmt.Println(renewed, renewedToken == pageToken, len(rec.Result().Cookies()))

	// Once the cookie expired, the page's forms are refused, and there is
	// no token to read: the fragment asks for a reload.
	now = now.Add(5 * time.Minute)
	_, ok = preauth.Token(r)
	fmt.Println(ok)
	// Output:
	// true true
	// true false 1
	// false
}
