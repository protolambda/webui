package app_test

import (
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/visible"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// cookieNames returns the names of the cookies that res sets.
func cookieNames(res *cthttp.Result) []string {
	var names []string
	for _, c := range (&http.Response{Header: res.Headers()}).Cookies() {
		names = append(names, c.Name)
	}
	return names
}

// logIn submits the login form of b with the name and password, from the
// login page at target.
func logIn(t devtest.T, b *webuitest.Browser, target, name, password string) *cthttp.Result {
	t.Helper()
	p := b.Get(t, target)
	t.Must(p.Status(http.StatusOK))
	page := p.HTML()
	t.Must(contract("")(page))
	form := page.Find(ct.ID("main")).Find(ct.Tag("form"), ct.Attr("action", "/login"))
	return b.Submit(t, page, form, nil, url.Values{"name": {name}, "password": {password}})
}

func TestLogin(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})

	// The private page sends visitors to the login page, and back.
	res := b.Get(t, "/notes")
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/login?next=%2Fnotes")))
	t.Must(webuitest.NoCookies(res))
	res = b.Get(t, "/notes", webuitest.HTMX(), webuitest.HXCurrentURL(webuitest.BaseURL+"/live"))
	t.Must(res.Status(http.StatusUnauthorized))
	t.Must(webuitest.HXRedirect(res, ct.Exact("/login?next=%2Flive")))

	login := b.Get(t, "/login?next=%2Fnotes")
	t.Must(webuitest.Cookie(login, "__Host-preauth", webuitest.CookieWant{
		HostPrefix: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}))

	t.Run("wrong password", func(t devtest.T) {
		res := logIn(t, b, "/login?next=%2Fnotes", "Ada", "wrong")
		page := res.HTML()
		t.Must(res.Status(http.StatusUnprocessableEntity))
		t.Must(contract("")(page))
		t.Must(page.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts("The name or password is wrong."))
		// The name is kept, the password is not, and the page to return to
		// stays in the form.
		t.Must(page.Find(ct.ID("login-name")).Matches(ct.Attr("value", "Ada")))
		t.Must(page.Find(ct.ID("login-password"), ct.HasAttr("value")).None())
		t.Must(page.Find(ct.Attr("name", web.NextParam)).Matches(ct.Attr("value", "/notes")))
		t.Must(be.False(slices.Contains(cookieNames(res), "__Host-session")))
	})

	t.Run("login CSRF", func(t devtest.T) {
		// A form token without its cookie, as another site could obtain
		// one for itself, does not log anyone in: the answer is the login
		// form again, with a token of its own.
		token := webuitest.CSRFToken(t, login.HTML())
		other := b.Fork()
		res := other.Post(t, "/login", webuitest.Form(url.Values{
			session.CSRFField: {token}, "name": {"Mallory"}, "password": {app.DemoPassword},
		}))
		t.Must(res.Status(http.StatusForbidden))
		t.Must(be.DeepEqual([]string{"__Host-preauth"}, cookieNames(res)))
		page := res.HTML()
		t.Must(contract("")(page))
		t.Must(be.NotEqual(token, webuitest.CSRFToken(t, page)))
	})

	res = logIn(t, b, "/login?next=%2Fnotes", "Ada", app.DemoPassword)
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes?notice=signed-in")))
	t.Must(webuitest.Cookie(res, "__Host-session", webuitest.CookieWant{
		HostPrefix: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Session: true,
	}))
	t.Must(webuitest.Cookie(res, "__Host-preauth", webuitest.CookieWant{HostPrefix: true, Cleared: true}))

	notes := b.Get(t, "/notes?notice=signed-in")
	t.Must(notes.Status(http.StatusOK))
	page := notes.HTML()
	csrf := webuitest.CSRFToken(t, page)
	t.Must(contract(csrf)(page))
	t.Must(page.Find(ct.ID("alerts")).Find(ct.Role("status")).Texts("Signed in."))
	t.Must(page.Find(ct.ID("navbar")).Find(ct.Tag("strong")).Texts("Ada"))
	// A signed-in visitor at the login page goes on to the notes.
	t.Must(webuitest.Location(b.Get(t, "/login"), http.StatusSeeOther, ct.Exact("/notes")))

	t.Run("notes", func(t devtest.T) {
		add := func(note string) *cthttp.Result {
			p := b.Get(t, "/notes").HTML()
			return b.Submit(t, p, p.Find(ct.Tag("form"), ct.Attr("action", "/notes")), nil, url.Values{"note": {note}})
		}
		t.Must(webuitest.Location(add("Buy <milk>"), http.StatusSeeOther, ct.Exact("/notes?notice=note-added")))
		p := b.Get(t, "/notes").HTML()
		t.Must(p.Find(ct.ID("main")).Find(ct.Class("list-group-item")).Texts("Buy <milk>"))
		t.Must(p.Find(ct.Tag("milk")).None())
		// A note keeps its lines, each shown through visible.Text: a line
		// break is no escape, an invisible character is.
		t.Must(webuitest.Location(add("Call Bo\r\nat\u200Bnoon"), http.StatusSeeOther, ct.Exact("/notes?notice=note-added")))
		item := b.Get(t, "/notes").HTML().Find(ct.ID("main")).Find(ct.Class("list-group-item")).Last()
		t.Must(item.Find(ct.Tag("div")).Count(2))
		t.Must(item.Find(ct.Tag("div")).First().Texts("Call Bo"))
		t.Must(item.Find(ct.Class("badge")).Texts(visible.MarkerText))

		res := add("   ")
		t.Must(res.Status(http.StatusUnprocessableEntity))
		t.Must(res.HTML().Find(ct.ID("note-error")).Texts("Write a note of 1 to 280 characters."))

		// Another browser sees none of them.
		other := b.Fork()
		t.Must(webuitest.Location(other.Get(t, "/notes"), http.StatusSeeOther, ct.Prefix("/login")))
	})

	t.Run("logout needs the token", func(t devtest.T) {
		res := b.Post(t, "/logout", webuitest.Form(url.Values{session.CSRFField: {"forged"}}))
		t.Must(res.Status(http.StatusForbidden))
		t.Must(webuitest.NoCookies(res))
		t.Must(b.Get(t, "/notes").Status(http.StatusOK))
	})

	p := b.Get(t, "/notes").HTML()
	res = b.Submit(t, p, p.Find(ct.ID("navbar")).Find(ct.Tag("form")), nil, nil)
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/?notice=signed-out")))
	t.Must(webuitest.Cookie(res, "__Host-session", webuitest.CookieWant{HostPrefix: true, Cleared: true}))
	t.Must(webuitest.Location(b.Get(t, "/notes"), http.StatusSeeOther, ct.Exact("/login?next=%2Fnotes")))
	home := b.Get(t, "/?notice=signed-out").HTML()
	t.Must(home.Find(ct.ID("alerts")).Find(ct.Role("status")).Texts("Signed out."))

	// Nothing secret reached the log: the password, the CSRF token, the
	// session cookie.
	t.Must(g.logs.NotLogged(app.DemoPassword, csrf, "Buy <milk>"))
	for _, c := range b.Jar().Cookies(&url.URL{Scheme: "https", Host: "app.example.test", Path: "/"}) {
		t.Must(g.logs.NotLogged(c.Value))
	}
}

// TestLoginThrottled makes more attempts than the throttle allows within a
// minute from one client: the answer is 429 with Retry-After, also with the
// right password and for another name, until the window has passed.
func TestLoginThrottled(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clock := newFakeClock()
	g := newGallery(t, app.Options{Now: clock.Now})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})
	for i := range session.DefaultThrottleLimit {
		res := logIn(t, b, "/login", "Ada", "wrong")
		t.Mustf(res.Status(http.StatusUnprocessableEntity), "attempt %d", i+1)
	}
	res := logIn(t, b, "/login", "Ada", app.DemoPassword)
	t.Must(res.Status(http.StatusTooManyRequests))
	t.Must(res.Header("Retry-After", ct.Exact("60")))
	t.Must(be.False(slices.Contains(cookieNames(res), "__Host-session")))
	page := res.HTML()
	t.Must(contract("")(page))
	t.Must(page.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts(web.DefaultMessage(http.StatusTooManyRequests)))
	// The client's address is a key of its own: another name is refused too.
	t.Must(logIn(t, b, "/login", "Bo", app.DemoPassword).Status(http.StatusTooManyRequests))
	clock.Advance(30 * time.Second)
	res = logIn(t, b, "/login", "Ada", app.DemoPassword)
	t.Must(res.Status(http.StatusTooManyRequests))
	t.Must(res.Header("Retry-After", ct.Exact("30")))

	clock.Advance(30 * time.Second)
	res = logIn(t, b, "/login", "Ada", app.DemoPassword)
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes?notice=signed-in")))
	t.Must(g.logs.NotLogged(app.DemoPassword))
}

func TestLoginThrottleBehindProxy(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct {
		name  string
		trust func(netip.Addr) bool
		want  int
	}{
		{"trusted", netip.MustParsePrefix("127.0.0.1/32").Contains, http.StatusSeeOther},
		{"untrusted header spoof", nil, http.StatusTooManyRequests},
	} {
		t.Run(c.name, func(t devtest.T) {
			clock := newFakeClock()
			g := newGallery(t, app.Options{Now: clock.Now, TrustedProxy: c.trust})
			first := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})
			first.Options = []webuitest.ReqOption{webuitest.Header("X-Real-IP", "198.51.100.10")}
			for range session.DefaultThrottleLimit {
				t.Must(logIn(t, first, "/login", "Ada", "wrong").Status(http.StatusUnprocessableEntity))
			}
			t.Must(logIn(t, first, "/login", "Bo", app.DemoPassword).Status(http.StatusTooManyRequests))

			// Both browsers connect from loopback, as two users behind the
			// same proxy do. Only a trusted peer may distinguish its clients.
			other := first.Fork()
			other.Options = []webuitest.ReqOption{webuitest.Header("X-Real-IP", "203.0.113.42")}
			t.Must(logIn(t, other, "/login", "Bo", app.DemoPassword).Status(c.want))
			// The account allowance still applies across client addresses.
			third := first.Fork()
			third.Options = []webuitest.ReqOption{webuitest.Header("X-Real-IP", "203.0.113.43")}
			t.Must(logIn(t, third, "/login", "Ada", app.DemoPassword).Status(http.StatusTooManyRequests))
		})
	}
}

func TestLoginRejectsInvalidProxyIdentity(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, header := range []string{"", "203.0.113.2, 203.0.113.3", "private request text"} {
		g := newGallery(t, app.Options{TrustedProxy: netip.MustParsePrefix("127.0.0.1/32").Contains})
		b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})
		if header != "" {
			b.Options = []webuitest.ReqOption{webuitest.Header("X-Real-IP", header)}
		}
		res := logIn(t, b, "/login", "Ada", app.DemoPassword)
		t.Must(res.Status(http.StatusBadRequest))
		t.Must(be.False(slices.Contains(cookieNames(res), "__Host-session")))
		t.Must(g.logs.NotLogged("private request text"))
	}
}

// TestLoginExpired submits a login form after its pre-authentication
// token expired: the answer is the login form again, with a fresh token,
// the name and the page to return to, but not the password, and the fresh
// form logs in.
func TestLoginExpired(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clock := newFakeClock()
	g := newGallery(t, app.Options{Now: clock.Now})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})
	p := b.Get(t, "/login?next=%2Fnotes")
	t.Must(p.Status(http.StatusOK))
	page := p.HTML()
	issued := webuitest.CSRFToken(t, page)

	clock.Advance(session.DefaultPreAuthTTL + time.Second)
	form := page.Find(ct.ID("main")).Find(ct.Tag("form"), ct.Attr("action", "/login"))
	res := b.Submit(t, page, form, nil, url.Values{"name": {"Ada"}, "password": {app.DemoPassword}})
	t.Must(res.Status(http.StatusForbidden))
	t.Must(be.False(slices.Contains(cookieNames(res), "__Host-session")))
	again := res.HTML()
	t.Must(contract("")(again))
	t.Must(again.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts("The login form expired. Enter your password again."))
	t.Must(again.Find(ct.ID("login-name")).Matches(ct.Attr("value", "Ada")))
	t.Must(again.Find(ct.ID("login-password"), ct.HasAttr("value")).None())
	t.Must(again.Find(ct.Attr("name", web.NextParam)).Matches(ct.Attr("value", "/notes")))
	t.Must(be.NotEqual(issued, webuitest.CSRFToken(t, again)))

	form = again.Find(ct.ID("main")).Find(ct.Tag("form"), ct.Attr("action", "/login"))
	res = b.Submit(t, again, form, nil, url.Values{"password": {app.DemoPassword}})
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes?notice=signed-in")))
	t.Must(g.logs.NotLogged(app.DemoPassword))
}

// TestInsecureCookies runs the login over plain http, as the gallery does
// on a loopback address: the cookies lose Secure and the __Host- prefix.
func TestInsecureCookies(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{Sites: []mw.Site{mw.MustParseSite(webuitest.InsecureBaseURL)}, InsecureCookies: true})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{Insecure: true})
	res := logIn(t, b, "/login", "Bo", app.DemoPassword)
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes?notice=signed-in")))
	t.Must(webuitest.Cookie(res, "session", webuitest.CookieWant{HttpOnly: true, SameSite: http.SameSiteLaxMode, Session: true, Path: "/"}))
	notes := b.Get(t, "/notes")
	t.Must(notes.Status(http.StatusOK))
	// A plain-http loopback site gets no HSTS.
	t.Must(webuitest.SecurityHeaders(notes, webuitest.StrictPolicy()))
}
