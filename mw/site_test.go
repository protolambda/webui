package mw_test

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

func TestParseSite(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		in, origin, host string
		https, loopback  bool
	}{
		{"https://app.example.com", "https://app.example.com", "app.example.com", true, false},
		{"HTTPS://App.Example.COM/", "https://app.example.com", "app.example.com", true, false},
		{"https://app.example.com:443", "https://app.example.com", "app.example.com", true, false},
		{"https://app.example.com:0443", "https://app.example.com", "app.example.com", true, false},
		{"https://app.example.com:8443", "https://app.example.com:8443", "app.example.com:8443", true, false},
		{"https://app.example.com:", "https://app.example.com", "app.example.com", true, false},
		{"https://192.0.2.10", "https://192.0.2.10", "192.0.2.10", true, false},
		{"https://[2001:DB8:0:0::1]:8443", "https://[2001:db8::1]:8443", "[2001:db8::1]:8443", true, false},
		{"https://xn--bcher-kva.example", "https://xn--bcher-kva.example", "xn--bcher-kva.example", true, false},
		{"http://localhost:8080", "http://localhost:8080", "localhost:8080", false, true},
		{"http://LOCALHOST", "http://localhost", "localhost", false, true},
		{"http://localhost:80/", "http://localhost", "localhost", false, true},
		{"http://app.localhost:8080", "http://app.localhost:8080", "app.localhost:8080", false, true},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080", "127.0.0.1:8080", false, true},
		{"http://127.1.2.3", "http://127.1.2.3", "127.1.2.3", false, true},
		{"http://[::1]:8080", "http://[::1]:8080", "[::1]:8080", false, true},
		{"http://[0:0:0:0:0:0:0:1]:8080", "http://[::1]:8080", "[::1]:8080", false, true},
		{"https://localhost:8443", "https://localhost:8443", "localhost:8443", true, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t devtest.T) {
			s, err := mw.ParseSite(c.in)
			t.Must(be.NoError(err))
			t.Must(be.Equal(c.origin, s.Origin()))
			t.Must(be.Equal(c.origin, s.String()))
			t.Must(be.Equal(c.host, s.Host()))
			t.Must(be.Equal(c.https, s.HTTPS()))
			t.Must(be.Equal(c.loopback, s.Loopback()))
			t.Must(be.False(s.IsZero()))
			again, err := mw.ParseSite(s.Origin())
			t.Must(be.NoError(err))
			t.Must(be.Equal(s, again))
		})
	}
}

func TestParseSiteErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := map[string]string{
		"empty":             "",
		"no scheme":         "marker.example.com",
		"protocol-relative": "//marker.example.com",
		"other scheme":      "ftp://marker.example.com",
		"mailto":            "mailto:marker@example.com",
		"opaque":            "https:marker.example.com",
		"plain http":        "http://marker.example.com",
		"plain http IP":     "http://192.0.2.10",
		"user":              "https://marker@app.example.com",
		"empty user":        "https://@marker.example.com",
		"path":              "https://marker.example.com/app",
		"query":             "https://marker.example.com?x=1",
		"empty query":       "https://marker.example.com?",
		"fragment":          "https://marker.example.com#top",
		"empty fragment":    "https://marker.example.com#",
		"large port":        "https://marker.example.com:65536",
		"zero port":         "https://marker.example.com:0",
		"underscore":        "https://marker_x.example.com",
		"non-ASCII":         "https://markér.example.com",
		"dotted I":          "https://\u0130nfo.marker.example.com", // lower-cases to "i" under Unicode rules
		"Kelvin sign":       "https://\u212Aexample.marker.com",     // lower-cases to "k"
		"long s":            "https://\u017Fhop.marker.example",
		"trailing dot":      "https://marker.example.com.",
		"empty label":       "https://marker..example.com",
		"wildcard":          "https://*.marker.example.com",
		"IPv6 zone":         "http://[fe80::1%25marker]:8080",
		"IPv4-mapped":       "http://[::ffff:127.0.0.1]:8080",
		"bracketed IPv4":    "http://[127.0.0.1]:8080",
		"unclosed bracket":  "http://[::1:8080",
		"short IPv4":        "http://127.1",
		"hex IPv4":          "http://0x7f.0.0.1",
		"octal IPv4":        "http://0177.0.0.1",
		"leading zero IPv4": "http://127.000.000.001",
		"too long":          "https://" + strings.Repeat("marker.", 300) + "example.com",
		"space":             "https://marker example.com",
	}
	for name, in := range cases {
		t.Run(name, func(t devtest.T) {
			s, err := mw.ParseSite(in)
			t.Must(be.ErrorIs(err, mw.ErrInvalidSite))
			t.Must(be.True(s.IsZero()))
			t.Mustf(be.False(strings.Contains(err.Error(), "marker")), "errors do not quote the input")
		})
	}
	t.Must(be.Panic(func() { mw.MustParseSite("http://marker.example.com") }))
}

func TestSiteText(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var cfg struct{ Site mw.Site }
	t.Must(be.NoError(json.Unmarshal([]byte(`{"Site":"https://App.example.com:443/"}`), &cfg)))
	t.Must(be.Equal(mw.MustParseSite("https://app.example.com"), cfg.Site))
	out, err := json.Marshal(cfg)
	t.Must(be.NoError(err))
	t.Must(be.Equal(`{"Site":"https://app.example.com"}`, string(out)))

	t.Must(be.ErrorIs(json.Unmarshal([]byte(`{"Site":"http://app.example.com"}`), &cfg), mw.ErrInvalidSite))
	t.Mustf(be.Equal(mw.MustParseSite("https://app.example.com"), cfg.Site), "a failed parse keeps the value")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var site mw.Site
	fs.TextVar(&site, "site", mw.MustParseSite("http://localhost:8080"), "site URL")
	t.Must(be.NoError(fs.Parse([]string{"-site", "https://app.example.com:8443"})))
	t.Must(be.Equal("https://app.example.com:8443", site.Origin()))
}

func TestIsLoopbackHost(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := map[string]bool{
		"localhost":           true,
		"LocalHost":           true,
		"localhost.":          true,
		"app.localhost":       true,
		"a.b.localhost":       true,
		"127.0.0.1":           true,
		"127.255.255.254":     true,
		"::1":                 true,
		"[::1]":               true,
		"::ffff:127.0.0.1":    true,
		"":                    false,
		"localhost:8080":      false,
		".localhost":          false,
		"a..localhost":        false,
		"a_b.localhost":       false,
		"localhost.example":   false,
		"notlocalhost":        false,
		"128.0.0.1":           false,
		"0.0.0.0":             false,
		"::":                  false,
		"[::1":                false,
		"[localhost]":         false,
		"[127.0.0.1]":         false,
		"[::ffff:127.0.0.1]":  true,
		"fe80::1%lo":          false,
		"::1%lo":              false,
		"127.1":               false,
		"192.0.2.1":           false,
		"2001:db8::1":         false,
		"localhost.localdom":  false,
		"\u0130nfo.localhost": false,
		"LOCALHOST":           true,
	}
	for host, want := range cases {
		t.Run(host, func(t devtest.T) {
			t.Must(be.Equal(want, mw.IsLoopbackHost(host)))
		})
	}
}

func TestSiteMatchHost(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		site  string
		hosts map[string]bool
	}{
		{"https://app.example.com", map[string]bool{
			"app.example.com":       true,
			"APP.Example.COM":       true,
			"app.example.com.":      true,
			"app.example.com:443":   true,
			"app.example.com.:443":  true,
			"app.example.com:8443":  false,
			"app.example.com:80":    false,
			"app.example.com..":     false,
			"app.example.com.evil":  false,
			"evil.app.example.com":  false,
			"app.example.co":        false,
			"app.example.com:443:1": false,
			"192.0.2.1":             false,
			"[::1]":                 false,
			"":                      false,
		}},
		{"https://app.example.com:8443", map[string]bool{
			"app.example.com:8443": true,
			"app.example.com":      false,
			"app.example.com:443":  false,
		}},
		{"http://127.0.0.1:8080", map[string]bool{
			"127.0.0.1:8080": true,
			"127.0.0.1":      false,
			"127.0.0.1:80":   false,
			"localhost:8080": false,
		}},
		{"http://[::1]:8080", map[string]bool{
			"[::1]:8080": true,
			"[::1]":      false,
			"::1:8080":   false,
			"[::1]8080":  false,
			"[::1:8080":  false,
		}},
		{"http://localhost", map[string]bool{
			"localhost":      true,
			"localhost:80":   true,
			"localhost:":     true,
			"localhost:8080": false,
			"localhost:443":  false,
		}},
	}
	for _, c := range cases {
		site := mw.MustParseSite(c.site)
		for host, want := range c.hosts {
			t.Mustf(be.Equal(want, site.MatchHost(host)), "site %s, host %q", c.site, host)
		}
	}
	t.Must(be.False(mw.Site{}.MatchHost("")))
	t.Must(be.False(mw.Site{}.MatchHost("app.example.com")))
	t.Must(be.False(mw.MustParseSite("https://app.example.com").MatchHost(strings.Repeat("a", 301))))

	// No Unicode case folding: U+017F folds to "s", U+212A to "k".
	t.Must(be.False(mw.MustParseSite("https://shop.example").MatchHost("\u017Fhop.example")))
	t.Must(be.False(mw.MustParseSite("https://kexample.com").MatchHost("\u212Aexample.com")))
	t.Must(be.True(mw.MustParseSite("https://kexample.com").MatchHost("KEXAMPLE.com")))
	t.Must(be.True(mw.MustParseSite("https://[2001:db8::a]").MatchHost("[2001:DB8::A]")))
}

func TestHostCheck(gt *testing.T) {
	t := devtest.ParallelT(gt)
	public := mw.MustParseSite("https://app.example.com")
	local := mw.MustParseSite("http://localhost:8080")
	var seen mw.Site
	var calls int
	h := mw.HostCheck(public, local)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls++
		seen, _ = mw.SiteFrom(r.Context())
	}))
	cases := []struct {
		host string
		site mw.Site
	}{
		{"app.example.com", public},
		{"App.Example.com.:443", public},
		{"localhost:8080", local},
		{"evil.example.com", mw.Site{}},
		{"app.example.com.evil.example.com", mw.Site{}},
		{"localhost", mw.Site{}},
		{"127.0.0.1:8080", mw.Site{}},
		{"", mw.Site{}},
	}
	for _, c := range cases {
		calls, seen = 0, mw.Site{}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if c.site.IsZero() {
			t.Must(be.Equal(http.StatusBadRequest, rec.Code))
			t.Must(be.Equal("invalid host\n", rec.Body.String()))
			t.Must(be.Equal(0, calls))
			continue
		}
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal(1, calls))
		t.Must(be.Equal(c.site, seen))
	}
}

// A refusal is a fixed plain-text answer that quotes nothing of the request,
// logged at debug level without the Host.
func TestHostCheckRefusal(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelDebug)
	h := mw.HostCheck(mw.MustParseSite("https://app.example.com"))(http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodPost, "https://app.example.com/notes?q=secret", strings.NewReader("x=1"))
	req = req.WithContext(mw.WithLogger(req.Context(), logger))
	req.Host = "<b>evil.example</b>"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	t.Must(be.Equal(http.StatusBadRequest, rec.Code))
	t.Must(be.Equal("invalid host\n", rec.Body.String()))
	t.Must(be.Equal("text/plain; charset=utf-8", rec.Header().Get("Content-Type")))
	t.Must(be.Equal("nosniff", rec.Header().Get("X-Content-Type-Options")))

	records := sink.Records()
	t.Must(be.Len(records, 1))
	t.Must(be.Equal(slog.LevelDebug, records[0].Level))
	t.Must(be.Equal("refused a request for an unknown host", records[0].Message))
	t.Must(be.DeepEqual(map[string]string{}, attrs(records[0])))
}

func TestHostCheckPanicsOnMisuse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Panic(func() { mw.HostCheck() }))
	t.Must(be.Panic(func() { mw.HostCheck(mw.MustParseSite("https://app.example.com"), mw.Site{}) }))
}

func TestSiteContext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	_, ok := mw.SiteFrom(context.Background())
	t.Must(be.False(ok))
	_, ok = mw.SiteFrom(mw.WithSite(context.Background(), mw.Site{}))
	t.Must(be.False(ok))
	site := mw.MustParseSite("https://app.example.com")
	got, ok := mw.SiteFrom(mw.WithSite(context.Background(), site))
	t.Must(be.True(ok))
	t.Must(be.Equal(site, got))
}
