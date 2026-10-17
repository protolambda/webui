package mw_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

// securityHeaderNames are the headers that SecurityHeaders may set.
var securityHeaderNames = []string{
	"Content-Security-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options",
	"Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy", "Permissions-Policy", "Cache-Control",
	"Strict-Transport-Security",
}

// securityHeaders serves one request through SecurityHeaders(cfg) and returns
// the security headers of the response, one value each.
func securityHeaders(t devtest.T, cfg mw.Headers, h http.Handler) map[string]string {
	t.Helper()
	if h == nil {
		h = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	}
	rec := httptest.NewRecorder()
	mw.SecurityHeaders(cfg)(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	out := map[string]string{}
	for _, name := range securityHeaderNames {
		values := rec.Header().Values(name)
		t.Must(be.LessOrEq(len(values), 1))
		if len(values) == 1 {
			out[name] = values[0]
		}
	}
	return out
}

func TestSecurityHeadersDefaults(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.DeepEqual(map[string]string{
		"Content-Security-Policy":      strictPolicy,
		"Referrer-Policy":              "same-origin",
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Permissions-Policy":           mw.DefaultPermissionsPolicy,
		"Cache-Control":                "no-store",
	}, securityHeaders(t, mw.Headers{}, nil)))
}

func TestSecurityHeadersConfigured(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cfg := mw.Headers{
		CSP:                       mw.StrictCSP().With(mw.FrameAncestors, mw.SourceSelf),
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "camera=()",
		CacheControl:              "private, no-cache",
		CrossOriginOpenerPolicy:   "same-origin-allow-popups",
		CrossOriginResourcePolicy: "same-site",
		HSTS:                      mw.HSTS{MaxAge: 365 * 24 * time.Hour, IncludeSubDomains: true, Preload: true},
	}
	t.Must(be.DeepEqual(map[string]string{
		"Content-Security-Policy":      strings.Replace(strictPolicy, "frame-ancestors 'none'", "frame-ancestors 'self'", 1),
		"Referrer-Policy":              "strict-origin-when-cross-origin",
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "SAMEORIGIN",
		"Cross-Origin-Opener-Policy":   "same-origin-allow-popups",
		"Cross-Origin-Resource-Policy": "same-site",
		"Permissions-Policy":           "camera=()",
		"Cache-Control":                "private, no-cache",
		"Strict-Transport-Security":    "max-age=31536000; includeSubDomains; preload",
	}, securityHeaders(t, cfg, nil)))
}

func TestSecurityHeadersFrameOptionsFollowCSP(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		csp  mw.CSP
		want string
	}{
		{"none", mw.StrictCSP(), "DENY"},
		{"self", mw.StrictCSP().With(mw.FrameAncestors, mw.SourceSelf), "SAMEORIGIN"},
		{"other origin", mw.StrictCSP().With(mw.FrameAncestors, "https://portal.example.com"), ""},
		{"self and other origin", mw.StrictCSP().With(mw.FrameAncestors, mw.SourceSelf, "https://portal.example.com"), ""},
		{"absent", mw.StrictCSP().Without(mw.FrameAncestors), "DENY"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			t.Must(be.Equal(c.want, securityHeaders(t, mw.Headers{CSP: c.csp}, nil)["X-Frame-Options"]))
		})
	}
}

func TestSecurityHeadersHandlersOverride(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Del("Cross-Origin-Resource-Policy")
		w.Header().Add("Permissions-Policy", "fullscreen=()")
	})
	got := mw.SecurityHeaders(mw.Headers{})(h)
	rec := httptest.NewRecorder()
	got.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal("public, max-age=31536000, immutable", rec.Header().Get("Cache-Control")))
	t.Must(be.Equal("", rec.Header().Get("Cross-Origin-Resource-Policy")))
	t.Must(be.DeepEqual([]string{mw.DefaultPermissionsPolicy, "fullscreen=()"}, rec.Header().Values("Permissions-Policy")))

	// What a handler did to one response leaves the next one alone.
	rec = httptest.NewRecorder()
	got.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.DeepEqual([]string{mw.DefaultPermissionsPolicy, "fullscreen=()"}, rec.Header().Values("Permissions-Policy")))
	t.Must(be.Equal(mw.DefaultPermissionsPolicy, securityHeaders(t, mw.Headers{}, nil)["Permissions-Policy"]))
}

func TestSecurityHeadersOnRejections(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := mw.Chain(http.NotFoundHandler(),
		mw.SecurityHeaders(mw.Headers{}),
		mw.HostCheck(mw.MustParseSite("https://app.example.com")),
		mw.SameOrigin(mw.OriginPolicy{}),
	)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "https://evil.example.com/", nil),
		httptest.NewRequest(http.MethodPost, "https://app.example.com/", nil),
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		t.Must(be.GreaterOrEq(rec.Code, 400))
		t.Must(be.Equal(strictPolicy, rec.Header().Get("Content-Security-Policy")))
		t.Must(be.Equal("nosniff", rec.Header().Get("X-Content-Type-Options")))
		t.Must(be.Equal("no-store", rec.Header().Get("Cache-Control")))
	}
}

func TestHSTSString(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		hsts mw.HSTS
		want string
	}{
		{mw.HSTS{}, ""},
		{mw.HSTS{MaxAge: time.Hour}, "max-age=3600"},
		{mw.HSTS{MaxAge: 1500 * time.Millisecond}, "max-age=1"},
		{mw.HSTS{MaxAge: time.Hour, IncludeSubDomains: true}, "max-age=3600; includeSubDomains"},
	}
	for _, c := range cases {
		t.Must(be.Equal(c.want, c.hsts.String()))
	}
}

func TestHSTSFor(gt *testing.T) {
	t := devtest.ParallelT(gt)
	public := mw.MustParseSite("https://app.example.com")
	other := mw.MustParseSite("https://admin.example.com:8443")
	year := mw.HSTS{MaxAge: mw.DefaultHSTSMaxAge}
	cases := []struct {
		name  string
		sites []mw.Site
		want  mw.HSTS
	}{
		{"https site", []mw.Site{public}, year},
		{"https sites", []mw.Site{public, other}, year},
		{"https IP address", []mw.Site{mw.MustParseSite("https://192.0.2.10")}, year},
		{"no sites", nil, mw.HSTS{}},
		{"zero site", []mw.Site{{}}, mw.HSTS{}},
		{"https and zero site", []mw.Site{public, {}}, mw.HSTS{}},
		{"http loopback", []mw.Site{mw.MustParseSite("http://127.0.0.1:8080")}, mw.HSTS{}},
		{"https localhost", []mw.Site{mw.MustParseSite("https://localhost:8443")}, mw.HSTS{}},
		{"https loopback name", []mw.Site{mw.MustParseSite("https://app.localhost")}, mw.HSTS{}},
		{"https IPv6 loopback", []mw.Site{mw.MustParseSite("https://[::1]")}, mw.HSTS{}},
		{"mixed", []mw.Site{public, mw.MustParseSite("http://localhost:8080")}, mw.HSTS{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			got := mw.HSTSFor(c.sites...)
			t.Must(be.Equal(c.want, got))
			t.Must(be.NoError(mw.Headers{HSTS: got}.Validate()))
		})
	}
	t.Must(be.Equal("max-age=31536000", year.String()))
	t.Must(be.Equal(365*24*time.Hour, mw.DefaultHSTSMaxAge))
	// The result may be extended into a preload policy.
	preload := year
	preload.IncludeSubDomains, preload.Preload = true, true
	t.Must(be.NoError(mw.Headers{HSTS: preload}.Validate()))
}

// HostCheck refusals never carry the HSTS of the configured sites: the
// policy would pin a name that a client chose.
func TestHostCheckRefusalDropsHSTS(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite("https://app.example.com")
	h := mw.Chain(http.NotFoundHandler(),
		mw.SecurityHeaders(mw.Headers{HSTS: mw.HSTSFor(site)}),
		mw.HostCheck(site),
	)
	for host, want := range map[string]string{
		"app.example.com":     "max-age=31536000",
		"localhost:8443":      "",
		"other.example.com":   "",
		"app.example.com:444": "",
	} {
		req := httptest.NewRequest(http.MethodGet, "https://app.example.com/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		t.Must(be.Equal(want, rec.Header().Get("Strict-Transport-Security")))
		if want == "" {
			t.Must(be.Equal(http.StatusBadRequest, rec.Code))
			t.Must(be.Len(rec.Header().Values("Strict-Transport-Security"), 0))
			// The other security headers stay.
			t.Must(be.Equal(strictPolicy, rec.Header().Get("Content-Security-Policy")))
			t.Must(be.Equal("no-store", rec.Header().Get("Cache-Control")))
		}
	}
}

func TestHeadersValidate(gt *testing.T) {
	t := devtest.ParallelT(gt)
	year := 365 * 24 * time.Hour
	valid := []mw.Headers{
		{},
		{HSTS: mw.HSTS{MaxAge: time.Second}},
		{HSTS: mw.HSTS{MaxAge: year, IncludeSubDomains: true, Preload: true}},
		{ReferrerPolicy: "no-referrer, strict-origin-when-cross-origin"},
	}
	for _, h := range valid {
		t.Must(be.NoError(h.Validate()))
		t.Must(be.NoPanic(func() { mw.SecurityHeaders(h) }))
	}
	invalid := map[string]mw.Headers{
		"newline in referrer policy": {ReferrerPolicy: "same-origin\r\nSet-Cookie: x=1"},
		"tab in cache control":       {CacheControl: "no-store\t"},
		"NUL in permissions policy":  {PermissionsPolicy: "camera=()\x00"},
		"non-ASCII COOP":             {CrossOriginOpenerPolicy: "same-orígin"},
		"DEL in CORP":                {CrossOriginResourcePolicy: "same-origin\x7f"},
		"HSTS without max-age":       {HSTS: mw.HSTS{IncludeSubDomains: true}},
		"HSTS below a second":        {HSTS: mw.HSTS{MaxAge: time.Millisecond}},
		"HSTS negative":              {HSTS: mw.HSTS{MaxAge: -time.Hour}},
		"preload without subdomains": {HSTS: mw.HSTS{MaxAge: year, Preload: true}},
		"preload too short":          {HSTS: mw.HSTS{MaxAge: year - time.Second, IncludeSubDomains: true, Preload: true}},
	}
	for name, h := range invalid {
		t.Run(name, func(t devtest.T) {
			t.Must(be.ErrorIs(h.Validate(), mw.ErrInvalidHeaders))
			t.Must(be.Panic(func() { mw.SecurityHeaders(h) }))
		})
	}
}
