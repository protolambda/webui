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

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/theme"
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
		{"secure, want not", http.Cookie{Name: "__Host-session", Value: cookieValue, Secure: true}, webuitest.CookieWant{NotSecure: true}, "Secure, want no Secure attribute"},
		{"http only, want not", http.Cookie{Name: "__Host-session", Value: cookieValue, HttpOnly: true}, webuitest.CookieWant{NotHttpOnly: true}, "HttpOnly, want no HttpOnly attribute"},
		{"secure and not", http.Cookie{Name: "__Host-session", Value: cookieValue}, webuitest.CookieWant{Secure: true, NotSecure: true}, "the want requires Secure (or HostPrefix) and NotSecure"},
		{"prefix and not secure", http.Cookie{Name: "__Host-session", Value: cookieValue, Path: "/", Secure: true}, webuitest.CookieWant{HostPrefix: true, NotSecure: true}, "the want requires Secure (or HostPrefix) and NotSecure"},
		{"http only and not", http.Cookie{Name: "__Host-session", Value: cookieValue, HttpOnly: true}, webuitest.CookieWant{HttpOnly: true, NotHttpOnly: true}, "the want requires HttpOnly and NotHttpOnly"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			err := webuitest.Cookie(serve(&c.cookie), "__Host-session", c.want).Check(t.Context())
			t.Must(be.ErrorIs(err, ct.ErrMismatch))
			t.Must(be.ErrorContains(err, c.msg))
			t.Must(be.False(strings.Contains(err.Error(), cookieValue)))
		})
	}
	t.Run("absent attributes", func(t devtest.T) {
		// The session cookie of a plain http site: neither Secure nor
		// __Host-, and here readable by scripts.
		plain := &http.Cookie{Name: "session", Value: cookieValue, Path: "/", SameSite: http.SameSiteLaxMode}
		t.Must(webuitest.Cookie(serve(plain), "session", webuitest.CookieWant{NotSecure: true, NotHttpOnly: true, SameSite: http.SameSiteLaxMode}))
		err := webuitest.Cookie(serve(good), "__Host-session", webuitest.CookieWant{NotSecure: true, NotHttpOnly: true}).Check(t.Context())
		t.Must(be.ErrorContains(err, "Secure, want no Secure attribute; HttpOnly, want no HttpOnly attribute"))
	})
	t.Run("cleared ways", func(t devtest.T) {
		for _, c := range []*http.Cookie{
			{Name: "s", MaxAge: -1},
			{Name: "s", Expires: time.Unix(1, 0)},
		} {
			t.Must(webuitest.Cookie(serve(c), "s", webuitest.CookieWant{Cleared: true}))
		}
	})
	t.Run("max-age wins over expires", func(t devtest.T) {
		// A positive Max-Age keeps the cookie whatever Expires says (RFC
		// 6265, section 5.3): not a logout, and not a cleared cookie.
		kept := &http.Cookie{Name: "s", Value: cookieValue, MaxAge: 3600, Expires: time.Unix(1, 0)}
		err := webuitest.Cookie(serve(kept), "s", webuitest.CookieWant{Cleared: true}).Check(t.Context())
		t.Must(be.ErrorContains(err, "not cleared"))
		t.Must(webuitest.Cookie(serve(kept), "s", webuitest.CookieWant{MaxAge: time.Hour}))
		// Max-Age=0 clears it even with a future Expires.
		gone := &http.Cookie{Name: "s", MaxAge: -1, Expires: time.Now().Add(time.Hour)}
		t.Must(webuitest.Cookie(serve(gone), "s", webuitest.CookieWant{Cleared: true}))
	})
	t.Run("expires against the application clock", func(t devtest.T) {
		now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		c := &http.Cookie{Name: "s", Value: cookieValue, Expires: now.Add(time.Hour)}
		t.Must(webuitest.Cookie(serve(c), "s", webuitest.CookieWant{Now: now}))
		err := webuitest.Cookie(serve(c), "s", webuitest.CookieWant{}).Check(t.Context())
		t.Must(be.ErrorContains(err, "the cookie is cleared"))
		t.Must(webuitest.Cookie(serve(c), "s", webuitest.CookieWant{Cleared: true, Now: now.Add(time.Hour)}))
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

func TestCookieValue(gt *testing.T) {
	t := devtest.ParallelT(gt)
	res := respond(t, func(w http.ResponseWriter) {
		http.SetCookie(w, &http.Cookie{Name: "theme", Value: "dark"})
		http.SetCookie(w, &http.Cookie{Name: "__Host-preauth", Value: cookieValue, Path: "/", Secure: true})
	})
	t.Must(be.Equal(cookieValue, webuitest.CookieValue(t, res, "__Host-preauth")))
	t.Must(be.Equal("dark", webuitest.CookieValue(t, res, "theme")))
	cleared := respond(t, func(w http.ResponseWriter) { http.SetCookie(w, &http.Cookie{Name: "s", MaxAge: -1}) })
	t.Must(be.Equal("", webuitest.CookieValue(t, cleared, "s")))

	cases := []struct {
		name string
		set  func(w http.ResponseWriter)
		msg  string
	}{
		{"missing", func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "theme", Value: cookieValue})
			w.Header().Add("Set-Cookie", "=broken")
		}, `webuitest.CookieValue: GET /x sets no cookie "s" (it sets theme); 1 Set-Cookie headers do not parse`},
		{"twice", func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue + "2"})
		}, `GET /x sets cookie "s" 2 times`},
		{"does not parse", func(w http.ResponseWriter) {
			w.Header().Add("Set-Cookie", `s="`+cookieValue+`"x`)
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
		}, `GET /x has a Set-Cookie header for "s" that does not parse`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			r := newRecordT(t.Context())
			t.Must(be.Equal("", webuitest.CookieValue(r, respond(t, c.set), "s")))
			t.Must(be.True(r.failed))
			t.Must(be.Substring(r.output(), c.msg))
			t.Must(be.False(strings.Contains(r.output(), cookieValue)))
		})
	}
	r := newRecordT(t.Context())
	webuitest.CookieValue(r, nil, "s")
	t.Must(be.Substring(r.output(), "webuitest.CookieValue: no response"))
	r = newRecordT(t.Context())
	tooLong := cthttp.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
		_, _ = w.Write([]byte("too long"))
	}), webuitest.Get(t, "/x"), cthttp.WithMaxBody(1))
	t.Must(be.Equal("", webuitest.CookieValue(r, tooLong, "s")))
	t.Must(be.True(r.failed))
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

func TestNoCookie(gt *testing.T) {
	t := devtest.ParallelT(gt)
	res := respond(t, func(w http.ResponseWriter) {
		http.SetCookie(w, &http.Cookie{Name: "__Host-preauth", Value: cookieValue, Path: "/", Secure: true})
		w.Header().Add("Set-Cookie", "=broken")
		w.Header().Add("Set-Cookie", "no-pair")
	})
	t.Must(webuitest.NoCookie(res, "__Host-session"))
	t.Must(webuitest.NoCookie(respond(t, func(http.ResponseWriter) {}), "__Host-session"))
	t.Must(be.Equal(`noCookie(GET /x, "__Host-session")`, webuitest.NoCookie(res, "__Host-session").String()))
	err := webuitest.NoCookie(res, "__Host-preauth").Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrMismatch))
	t.Must(be.ErrorContains(err, `GET /x sets cookie "__Host-preauth"`))
	t.Must(be.False(strings.Contains(err.Error(), cookieValue)))
	cases := []struct {
		name string
		set  func(w http.ResponseWriter)
		msg  string
	}{
		{"cleared", func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "s", MaxAge: -1})
		}, `sets cookie "s"`},
		{"twice", func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
		}, `sets cookie "s" 2 times`},
		{"does not parse", func(w http.ResponseWriter) {
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
			w.Header().Add("Set-Cookie", ` s = "`+cookieValue+`"x; Path=/`)
		}, `sets cookie "s" in 2 Set-Cookie headers, 1 of which do not parse`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			err := webuitest.NoCookie(respond(t, c.set), "s").Check(t.Context())
			t.Must(be.ErrorIs(err, ct.ErrMismatch))
			t.Must(be.ErrorContains(err, c.msg))
			t.Must(be.False(strings.Contains(err.Error(), cookieValue)))
		})
	}
	res = cthttp.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("too long"))
	}), webuitest.Get(t, "/x"), cthttp.WithMaxBody(1))
	t.Must(be.ErrorIs(webuitest.NoCookie(res, "s").Check(t.Context()), ct.ErrLoad))
}

// secured serves a page behind mw.SecurityHeaders with the configuration.
func secured(t devtest.T, cfg mw.Headers, page func(w http.ResponseWriter)) *cthttp.Result {
	h := mw.SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { page(w) }))
	return webuitest.Serve(t, h, webuitest.Get(t, "/"))
}

// TestAssetPolicy serves the files of an asset set behind webui's
// security headers: they keep every header of the middleware but the
// Cache-Control.
func TestAssetPolicy(gt *testing.T) {
	t := devtest.ParallelT(gt)
	set := theme.MustAssets(theme.Options{})
	serve := func(t devtest.T, cfg mw.Headers, target string) *cthttp.Result {
		return webuitest.Serve(t, mw.SecurityHeaders(cfg)(set), webuitest.Get(t, target))
	}
	t.Run("defaults", func(t devtest.T) {
		for info := range set.Files() {
			res := serve(t, mw.Headers{}, info.URL)
			t.Must(res.Status(http.StatusOK))
			t.Mustf(webuitest.SecurityHeaders(res, webuitest.AssetPolicy()), "GET %s", info.Name)
			t.Must(webuitest.SecurityHeaders(res, webuitest.AssetPolicyFor(mw.Headers{})))
			err := webuitest.SecurityHeaders(res, webuitest.StrictPolicy()).Check(t.Context())
			t.Must(be.ErrorContains(err, `has Cache-Control "public, max-age=31536000, immutable", want "no-store"`))
		}
	})
	t.Run("configured", func(t devtest.T) {
		cfg := mw.Headers{
			CSP:          mw.StrictCSP().Add(mw.ImgSrc, "https://img.example.test"),
			CacheControl: "private, no-cache",
			HSTS:         mw.HSTS{MaxAge: 365 * 24 * time.Hour},
		}
		res := serve(t, cfg, set.URL(theme.ThemeCSS))
		t.Must(webuitest.SecurityHeaders(res, webuitest.AssetPolicyFor(cfg)))
		err := webuitest.SecurityHeaders(res, webuitest.AssetPolicy()).Check(t.Context())
		t.Must(be.ErrorContains(err, "CSP img-src is 'self' data: https://img.example.test, want 'self' data:"))
		t.Must(be.ErrorContains(err, `has Strict-Transport-Security "max-age=31536000", want none`))
		t.Must(be.False(strings.Contains(err.Error(), "Cache-Control")))
	})
	t.Run("other version", func(t devtest.T) {
		res := serve(t, mw.Headers{}, set.Prefix()+"0123456789abcdef/"+theme.ThemeCSS)
		t.Must(res.Status(http.StatusOK))
		err := webuitest.SecurityHeaders(res, webuitest.AssetPolicy()).Check(t.Context())
		t.Must(be.ErrorContains(err, `has Cache-Control "no-cache", want "public, max-age=31536000, immutable"`))
		p := webuitest.AssetPolicy()
		p.Headers.CacheControl = asset.CacheRevalidate
		t.Must(webuitest.SecurityHeaders(res, p))
	})
	t.Run("not found", func(t devtest.T) {
		// An unknown file keeps the middleware's Cache-Control.
		res := serve(t, mw.Headers{}, set.Prefix()+"x/missing.css")
		t.Must(res.Status(http.StatusNotFound))
		t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
	})
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
	t.Run("frame options follow frame-ancestors", func(t devtest.T) {
		for _, c := range []struct {
			name string
			csp  mw.CSP
			want string
		}{
			{"absent", mw.StrictCSP().Without(mw.FrameAncestors), "DENY"},
			{"upper-case none", mw.StrictCSP().With(mw.FrameAncestors, "'NONE'"), "DENY"},
			{"upper-case self", mw.StrictCSP().With(mw.FrameAncestors, "'SELF'"), "SAMEORIGIN"},
			{"other origin", mw.StrictCSP().With(mw.FrameAncestors, mw.SourceSelf, "https://embed.example.test"), ""},
		} {
			t.Run(c.name, func(t devtest.T) {
				cfg := mw.Headers{CSP: c.csp}
				res := secured(t, cfg, plain)
				t.Must(be.Equal(c.want, res.Headers().Get("X-Frame-Options")))
				t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(cfg)))
			})
		}
		// Without frame-ancestors, a response that drops X-Frame-Options
		// can be framed by any site.
		cfg := mw.Headers{CSP: mw.StrictCSP().Without(mw.FrameAncestors)}
		res := secured(t, cfg, func(w http.ResponseWriter) { w.Header().Del("X-Frame-Options") })
		err := webuitest.SecurityHeaders(res, webuitest.PolicyFor(cfg)).Check(t.Context())
		t.Must(be.ErrorContains(err, `has no X-Frame-Options, want "DENY"`))
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
		t.Must(webuitest.SecurityHeaders(res, webuitest.AssetPolicy()))
	})
	t.Run("hash sources keep their case", func(t devtest.T) {
		csp := mw.StrictCSP().Add(mw.ScriptSrc, "'sha256-abc='")
		res := secured(t, mw.Headers{CSP: csp}, plain)
		t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{CSP: csp})))
		err := webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{CSP: mw.StrictCSP().Add(mw.ScriptSrc, "'sha256-ABC='")})).Check(t.Context())
		t.Must(be.ErrorContains(err, "CSP script-src is"))
	})
}
