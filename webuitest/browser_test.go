package webuitest_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/webuitest"
)

// accountApp is a full stack: security headers, host check, same-origin
// policy and cookie sessions in front of a few routes.
type accountApp struct {
	store  *session.Store[string]
	proto  atomic.Int32
	logins atomic.Int32
}

func newAccountApp(t devtest.T, site string, insecure bool) (*accountApp, http.Handler) {
	store, err := session.NewStore(session.Options[string]{Insecure: insecure})
	t.Must(be.NoError(err))
	preauth, err := session.NewPreAuth(session.PreAuthOptions{Insecure: insecure})
	t.Must(be.NoError(err))
	a := &accountApp{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		a.proto.Store(int32(r.ProtoMajor))
		s, ok := store.Lookup(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		name, _ := s.Data()
		page := validDoc().with(func(d *docSpec) {
			d.body = []elem.Node{section.Main(attr.ID("main"))(
				section.H1()(text.Text("Hello "+name)),
				form.Form(form.Method(form.MethodPost), form.Action("/logout"))(
					session.CSRFInput(s.CSRF()),
					button.Button(button.Type(button.TypeSubmit))(text.Text("Log out")),
				),
			)}
		})
		render(w, r, http.StatusOK, page.node())
	})
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		token, err := preauth.Issue(w, r)
		if err != nil {
			http.Error(w, "no token", http.StatusInternalServerError)
			return
		}
		page := validDoc().with(func(d *docSpec) {
			d.body = []elem.Node{section.Main(attr.ID("main"))(
				form.Form(form.Method(form.MethodPost), form.Action("/login"))(
					session.CSRFInput(token),
					label.Label(label.For("name"))(text.Text("Name")),
					input.Input(attr.ID("name"), input.Name("name")),
					button.Button(button.Type(button.TypeSubmit))(text.Text("Log in")),
				),
			)}
		})
		render(w, r, http.StatusOK, page.node())
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if preauth.Verify(r, r.PostFormValue(session.CSRFField)) != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if _, err := store.New(w, r, r.FormValue("name")); err != nil {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		a.logins.Add(1)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		store.End(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /live", liveTasks)
	h := mw.Chain(mux,
		mw.SecurityHeaders(mw.Headers{}),
		mw.HostCheck(mw.MustParseSite(site)),
		mw.SameOrigin(mw.OriginPolicy{}),
	)
	return a, h
}

func TestBrowser(gt *testing.T) {
	t := devtest.ParallelT(gt)
	app, h := newAccountApp(t, webuitest.BaseURL, false)
	b := webuitest.NewBrowser(t, h, webuitest.BrowserOptions{})
	t.Must(be.Equal(webuitest.BaseURL, b.URL()))

	res := b.Get(t, "/")
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/login")))
	t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
	t.Must(webuitest.NoCookies(res))

	login := b.Get(t, "/login")
	t.Must(login.Status(http.StatusOK))
	page := login.HTML()
	t.Must(webuitest.PageContract(page, webuitest.DefaultShell()))
	res = b.Submit(t, page, page.Find(ct.Tag("form")), nil, url.Values{"name": {"Ada"}})
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/")))
	t.Must(webuitest.Cookie(res, "__Host-session", webuitest.CookieWant{
		HostPrefix: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Session: true,
	}))

	// The jar sends the cookie; the redirect was not followed.
	home := b.Get(t, "/")
	t.Must(home.Status(http.StatusOK))
	t.Must(home.HTML().Find(ct.Role("heading", ct.Level(1))).Texts("Hello Ada"))
	var names []string
	for _, c := range b.Jar().Cookies(&url.URL{Scheme: "https", Host: "app.example.test", Path: "/"}) {
		names = append(names, c.Name)
	}
	t.Must(be.Contains(names, "__Host-session"))

	// Another browser has no session.
	other := b.Fork()
	t.Must(webuitest.Location(other.Get(t, "/"), http.StatusSeeOther, ct.Exact("/login")))

	// Cross-site requests cannot log the user out.
	t.Must(webuitest.RejectsCrossSite(t, b, http.MethodPost, "/logout", webuitest.Body{}, func() error {
		if code := b.Get(t, "/").StatusCode(); code != http.StatusOK {
			return errStatus(code)
		}
		return nil
	}))

	csrf := webuitest.CSRFToken(t, home.HTML())
	out := b.Post(t, "/logout", webuitest.Form(url.Values{session.CSRFField: {csrf}}))
	t.Must(webuitest.Cookie(out, "__Host-session", webuitest.CookieWant{HostPrefix: true, Cleared: true}))
	t.Must(webuitest.Location(b.Get(t, "/"), http.StatusSeeOther, ct.Exact("/login")))
	t.Must(be.Equal(int32(1), app.logins.Load()))
	t.Must(be.Equal(int32(1), app.proto.Load()))
}

type errStatus int

func (e errStatus) Error() string { return http.StatusText(int(e)) }

func TestBrowserHTTP2AndInsecure(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("http2", func(t devtest.T) {
		app, h := newAccountApp(t, webuitest.BaseURL, false)
		b := webuitest.NewBrowser(t, h, webuitest.BrowserOptions{HTTP2: true})
		t.Must(b.Get(t, "/").Status(http.StatusSeeOther))
		t.Must(be.Equal(int32(2), app.proto.Load()))
	})
	t.Run("insecure", func(t devtest.T) {
		_, h := newAccountApp(t, webuitest.InsecureBaseURL, true)
		b := webuitest.NewBrowser(t, h, webuitest.BrowserOptions{Insecure: true})
		t.Must(be.Equal("http://localhost", b.URL()))
		page := b.Get(t, "/login").HTML()
		res := b.Submit(t, page, page.Find(ct.Tag("form")), nil, url.Values{"name": {"Bo"}})
		t.Must(webuitest.Cookie(res, "session", webuitest.CookieWant{HttpOnly: true}))
		t.Must(b.Get(t, "/").Status(http.StatusOK))
	})
	t.Run("tls server name is the site", func(t devtest.T) {
		sni := func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil {
				http.Error(w, "no tls", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, r.Host+" "+r.TLS.ServerName)
		}
		for _, o := range []webuitest.BrowserOptions{{}, {HTTP2: true}} {
			b := webuitest.NewBrowser(t, http.HandlerFunc(sni), o)
			res := b.Get(t, "/")
			t.Must(res.Status(http.StatusOK))
			t.Must(be.Equal("app.example.test app.example.test", string(res.Bytes())))
			// Requests built for handlers agree.
			res = webuitest.Serve(t, http.HandlerFunc(sni), webuitest.Get(t, "/"))
			t.Must(be.Equal("app.example.test app.example.test", string(res.Bytes())))
		}
	})
	t.Run("configure", func(t devtest.T) {
		configured := false
		b := webuitest.NewBrowser(t, http.NotFoundHandler(), webuitest.BrowserOptions{Configure: func(s *http.Server) { configured = s != nil }})
		t.Must(be.True(configured))
		t.Must(be.True(b.Server() != nil))
		t.Must(b.Get(t, "/").Status(http.StatusNotFound))
	})
}

// loopbackSite is the site of an application served over plain http on a
// loopback address.
const loopbackSite = "http://127.0.0.1:8080"

// TestBase serves an application at a loopback site through a Browser and
// a Client with that base: requests name the site, and the application's
// host check and same-origin policy accept them.
func TestBase(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("browser", func(t devtest.T) {
		_, h := newAccountApp(t, loopbackSite, true)
		b := webuitest.NewBrowser(t, h, webuitest.BrowserOptions{Base: loopbackSite + "/"})
		t.Must(be.Equal(loopbackSite, b.URL()))
		t.Must(webuitest.Location(b.Get(t, "/"), http.StatusSeeOther, ct.Exact("/login")))
		page := b.Get(t, "/login").HTML()
		res := b.Submit(t, page, page.Find(ct.Tag("form")), nil, url.Values{"name": {"Ada"}})
		t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/")))
		t.Must(webuitest.Cookie(res, "session", webuitest.CookieWant{NotSecure: true, HttpOnly: true}))
		t.Must(b.Get(t, "/").HTML().Find(ct.Role("heading", ct.Level(1))).Texts("Hello Ada"))
		st := b.Stream(t, "/live")
		st.Expect(t, "tasks")
	})
	t.Run("client", func(t devtest.T) {
		_, h := newAccountApp(t, loopbackSite, true)
		c := &webuitest.Client{Handler: h, Base: loopbackSite}
		t.Must(be.Equal(loopbackSite, c.URL()))
		login := c.Get(t, "/login")
		page := c.Page(t, login, http.StatusOK)
		res := c.Submit(t, page, page.Find(ct.Tag("form")), nil, url.Values{"name": {"Ada"}}, webuitest.SendCookies(login))
		t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/")))
		home := c.Page(t, c.Get(t, "/", webuitest.SendCookies(res)), http.StatusOK)
		t.Must(home.Find(ct.Role("heading", ct.Level(1))).Texts("Hello Ada"))
		// On BaseURL, the application refuses the requests.
		t.Must(c.Get(t, webuitest.BaseURL+"/login").Status(http.StatusBadRequest))
	})
	t.Run("what the handler sees", func(t devtest.T) {
		echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sni := "-"
			if r.TLS != nil {
				sni = r.TLS.ServerName
			}
			_, _ = io.WriteString(w, r.Host+" "+r.Header.Get("Origin")+" "+sni)
		})
		for _, c := range []struct{ base, want string }{
			{loopbackSite, "127.0.0.1:8080 http://127.0.0.1:8080 -"},
			{"HTTPS://Notes.Example.Test:8443", "notes.example.test:8443 https://notes.example.test:8443 notes.example.test"},
			{"https://notes.example.test:443", "notes.example.test https://notes.example.test notes.example.test"},
			{"http://[::1]:8080", "[::1]:8080 http://[::1]:8080 -"},
			// Normalized as mw.ParseSite normalizes the site.
			{"https://notes.example.test:0443", "notes.example.test https://notes.example.test notes.example.test"},
			{"http://127.0.0.1:08080", "127.0.0.1:8080 http://127.0.0.1:8080 -"},
			{"http://[0:0:0:0:0:0:0:1]:8080", "[::1]:8080 http://[::1]:8080 -"},
		} {
			client := &webuitest.Client{Handler: echo, Base: c.base}
			t.Mustf(client.Post(t, "/x", webuitest.Body{}).Body(ct.Exact(c.want)), "client on %s", c.base)
			for _, http2 := range []bool{false, true} {
				b := webuitest.NewBrowser(t, echo, webuitest.BrowserOptions{Base: c.base, HTTP2: http2})
				t.Mustf(b.Post(t, "/x", webuitest.Body{}).Body(ct.Exact(c.want)), "browser on %s", c.base)
			}
			// The host check of the same site accepts the requests.
			checked := &webuitest.Client{Handler: mw.HostCheck(mw.MustParseSite(c.base))(echo), Base: c.base}
			t.Mustf(checked.Post(t, "/x", webuitest.Body{}).Status(http.StatusOK), "host check of %s", c.base)
		}
	})
	t.Run("htmx", func(t devtest.T) {
		var got string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Host + " " + r.URL.Path + " " + r.Header.Get("HX-Current-URL")
		})
		c := &webuitest.Client{Handler: h, Base: loopbackSite}
		page := cthtml.PageString(htmlPage(`<button id="go" type="button" hx-post="/tasks/clear">Clear</button>` +
			`<button id="away" type="button" hx-post="` + webuitest.BaseURL + `/tasks">Away</button>`))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("go")), nil, nil, webuitest.PageURL("/tasks")).Status(http.StatusOK))
		t.Must(be.Equal("127.0.0.1:8080 /tasks/clear "+loopbackSite+"/tasks", got))
		// A URL of another origin than the page's is refused, as htmx does.
		r := newRecordT(t.Context())
		t.Must(be.True(c.SubmitHTMX(r, page, page.Find(ct.ID("away")), nil, nil) == nil))
		t.Must(be.Substring(r.output(), "is not of the page's origin "+loopbackSite))
	})
	t.Run("invalid", func(t devtest.T) {
		for _, base := range []string{"ftp://example.test", "127.0.0.1:8080", "http://", "https://example.test/app", "https://example.test?x", "https://user@example.test", "https://example.test#x", "http://%zz", "http://example.test", "https://ex_ample.test", "https://example.test:99999"} {
			r := newRecordT(t.Context())
			c := &webuitest.Client{Handler: http.NotFoundHandler(), Base: base}
			t.Mustf(be.True(c.Get(r, "/") == nil), "client on %q", base)
			t.Must(be.True(r.failed))
			t.Must(be.Substring(r.output(), "webuitest.Client: "))
			t.Must(be.Equal(base, c.URL()))
			r = newRecordT(t.Context())
			t.Mustf(be.True(webuitest.NewBrowser(r, http.NotFoundHandler(), webuitest.BrowserOptions{Base: base}) == nil), "browser on %q", base)
			t.Must(be.Substring(r.output(), "webuitest.NewBrowser: "))
		}
		r := newRecordT(t.Context())
		t.Must(be.True(webuitest.NewBrowser(r, http.NotFoundHandler(), webuitest.BrowserOptions{Base: "https://notes.example.test", Insecure: true}) == nil))
		t.Must(be.Substring(r.output(), `webuitest.NewBrowser: Insecure with the https Base "https://notes.example.test"`))
		// Insecure and an http Base agree.
		b := webuitest.NewBrowser(t, http.NotFoundHandler(), webuitest.BrowserOptions{Base: loopbackSite, Insecure: true})
		t.Must(be.Equal(loopbackSite, b.URL()))
	})
}

// TestCookieKeeping pins the documented difference between the harnesses:
// a Client keeps no cookies, a Browser keeps them, Secure ones over plain
// http on localhost too, as browsers do.
func TestCookieKeeping(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /set", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "__Host-items", Value: "kept", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /get", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("__Host-items")
		if err != nil {
			http.Error(w, "no cookie", http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, c.Value)
	})
	t.Run("client", func(t devtest.T) {
		c := &webuitest.Client{Handler: mux}
		t.Must(c.Get(t, "/set").Status(http.StatusNoContent))
		t.Must(c.Get(t, "/get").Status(http.StatusNotFound))
		t.Must(c.Get(t, "/get", webuitest.SendCookie("__Host-items", "sent")).Body(ct.Exact("sent")))
		// It sends the cookies of a response back when told to.
		set := c.Get(t, "/set")
		t.Must(c.Get(t, "/get", webuitest.SendCookies(set)).Body(ct.Exact("kept")))
		t.Must(be.Equal("kept", webuitest.CookieValue(t, set, "__Host-items")))
	})
	for name, o := range map[string]webuitest.BrowserOptions{"https": {}, "insecure": {Insecure: true}} {
		t.Run("browser "+name, func(t devtest.T) {
			b := webuitest.NewBrowser(t, mux, o)
			t.Must(b.Get(t, "/get").Status(http.StatusNotFound))
			t.Must(b.Get(t, "/set").Status(http.StatusNoContent))
			t.Must(b.Get(t, "/get").Body(ct.Exact("kept")))
			t.Must(b.Fork().Get(t, "/get").Status(http.StatusNotFound))
		})
	}
}

func TestBrowserStream(gt *testing.T) {
	t := devtest.ParallelT(gt)
	_, h := newAccountApp(t, webuitest.BaseURL, false)
	b := webuitest.NewBrowser(t, h, webuitest.BrowserOptions{})
	st := b.Stream(t, "/live")
	t.Must(st.Status(http.StatusOK))
	t.Must(st.Header("Content-Type", ct.Prefix(sse.ContentType[:17])))
	st.Expect(t, "tasks")
	st.Expect(t, sse.EventEnd)
	st.ExpectEnd(t)
	r := newRecordT(t.Context())
	st.Returned(r)
	t.Must(be.Substring(r.output(), "Returned needs a stream of ServeStream"))

	// A refused stream request fails the test without a stream.
	r = newRecordT(t.Context())
	t.Must(be.True(webuitest.NewBrowser(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}), webuitest.BrowserOptions{}).Stream(r, "/x") == nil))
	t.Must(be.True(r.failed))
}

func TestCrossSiteProbes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "flash", Value: "added"})
		w.WriteHeader(http.StatusNoContent)
	})
	unchanged := func() error {
		if n := calls.Load(); n != 0 {
			return errStatus(http.StatusConflict)
		}
		return nil
	}
	strict := &webuitest.Client{Handler: mw.Chain(mux, mw.HostCheck(mw.MustParseSite(webuitest.BaseURL)), mw.SameOrigin(mw.OriginPolicy{}))}
	t.Must(webuitest.RejectsCrossSite(t, strict, http.MethodPost, "/tasks", webuitest.Form(url.Values{"title": {"x"}}), unchanged))
	// A same-origin request passes the stack, so the probes are meaningful.
	t.Must(strict.Post(t, "/tasks", webuitest.Body{}).Status(http.StatusNoContent))

	calls.Store(0)
	weak := &webuitest.Client{Handler: mux}
	err := webuitest.RejectsCrossSite(t, weak, http.MethodPost, "/tasks", webuitest.Body{}, unchanged).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrMismatch))
	for _, want := range []string{
		`rejectsCrossSite(POST /tasks): `,
		`probe "cross-site": `,
		"POST /tasks got status 204, want 400 or 403",
		`sets 1 cookies: "flash"`,
		`probe "dns rebinding" changed the state`,
	} {
		t.Must(be.ErrorContains(err, want))
	}
	// An origin check by prefix (or by host without the port), behind fetch
	// metadata, passes the probes with foreign origins but not the
	// lookalikes.
	calls.Store(0)
	prefix := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch site := r.Header.Get("Sec-Fetch-Site"); {
		case site == "same-origin" || site == "none":
		case site != "":
			http.Error(w, "cross-site", http.StatusForbidden)
			return
		case !strings.HasPrefix(r.Header.Get("Origin"), webuitest.BaseURL) && !strings.HasPrefix(r.Header.Get("Referer"), webuitest.BaseURL):
			http.Error(w, "no origin", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
	lax := &webuitest.Client{Handler: mw.Chain(prefix, mw.HostCheck(mw.MustParseSite(webuitest.BaseURL)))}
	err = webuitest.RejectsCrossSite(t, lax, http.MethodPost, "/tasks", webuitest.Body{}, nil).Check(t.Context())
	for _, want := range []string{`probe "lookalike origin"`, `probe "other port origin"`, `probe "lookalike referer only"`} {
		t.Must(be.ErrorContains(err, want))
	}
	for _, refused := range []string{`probe "cross-site"`, `probe "foreign origin without fetch metadata"`, `probe "suffix origin"`, `probe "null origin"`} {
		t.Must(be.False(strings.Contains(err.Error(), refused)))
	}

	names := map[string]bool{}
	for _, p := range webuitest.CrossSiteProbes() {
		t.Must(be.False(names[p.Name]))
		names[p.Name] = true
		t.Must(be.NonEmpty(p.Options))
	}
}

// probeRequests returns the probes as requests for "/x": the name, Host
// and origin headers of each.
func probeRequests(t devtest.T, probes []webuitest.Probe) []string {
	var out []string
	for _, p := range probes {
		req := webuitest.Post(t, "/x", webuitest.Body{}, p.Options...)
		out = append(out, strings.Join([]string{p.Name, req.Host, req.Header.Get("Sec-Fetch-Site"), req.Header.Get("Origin"), req.Header.Get("Referer")}, " | "))
	}
	return out
}

// TestCrossSiteProbesFor probes an application at a loopback site, whose
// look-alikes the probes of BaseURL do not resemble.
func TestCrossSiteProbesFor(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.DeepEqual(probeRequests(t, webuitest.CrossSiteProbes()), probeRequests(t, webuitest.CrossSiteProbesFor(webuitest.BaseURL))))
	t.Must(be.DeepEqual([]string{
		"cross-site | app.example.test | cross-site | https://evil.example | ",
		"same-site | app.example.test | same-site | http://127.0.0.1:8443 | ",
		"foreign origin without fetch metadata | app.example.test |  | https://evil.example | ",
		"lookalike origin | app.example.test |  | http://127.0.0.1:8080.evil.example | ",
		"suffix origin | app.example.test |  | http://evil127.0.0.1:8080 | ",
		"other port origin | app.example.test |  | http://127.0.0.1:8443 | ",
		"null origin | app.example.test |  | null | ",
		"https origin | app.example.test |  | https://127.0.0.1:8080 | ",
		"foreign referer only | app.example.test |  |  | https://evil.example/page",
		"lookalike referer only | app.example.test |  |  | http://127.0.0.1:8080.evil.example/page",
		"dns rebinding | evil.example:8080 | same-origin | http://evil.example:8080 | ",
	}, probeRequests(t, webuitest.CrossSiteProbesFor(loopbackSite))))
	for site, want := range map[string]string{
		"https://example.com":                "https://other.example.com",
		"https://other.example.com:8443":     "https://another.example.com:8443",
		"http://localhost:3000":              "http://localhost:8443",
		"http://[::1]:8443":                  "http://[::1]:8444",
		"https://notes.app.example.test:443": "https://other.app.example.test",
	} {
		got := probeRequests(t, webuitest.CrossSiteProbesFor(site))[1]
		t.Mustf(be.Substring(got, "| same-site | "+want+" |"), "same-site probe of %s", site)
	}
	t.Must(be.Panic(func() { webuitest.CrossSiteProbesFor("localhost:8080") }))

	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	unchanged := func() error {
		if n := calls.Load(); n != 0 {
			return errStatus(http.StatusConflict)
		}
		return nil
	}
	site := mw.MustParseSite(loopbackSite)
	body := webuitest.Form(url.Values{"title": {"x"}})

	// webui's policy refuses every probe at the site, and accepts the
	// requests of the site itself, also without origin headers.
	strict := mw.Chain(mux, mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{}))
	c := &webuitest.Client{Handler: strict, Base: loopbackSite}
	t.Must(webuitest.RejectsCrossSite(t, c, http.MethodPost, "/tasks", body, unchanged))
	b := webuitest.NewBrowser(t, strict, webuitest.BrowserOptions{Base: loopbackSite})
	t.Must(webuitest.RejectsCrossSite(t, b, http.MethodPost, "/tasks", body, unchanged))
	t.Must(c.Post(t, "/tasks", body, webuitest.NoFetchMetadata()).Status(http.StatusNoContent))
	// An application that refuses header-less requests there too checks
	// it itself.
	calls.Store(0)
	deny := &webuitest.Client{Handler: mw.Chain(mux, mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{Headerless: mw.HeaderlessDeny})), Base: loopbackSite}
	t.Must(webuitest.RejectsCrossSite(t, deny, http.MethodPost, "/tasks", body, unchanged))
	t.Must(deny.Post(t, "/tasks", body, webuitest.NoFetchMetadata()).Status(http.StatusForbidden))

	// An origin check by prefix, behind fetch metadata, lets the
	// look-alikes of the site through, which the probes of BaseURL miss.
	prefix := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch fs := r.Header.Get("Sec-Fetch-Site"); {
		case fs == "same-origin" || fs == "none":
		case fs != "":
			http.Error(w, "cross-site", http.StatusForbidden)
			return
		case !strings.HasPrefix(r.Header.Get("Origin"), loopbackSite) && !strings.HasPrefix(r.Header.Get("Referer"), loopbackSite):
			http.Error(w, "no origin", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
	lax := &webuitest.Client{Handler: mw.Chain(prefix, mw.HostCheck(site)), Base: loopbackSite}
	calls.Store(0)
	err := webuitest.RejectsCrossSite(t, lax, http.MethodPost, "/tasks", body, nil).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrMismatch))
	for _, want := range []string{`probe "lookalike origin"`, `probe "lookalike referer only"`} {
		t.Must(be.ErrorContains(err, want))
	}
	for _, p := range webuitest.CrossSiteProbes() {
		t.Mustf(be.False(lax.Post(t, "/tasks", body, p.Options...).StatusCode() == http.StatusNoContent), "probe %q of BaseURL", p.Name)
	}
}

// safeNext is a reference check of a redirect target: a local path that
// browsers cannot read as another host.
func safeNext(v string) bool {
	if v == "" || v[0] != '/' || len(v) > 1 && (v[1] == '/' || v[1] == '\\') {
		return false
	}
	for i := 0; i < len(v); i++ {
		if c := v[i]; c < 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "" && u.Host == "" && u.User == nil
}

func TestRedirectVectors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, v := range webuitest.OpenRedirectVectors() {
		t.Mustf(be.False(safeNext(v)), "vector %q", v)
	}
	for _, v := range webuitest.LocalRedirectTargets() {
		t.Mustf(be.True(safeNext(v)), "target %q", v)
	}
}

func TestHostile(gt *testing.T) {
	t := devtest.ParallelT(gt)
	inputs := webuitest.Hostile()
	t.Must(be.True(len(inputs) > 10))
	// The HTML parser normalizes line breaks and NUL in attribute values.
	normalize := strings.NewReplacer("\r\n", "\n", "\r", "\n", "\x00", "\uFFFD")
	for _, h := range inputs {
		view := section.Main()(text.P(attr.ID("echo"), attr.Title(h))(text.Text(h)))
		t.Must(ct.View(view).Valid(webuitest.Rules()...))
		var b strings.Builder
		t.Must(be.NoError(core.Render(t.Context(), view, &b)))
		parsed := cthtml.FragmentString("div", b.String())
		t.Must(parsed.Valid(webuitest.Rules()...))
		t.Must(parsed.Find(ct.ID("echo")).Matches(ct.Attr("title", normalize.Replace(h))))
		t.Must(parsed.Find(ct.Tag("script")).None())
	}
}
