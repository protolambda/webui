package webuitest_test

import (
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
	t.Run("configure", func(t devtest.T) {
		configured := false
		b := webuitest.NewBrowser(t, http.NotFoundHandler(), webuitest.BrowserOptions{Configure: func(s *http.Server) { configured = s != nil }})
		t.Must(be.True(configured))
		t.Must(be.True(b.Server() != nil))
		t.Must(b.Get(t, "/").Status(http.StatusNotFound))
	})
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
	names := map[string]bool{}
	for _, p := range webuitest.CrossSiteProbes() {
		t.Must(be.False(names[p.Name]))
		names[p.Name] = true
		t.Must(be.NonEmpty(p.Options))
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
