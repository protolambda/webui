package webuitest_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/webuitest"
)

// respond serves a request with a handler that writes the response f
// shapes.
func respond(t devtest.T, f func(w http.ResponseWriter)) *cthttp.Result {
	return webuitest.Serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f(w) }), webuitest.Get(t, "/x"))
}

func TestLocation(gt *testing.T) {
	t := devtest.ParallelT(gt)
	res := respond(t, func(w http.ResponseWriter) {
		w.Header().Set("Location", "/tasks")
		w.WriteHeader(http.StatusSeeOther)
	})
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/tasks")))
	err := webuitest.Location(res, http.StatusFound, ct.Prefix("/notes")).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrMismatch))
	t.Must(be.ErrorContains(err, `location(GET /x, 302, prefix("/notes")): 2 failures`))
}

func TestHXHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	res := respond(t, func(w http.ResponseWriter) {
		hx2.SetRedirect(w.Header(), "/done")
		hx2.SetRetarget(w.Header(), "#alerts")
		hx2.SetRefresh(w.Header())
	})
	t.Must(webuitest.HXRedirect(res, ct.Exact("/done")))
	t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
	t.Must(webuitest.HXRefresh(res))
	redirect := respond(t, func(w http.ResponseWriter) {
		hx2.SetRedirect(w.Header(), "/done")
		w.WriteHeader(http.StatusSeeOther)
	})
	err := webuitest.HXRedirect(redirect, ct.Exact("/done")).Check(t.Context())
	t.Must(be.ErrorContains(err, "has status 303: a browser follows a redirect before htmx sees the response"))
	err = webuitest.HXRefresh(respond(t, func(http.ResponseWriter) {})).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrMismatch))
}

const cookieValue = "cookie-value-never-printed"

func TestCookie(gt *testing.T) {
	t := devtest.ParallelT(gt)
	serve := func(c *http.Cookie) *cthttp.Result {
		return respond(t, func(w http.ResponseWriter) { http.SetCookie(w, c) })
	}
	good := &http.Cookie{Name: "__Host-session", Value: cookieValue, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
	want := webuitest.CookieWant{HostPrefix: true, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Path: "/", Session: true}
	t.Must(webuitest.Cookie(serve(good), "__Host-session", want))

	cases := []struct {
		name   string
		cookie http.Cookie
		want   webuitest.CookieWant
		msg    string
	}{
		{"missing", http.Cookie{Name: "other", Value: cookieValue}, want, `GET /x sets no cookie "__Host-session" (it sets other)`},
		{"prefix", http.Cookie{Name: "__Host-session", Value: cookieValue, Path: "/x", Secure: true}, webuitest.CookieWant{HostPrefix: true}, `a __Host- cookie needs Path=/, has "/x"`},
		{"not secure", http.Cookie{Name: "__Host-session", Value: cookieValue, Path: "/"}, webuitest.CookieWant{HostPrefix: true}, "a __Host- cookie needs Secure"},
		{"domain", http.Cookie{Name: "__Host-session", Value: cookieValue, Path: "/", Secure: true, Domain: "example.test"}, webuitest.CookieWant{HostPrefix: true}, "must not have a Domain"},
		{"secure", http.Cookie{Name: "__Host-session", Value: cookieValue}, webuitest.CookieWant{Secure: true}, "not Secure"},
		{"http only", http.Cookie{Name: "__Host-session", Value: cookieValue}, webuitest.CookieWant{HttpOnly: true}, "not HttpOnly"},
		{"same site", http.Cookie{Name: "__Host-session", Value: cookieValue, SameSite: http.SameSiteNoneMode, Secure: true}, webuitest.CookieWant{SameSite: http.SameSiteStrictMode}, "SameSite is None, want Strict"},
		{"same site unset", http.Cookie{Name: "__Host-session", Value: cookieValue}, webuitest.CookieWant{SameSite: http.SameSiteLaxMode}, "SameSite is unset, want Lax"},
		{"path", http.Cookie{Name: "__Host-session", Value: cookieValue, Path: "/a"}, webuitest.CookieWant{Path: "/"}, `Path is "/a", want "/"`},
		{"persistent", http.Cookie{Name: "__Host-session", Value: cookieValue, MaxAge: 60}, webuitest.CookieWant{Session: true}, "has Max-Age or Expires, want a session cookie"},
		{"max age", http.Cookie{Name: "__Host-session", Value: cookieValue, MaxAge: 60}, webuitest.CookieWant{MaxAge: time.Hour}, "Max-Age is 60, want 3600"},
		{"cleared", http.Cookie{Name: "__Host-session", MaxAge: -1}, webuitest.CookieWant{}, "the cookie is cleared"},
		{"not cleared", http.Cookie{Name: "__Host-session", Value: cookieValue}, webuitest.CookieWant{Cleared: true}, "not cleared"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			err := webuitest.Cookie(serve(&c.cookie), "__Host-session", c.want).Check(t.Context())
			t.Must(be.ErrorIs(err, ct.ErrMismatch))
			t.Must(be.ErrorContains(err, c.msg))
			t.Must(be.False(strings.Contains(err.Error(), cookieValue)))
		})
	}
	t.Run("cleared ways", func(t devtest.T) {
		for _, c := range []*http.Cookie{
			{Name: "s", MaxAge: -1},
			{Name: "s", Expires: time.Unix(1, 0)},
		} {
			t.Must(webuitest.Cookie(serve(c), "s", webuitest.CookieWant{Cleared: true}))
		}
	})
	t.Run("twice", func(t devtest.T) {
		res := respond(t, func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "s", Value: "1"})
			http.SetCookie(w, &http.Cookie{Name: "s", Value: "2"})
		})
		t.Must(be.ErrorContains(webuitest.Cookie(res, "s", webuitest.CookieWant{}).Check(t.Context()), `sets cookie "s" 2 times`))
	})
	t.Run("capture failure", func(t devtest.T) {
		res := cthttp.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.SetCookie(w, good)
			_, _ = w.Write([]byte("too long"))
		}), webuitest.Get(t, "/x"), cthttp.WithMaxBody(1))
		t.Must(be.ErrorIs(webuitest.Cookie(res, "__Host-session", want).Check(t.Context()), ct.ErrLoad))
	})
	t.Run("session store", func(t devtest.T) {
		store, err := session.NewStore(session.Options[string]{})
		t.Must(be.NoError(err))
		h := http.NewServeMux()
		h.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
			if _, err := store.New(w, r, "account"); err != nil {
				http.Error(w, "no", http.StatusInternalServerError)
			}
		})
		h.HandleFunc("POST /logout", store.End)
		login := webuitest.Serve(t, h, webuitest.Post(t, "/login", webuitest.Body{}))
		t.Must(webuitest.Cookie(login, "__Host-session", want))
		logout := webuitest.Serve(t, h, webuitest.Post(t, "/logout", webuitest.Body{}))
		t.Must(webuitest.Cookie(logout, "__Host-session", webuitest.CookieWant{HostPrefix: true, Cleared: true}))
	})
}

func TestNoCookies(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(webuitest.NoCookies(respond(t, func(http.ResponseWriter) {})))
	res := respond(t, func(w http.ResponseWriter) {
		http.SetCookie(w, &http.Cookie{Name: "a", Value: cookieValue})
		w.Header().Add("Set-Cookie", "=broken")
	})
	err := webuitest.NoCookies(res).Check(t.Context())
	t.Must(be.ErrorContains(err, `GET /x sets 2 cookies: "a" (1 do not parse)`))
	t.Must(be.False(strings.Contains(err.Error(), cookieValue)))
}

// secured serves a page behind mw.SecurityHeaders with the configuration.
func secured(t devtest.T, cfg mw.Headers, page func(w http.ResponseWriter)) *cthttp.Result {
	h := mw.SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { page(w) }))
	return webuitest.Serve(t, h, webuitest.Get(t, "/"))
}

func TestSecurityHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	plain := func(http.ResponseWriter) {}
	t.Run("defaults", func(t devtest.T) {
		res := secured(t, mw.Headers{}, plain)
		t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
		t.Must(webuitest.SecurityHeaders(res, webuitest.Policy{}))
		t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{})))
	})
	t.Run("configured", func(t devtest.T) {
		cfg := mw.Headers{
			CSP:          mw.StrictCSP().Add(mw.ImgSrc, "https://img.example.test").With(mw.FrameAncestors, mw.SourceSelf),
			CacheControl: "private, no-cache",
			HSTS:         mw.HSTS{MaxAge: 365 * 24 * time.Hour, IncludeSubDomains: true},
		}
		res := secured(t, cfg, plain)
		t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(cfg)))
		err := webuitest.SecurityHeaders(res, webuitest.StrictPolicy()).Check(t.Context())
		t.Must(be.ErrorIs(err, ct.ErrMismatch))
		for _, want := range []string{
			"CSP img-src is 'self' data: https://img.example.test, want 'self' data:",
			"CSP frame-ancestors is 'self', want 'none'",
			`has X-Frame-Options "SAMEORIGIN", want "DENY"`,
			`has Cache-Control "private, no-cache", want "no-store"`,
			`has Strict-Transport-Security "max-age=31536000; includeSubDomains", want none`,
		} {
			t.Must(be.ErrorContains(err, want))
		}
	})
	t.Run("no headers", func(t devtest.T) {
		err := webuitest.SecurityHeaders(respond(t, plain), webuitest.StrictPolicy()).Check(t.Context())
		for _, want := range []string{
			"security headers of GET /x:",
			"has no Content-Security-Policy",
			`has no X-Frame-Options, want "DENY"`,
			`has no X-Content-Type-Options, want "nosniff"`,
			`has no Referrer-Policy, want "same-origin"`,
			"has no Permissions-Policy",
			`has no Cross-Origin-Opener-Policy, want "same-origin"`,
			`has no Cross-Origin-Resource-Policy, want "same-origin"`,
		} {
			t.Must(be.ErrorContains(err, want))
		}
	})
	t.Run("unsafe", func(t devtest.T) {
		weak := mw.StrictCSP().With(mw.StyleSrc, mw.SourceSelf, "'unsafe-inline'")
		res := secured(t, mw.Headers{}, func(w http.ResponseWriter) { mw.SetCSP(w, weak) })
		err := webuitest.SecurityHeaders(res, webuitest.StrictPolicy()).Check(t.Context())
		t.Must(be.ErrorContains(err, "CSP style-src allows 'unsafe-inline'"))
		t.Must(be.ErrorContains(err, "CSP style-src is 'self' 'unsafe-inline', want 'self'"))
		// The expected policy is fenced too, unless unsafe sources are allowed.
		err = webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{CSP: weak})).Check(t.Context())
		t.Must(be.ErrorContains(err, "CSP style-src allows 'unsafe-inline'"))
		t.Must(webuitest.SecurityHeaders(res, webuitest.Policy{Headers: mw.Headers{CSP: weak}, AllowUnsafe: true}))
	})
	t.Run("policy differences", func(t devtest.T) {
		res := secured(t, mw.Headers{}, func(w http.ResponseWriter) {
			w.Header().Set("Content-Security-Policy", "script-src 'self'; default-src 'none'; img-src data: 'SELF'; object-src 'none'")
		})
		err := webuitest.SecurityHeaders(res, webuitest.StrictPolicy()).Check(t.Context())
		t.Must(be.ErrorContains(err, "CSP lacks style-src, want 'self'"))
		t.Must(be.ErrorContains(err, "CSP has unexpected object-src"))
		t.Must(be.False(strings.Contains(err.Error(), "CSP img-src")))
	})
	t.Run("malformed", func(t devtest.T) {
		res := secured(t, mw.Headers{}, func(w http.ResponseWriter) {
			w.Header().Set("Content-Security-Policy", "script-src self")
		})
		t.Must(be.ErrorContains(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()).Check(t.Context()), "Content-Security-Policy does not parse"))
		res = secured(t, mw.Headers{}, func(w http.ResponseWriter) {
			w.Header().Add("Content-Security-Policy", "default-src 'none'")
			w.Header().Add("X-Content-Type-Options", "nosniff")
		})
		err := webuitest.SecurityHeaders(res, webuitest.StrictPolicy()).Check(t.Context())
		t.Must(be.ErrorContains(err, "has 2 Content-Security-Policy headers, which browsers all enforce"))
		t.Must(be.ErrorContains(err, "has 2 X-Content-Type-Options headers"))
	})
	t.Run("assets", func(t devtest.T) {
		res := secured(t, mw.Headers{}, func(w http.ResponseWriter) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		})
		t.Must(webuitest.SecurityHeaders(res, webuitest.Policy{Headers: mw.Headers{CacheControl: "public, max-age=31536000, immutable"}}))
	})
	t.Run("hash sources keep their case", func(t devtest.T) {
		csp := mw.StrictCSP().Add(mw.ScriptSrc, "'sha256-abc='")
		res := secured(t, mw.Headers{CSP: csp}, plain)
		t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{CSP: csp})))
		err := webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{CSP: mw.StrictCSP().Add(mw.ScriptSrc, "'sha256-ABC='")})).Check(t.Context())
		t.Must(be.ErrorContains(err, "CSP script-src is"))
	})
}
