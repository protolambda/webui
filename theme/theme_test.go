package theme_test

import (
	"crypto/sha512"
	"encoding/base64"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/theme"
)

func names(s *asset.Set) []string {
	var out []string
	for info := range s.Files() {
		out = append(out, info.Name)
	}
	return out
}

func TestDefaultAssets(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s, err := theme.Assets(theme.Options{})
	t.Must(be.NoError(err))
	t.Must(be.Equal(theme.DefaultPrefix, s.Prefix()))
	t.Must(be.DeepEqual([]string{
		theme.BootstrapCSS, theme.IconsCSS, theme.IconsFont, theme.ThemeCSS, theme.HTMXJS, theme.SSEJS,
	}, names(s)))
}

func TestOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := theme.MustAssets(theme.Options{
		Prefix:       "/static/ui/",
		WithoutIcons: true,
		WithoutSSE:   true,
		BootstrapJS:  true,
		WebSocket:    true,
	})
	t.Must(be.Equal("/static/ui/", s.Prefix()))
	t.Must(be.DeepEqual([]string{
		theme.BootstrapCSS, theme.ThemeCSS, theme.HTMXJS, theme.WSJS, theme.BootstrapJS,
	}, names(s)))
	t.Must(be.Panic(func() { theme.MustAssets(theme.Options{Prefix: "no-slash"}) }))
}

// TestPinnedIntegrity pins every third-party file twice, by the version in
// its name and by its expected integrity value (the one its project
// publishes, or for Bootstrap Icons, which publishes none, the value checked
// against the npm registry and jsDelivr), so that a file and its constant
// cannot be replaced together by mistake.
func TestPinnedIntegrity(gt *testing.T) {
	t := devtest.ParallelT(gt)
	pins := map[string]string{
		"bootstrap-5.3.8.min.css":        "sha384-sRIl4kxILFvY47J16cr9ZwB07vP4J8+LH7qKQnuqkuIAvNWLzeN8tE5YBujZqJLB",
		"bootstrap-5.3.8.bundle.min.js":  "sha384-FKyoEForCGlyvwx9Hj09JcYn3nv7wiPVlz7YYwJrWVcXK/BmnVDxM+D2scQbITxI",
		"bootstrap-icons-1.13.1.min.css": "sha384-CK2SzKma4jA5H/MXDUU7i1TqZlCFaD4T01vtyDFvPlD97JQyS+IsSh1nI2EFbpyk",
		"fonts/bootstrap-icons.woff2":    "sha384-xEoI56EFpIZiDZZKBZxsn3gO3u/FvXtOpHbtkMWmSdfzDw3x9XdVc3i70O9hm4SC",
		"htmx-2.0.10.min.js":             "sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V",
		"htmx-ext-sse-2.2.4.min.js":      "sha384-A986SAtodyH8eg8x8irJnYUk7i9inVQqYigD6qZ9evobksGNIXfeFvDwLSHcp31N",
		"htmx-ext-ws-2.0.4.min.js":       "sha384-1RwI/nvUSrMRuNj7hX1+27J8XDdCoSLf0EjEyF69nacuWyiJYoQ/j39RT1mSnd2G",
	}
	consts := map[string]string{
		theme.BootstrapCSS: theme.BootstrapCSSIntegrity,
		theme.BootstrapJS:  theme.BootstrapJSIntegrity,
		theme.IconsCSS:     theme.IconsCSSIntegrity,
		theme.IconsFont:    theme.IconsFontIntegrity,
		theme.HTMXJS:       theme.HTMXJSIntegrity,
		theme.SSEJS:        theme.SSEJSIntegrity,
		theme.WSJS:         theme.WSJSIntegrity,
	}
	t.Must(be.DeepEqual(pins, consts))
	for name, want := range pins {
		data, err := fs.ReadFile(theme.FS(), name)
		t.Must(be.NoError(err))
		sum := sha512.Sum384(data)
		t.Must(be.Equal(want, "sha384-"+base64.StdEncoding.EncodeToString(sum[:])))
	}
}

func TestLicenses(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, want := range map[string]string{
		"LICENSE-bootstrap":       "The MIT License (MIT)",
		"LICENSE-bootstrap-icons": "The MIT License (MIT)",
		"LICENSE-htmx":            "Zero-Clause BSD",
		"LICENSE-htmx-ext-sse":    "BSD Zero Clause License",
		"LICENSE-htmx-ext-ws":     "BSD Zero Clause License",
	} {
		data, err := fs.ReadFile(theme.FS(), name)
		t.Must(be.NoError(err))
		t.Must(be.Prefix(string(data), want))
	}
}

// TestIconsFontPath checks that the icons stylesheet loads the embedded font
// by the relative path the set serves it at.
func TestIconsFontPath(gt *testing.T) {
	t := devtest.ParallelT(gt)
	data, err := fs.ReadFile(theme.FS(), theme.IconsCSS)
	t.Must(be.NoError(err))
	urls := regexp.MustCompile(`url\("([^"?]+)`).FindAllStringSubmatch(string(data), -1)
	var got []string
	for _, m := range urls {
		got = append(got, m[1])
	}
	t.Must(be.DeepEqual([]string{"fonts/bootstrap-icons.woff2", "fonts/bootstrap-icons.woff"}, got))
	t.Must(be.True(strings.Contains(string(data), "Bootstrap Icons v"+theme.IconsVersion)))

	s := theme.MustAssets(theme.Options{})
	css := s.URL(theme.IconsCSS)
	font := css[:strings.LastIndex(css, "/")+1] + urls[0][1]
	t.Must(be.Equal(s.URL(theme.IconsFont), font))
}

// evalCall matches the ways a script evaluates a string as code: eval in any
// form (direct, indirect as (0,eval)(s), or by name as window["eval"]) and
// the Function constructor, with or without new.
var evalCall = regexp.MustCompile(`\beval\b|\bFunction\s*\(`)

// TestNoEval checks that the htmx extensions and Bootstrap's script never
// evaluate strings as code, which the content security policy forbids.
//
// htmx itself is left out on purpose: it has eval paths (hx-on handlers,
// event filters, js: and javascript: values), and webui keeps them unused
// in two ways: hx2.Config's zero value sets allowEval false, and the default
// content security policy has no 'unsafe-eval'. The check on htmx shows the
// pattern finds them, so a clean result elsewhere means something.
func TestNoEval(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, name := range []string{theme.SSEJS, theme.WSJS, theme.BootstrapJS} {
		data, err := fs.ReadFile(theme.FS(), name)
		t.Must(be.NoError(err))
		t.Mustf(be.Nil(evalCall.Find(data)), "%s evaluates strings: %q", name, evalCall.Find(data))
	}
	htmx, err := fs.ReadFile(theme.FS(), theme.HTMXJS)
	t.Must(be.NoError(err))
	t.Must(be.GreaterOrEq(len(evalCall.FindAll(htmx, -1)), 3))

	for _, c := range []struct {
		src  string
		want bool
	}{
		{`return Function(t)()`, true},
		{`new Function("event", n)`, true},
		{`(0,eval)(x)`, true},
		{`window["eval"](x)`, true},
		{`eval (x)`, true},
		{`isFunction(x); retrieval(x); evaluate(x)`, false},
	} {
		t.Mustf(be.Equal(c.want, evalCall.MatchString(c.src)), "%s", c.src)
	}
}

func TestServed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := theme.MustAssets(theme.Options{BootstrapJS: true, WebSocket: true})
	mux := http.NewServeMux()
	mux.Handle(s.Pattern(), s)
	for info := range s.Files() {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, info.URL, nil))
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal(info.ContentType, rec.Header().Get("Content-Type")))
		sum := sha512.Sum384(rec.Body.Bytes())
		t.Must(be.Equal(info.Integrity, "sha384-"+base64.StdEncoding.EncodeToString(sum[:])))
	}
	kinds := map[asset.Kind][]string{}
	for info := range s.Files() {
		kinds[info.Kind] = append(kinds[info.Kind], info.Name)
	}
	t.Must(be.True(slices.Equal([]string{theme.BootstrapCSS, theme.IconsCSS, theme.ThemeCSS}, kinds[asset.KindStylesheet])))
	t.Must(be.True(slices.Equal([]string{theme.HTMXJS, theme.SSEJS, theme.WSJS, theme.BootstrapJS}, kinds[asset.KindScript])))
}
