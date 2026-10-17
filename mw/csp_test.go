package mw_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

const strictPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"

func TestStrictCSP(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(strictPolicy, mw.StrictCSP().String()))
	t.Must(be.False(mw.StrictCSP().IsZero()))
	t.Must(be.True(mw.CSP{}.IsZero()))
	t.Must(be.Equal("", mw.CSP{}.String()))
}

func TestCSPWith(gt *testing.T) {
	t := devtest.ParallelT(gt)
	base := mw.StrictCSP()
	scratch, err := mw.ParseCSP("default-src 'self'")
	t.Must(be.NoError(err))
	cases := []struct {
		name string
		got  mw.CSP
		want string
	}{
		{"replaces in place", base.With(mw.ImgSrc, mw.SourceSelf),
			strings.Replace(strictPolicy, "img-src 'self' data:", "img-src 'self'", 1)},
		{"appends new directives", base.With(mw.ObjectSrc, mw.SourceNone),
			strictPolicy + "; object-src 'none'"},
		{"lower-cases names", base.With("Media-SRC", "https://media.example.com"),
			strictPolicy + "; media-src https://media.example.com"},
		{"drops duplicates", scratch.With(mw.ConnectSrc, mw.SourceSelf, "wss://x.example.com", mw.SourceSelf),
			"default-src 'self'; connect-src 'self' wss://x.example.com"},
		{"drops many duplicates", scratch.With(mw.ImgSrc, strings.Fields(strings.Repeat("'self' data: ", 20))...),
			"default-src 'self'; img-src 'self' data:"},
		{"empty source list is none", scratch.With(mw.ScriptSrc), "default-src 'self'; script-src 'none'"},
		{"value-less directive", scratch.With(mw.UpgradeInsecureRequests), "default-src 'self'; upgrade-insecure-requests"},
		{"token directive", scratch.With(mw.Sandbox, "allow-forms", "allow-scripts"),
			"default-src 'self'; sandbox allow-forms allow-scripts"},
		{"trusted types", scratch.With(mw.RequireTrustedTypesFor, "'script'"),
			"default-src 'self'; require-trusted-types-for 'script'"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			t.Must(be.Equal(c.want, c.got.String()))
		})
	}
	t.Must(be.Equal(strictPolicy, base.String()))
}

func TestCSPAdd(gt *testing.T) {
	t := devtest.ParallelT(gt)
	base := mw.StrictCSP()
	t.Must(be.Equal(
		strings.Replace(strictPolicy, "form-action 'self'", "form-action 'self' https://app.example.com", 1),
		base.Add(mw.FormAction, "https://app.example.com", mw.SourceSelf).String()))
	t.Mustf(be.Equal(
		strings.Replace(strictPolicy, "default-src 'none'", "default-src 'self'", 1),
		base.Add(mw.DefaultSrc, mw.SourceSelf).String()), "adding to 'none' replaces it")
	t.Must(be.Equal(strictPolicy+"; worker-src 'self'", base.Add(mw.WorkerSrc, mw.SourceSelf).String()))
	t.Mustf(be.Equal(strictPolicy, base.Add(mw.FrameAncestors).String()), "adding nothing keeps 'none'")
	t.Mustf(be.Panic(func() { base.Add(mw.ScriptSrc, mw.SourceNone) }), "'none' is not alone")
	t.Must(be.Equal(strictPolicy, base.String()))
}

// The zero CSP stands for StrictCSP also when a policy is derived from it, so
// that widening the default of a zero Headers keeps the strict baseline.
func TestCSPZeroDerivesFromStrict(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var zero mw.CSP
	t.Must(be.Equal(mw.StrictCSP().Add(mw.ConnectSrc, "wss://live.example.com").String(),
		zero.Add(mw.ConnectSrc, "wss://live.example.com").String()))
	t.Must(be.Equal(mw.StrictCSP().With(mw.ImgSrc, mw.SourceSelf).String(), zero.With(mw.ImgSrc, mw.SourceSelf).String()))
	t.Must(be.Equal(strings.Replace(strictPolicy, "font-src 'self'; ", "", 1), zero.Without(mw.FontSrc).String()))
	t.Mustf(be.True(zero.IsZero()), "the receiver is unchanged")
	t.Must(be.Equal("", zero.String()))
	_, ok := zero.Values(mw.DefaultSrc)
	t.Mustf(be.False(ok), "Values describes the value itself")

	var cfg mw.Headers
	cfg.CSP = cfg.CSP.Add(mw.ConnectSrc, "wss://live.example.com")
	rec := httptest.NewRecorder()
	mw.SecurityHeaders(cfg)(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal(
		strings.Replace(strictPolicy, "connect-src 'self'", "connect-src 'self' wss://live.example.com", 1),
		rec.Header().Get("Content-Security-Policy")))

	rec = httptest.NewRecorder()
	mw.SecurityHeaders(mw.Headers{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mw.SetCSP(w, mw.CSP{}.Add(mw.FormAction, "https://partner.example.com"))
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal(
		strings.Replace(strictPolicy, "form-action 'self'", "form-action 'self' https://partner.example.com", 1),
		rec.Header().Get("Content-Security-Policy")))
}

func TestCSPWithout(gt *testing.T) {
	t := devtest.ParallelT(gt)
	base := mw.StrictCSP()
	got := base.Without("FONT-src").Without(mw.Sandbox)
	t.Must(be.Equal(strings.Replace(strictPolicy, "font-src 'self'; ", "", 1), got.String()))
	_, ok := got.Values(mw.FontSrc)
	t.Must(be.False(ok))
	t.Must(be.Equal(strictPolicy, base.String()))
}

func TestCSPValuesAreCopies(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p := mw.StrictCSP()
	img, ok := p.Values(mw.ImgSrc)
	t.Must(be.True(ok))
	t.Must(be.DeepEqual([]string{mw.SourceSelf, mw.SourceData}, img))
	img[0] = "*"
	for d, v := range p.All() {
		if d == mw.ImgSrc {
			v[1] = "*"
		}
	}
	t.Must(be.Equal(strictPolicy, p.String()))
	none, ok := mw.StrictCSP().With(mw.UpgradeInsecureRequests).Values(mw.UpgradeInsecureRequests)
	t.Must(be.True(ok))
	t.Must(be.Len(none, 0))

	var names []string
	for d := range p.All() {
		names = append(names, string(d))
		if d == mw.ImgSrc {
			break
		}
	}
	t.Must(be.Equal("default-src script-src style-src img-src", strings.Join(names, " ")))
}

func TestCSPConcurrentDerivation(gt *testing.T) {
	t := devtest.ParallelT(gt)
	base := mw.StrictCSP().With(mw.ConnectSrc, mw.SourceSelf)
	var wg sync.WaitGroup
	results := make([]string, 16)
	for i := range results {
		wg.Go(func() {
			results[i] = base.Add(mw.ConnectSrc, "https://x.example.com").Add(mw.ImgSrc, mw.SourceBlob).String()
		})
	}
	wg.Wait()
	for _, r := range results {
		t.Must(be.Equal(results[0], r))
	}
	t.Must(be.Equal(strictPolicy, base.String()))
}

func TestCSPValidSources(gt *testing.T) {
	t := devtest.ParallelT(gt)
	valid := []string{
		"'self'", "'SELF'", "'none'", "'unsafe-inline'", "'strict-dynamic'", "'wasm-unsafe-eval'",
		"'nonce-abc123+/='", "'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='", "'sha384-a_b-c'",
		"https:", "data:", "blob:", "wss:", "web+app:",
		"example.com", "*.example.com", "https://example.com", "https://example.com:8443",
		"https://example.com:*", "https://example.com/path/to/", "https://example.com:8443/a%20b", "*",
		"http://localhost:8080", "EXAMPLE.com",
	}
	for _, v := range valid {
		t.Run(v, func(t devtest.T) {
			t.Must(be.NoPanic(func() { mw.CSP{}.With(mw.ScriptSrc, v) }))
		})
	}
}

func TestCSPInvalidValuesPanic(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name   string
		d      mw.Directive
		values []string
	}{
		{"directive injection", mw.ImgSrc, []string{"'self';script-src"}},
		{"second policy", mw.ImgSrc, []string{"a.example.com,b.example.com"}},
		{"empty", mw.ImgSrc, []string{""}},
		{"space", mw.ImgSrc, []string{"a b"}},
		{"newline", mw.ImgSrc, []string{"a\nb"}},
		{"non-ASCII", mw.ImgSrc, []string{"bücher.example"}},
		{"unknown keyword", mw.ScriptSrc, []string{"'unsafe-everything'"}},
		{"unquoted keyword", mw.ScriptSrc, []string{"self"}},
		{"empty nonce", mw.ScriptSrc, []string{"'nonce-'"}},
		{"bad nonce", mw.ScriptSrc, []string{"'nonce-a*b'"}},
		{"unterminated hash", mw.ScriptSrc, []string{"'sha256-abc"}},
		{"too much padding", mw.ScriptSrc, []string{"'sha256-abc==='"}},
		{"unknown hash", mw.ScriptSrc, []string{"'md5-abc'"}},
		{"IPv6 literal", mw.ConnectSrc, []string{"http://[::1]:8080"}},
		{"underscore", mw.ConnectSrc, []string{"https://exa_mple.com"}},
		{"empty label", mw.ConnectSrc, []string{"https://a..example.com"}},
		{"wildcard only label", mw.ConnectSrc, []string{"*."}},
		{"inner wildcard", mw.ConnectSrc, []string{"https://a.*.example.com"}},
		{"bad port", mw.ConnectSrc, []string{"https://example.com:99999"}},
		{"empty port", mw.ConnectSrc, []string{"https://example.com:"}},
		{"port letters", mw.ConnectSrc, []string{"https://example.com:http"}},
		{"protocol-relative", mw.ConnectSrc, []string{"//example.com"}},
		{"double slash path", mw.ConnectSrc, []string{"https://example.com//x"}},
		{"bad scheme", mw.ConnectSrc, []string{"1http:"}},
		{"none and more", mw.ImgSrc, []string{mw.SourceNone, mw.SourceSelf}},
		{"frame-ancestors keyword", mw.FrameAncestors, []string{"'unsafe-inline'"}},
		{"frame-ancestors nonce", mw.FrameAncestors, []string{"'nonce-abc'"}},
		{"token with semicolon", mw.Sandbox, []string{"allow-forms;"}},
		{"empty token", mw.Sandbox, []string{""}},

		{"script-src-elem unquoted keyword", mw.ScriptSrcElem, []string{"self"}},
		{"script-src-attr unquoted keyword", mw.ScriptSrcAttr, []string{"unsafe-inline"}},
		{"style-src-elem unquoted keyword", mw.StyleSrcElem, []string{"self"}},
		{"style-src-attr unquoted keyword", mw.StyleSrcAttr, []string{"unsafe-inline"}},
		{"script-src-elem none and more", mw.ScriptSrcElem, []string{mw.SourceNone, mw.SourceSelf}},
		{"style-src-attr none and more", mw.StyleSrcAttr, []string{mw.SourceNone, mw.SourceSelf}},
		{"script-src-elem bad hash", mw.ScriptSrcElem, []string{"'sha256-??'"}},
		{"style-src-elem unknown keyword", mw.StyleSrcElem, []string{"'bogus'"}},
		{"style-src-elem invalid host", mw.StyleSrcElem, []string{"evil^^"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			t.Must(be.Panic(func() { mw.CSP{}.With(c.d, c.values...) }))
			t.Must(be.Panic(func() { mw.StrictCSP().Add(c.d, c.values...) }))
		})
	}
	// U+0130 and U+212A lower-case to "i" and "k" under Unicode rules.
	for _, name := range []mw.Directive{"img src", "", "img-src;", "img_src", "imgé", "scr\u0130pt-src", "wor\u212Aer-src"} {
		t.Must(be.Panic(func() { mw.CSP{}.With(name, mw.SourceSelf) }))
		t.Must(be.Panic(func() { mw.CSP{}.Add(name, mw.SourceSelf) }))
	}
}

// The CSP Level 3 -elem and -attr directives are source lists like the
// directives they refine.
func TestCSPElemAttrSourceLists(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, d := range []mw.Directive{mw.ScriptSrcElem, mw.ScriptSrcAttr, mw.StyleSrcElem, mw.StyleSrcAttr} {
		t.Run(string(d), func(t devtest.T) {
			v, ok := mw.StrictCSP().With(d).Values(d)
			t.Must(be.True(ok))
			t.Mustf(be.DeepEqual([]string{mw.SourceNone}, v), "an empty source list is 'none'")
			v, _ = mw.StrictCSP().With(d).Add(d, mw.SourceSelf).Values(d)
			t.Mustf(be.DeepEqual([]string{mw.SourceSelf}, v), "adding replaces 'none'")

			p, err := mw.ParseCSP(string(d) + " 'self' 'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='")
			t.Must(be.NoError(err))
			t.Must(be.Equal(string(d)+" 'self' 'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='", p.String()))
			p, err = mw.ParseCSP(string(d))
			t.Must(be.NoError(err))
			t.Must(be.Equal(string(d)+" 'none'", p.String()))
			for _, bad := range []string{" self", " 'none' 'self'", " 'sha256-??'", " 'marker'"} {
				_, err := mw.ParseCSP(string(d) + bad)
				t.Must(be.ErrorIs(err, mw.ErrInvalidCSP))
				t.Must(be.False(strings.Contains(err.Error(), "marker")))
			}
		})
	}
}

func TestCSPPanicValueIsError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	defer func() {
		err, ok := recover().(error)
		t.Must(be.True(ok))
		t.Must(be.ErrorIs(err, mw.ErrInvalidCSP))
		t.Must(be.False(strings.Contains(err.Error(), "marker")))
	}()
	mw.CSP{}.With(mw.ImgSrc, "marker;")
}

func TestParseCSP(gt *testing.T) {
	t := devtest.ParallelT(gt)
	p, err := mw.ParseCSP(strictPolicy)
	t.Must(be.NoError(err))
	t.Must(be.Equal(strictPolicy, p.String()))

	p, err = mw.ParseCSP(" Default-SRC\t'self' ;; img-src  'self'\ndata: ; upgrade-insecure-requests;")
	t.Must(be.NoError(err))
	t.Must(be.Equal("default-src 'self'; img-src 'self' data:; upgrade-insecure-requests", p.String()))

	p, err = mw.ParseCSP("")
	t.Must(be.NoError(err))
	t.Must(be.True(p.IsZero()))

	p, err = mw.ParseCSP("script-src")
	t.Must(be.NoError(err))
	t.Must(be.Equal("script-src 'none'", p.String()))
}

func TestParseCSPErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := map[string]string{
		"repeated directive": "img-src 'self'; IMG-SRC data:",
		"second policy":      "img-src 'self', script-src marker.example.com",
		"invalid name":       "img_src marker",
		"invalid source":     "img-src 'marker'",
		"none with more":     "img-src 'none' marker.example.com",
		"non-ASCII":          "img-src markér.example.com",
		"control character":  "img-src marker\x00",
		"too long":           "img-src " + strings.Repeat("marker ", 10000),
		"elem unquoted":      "script-src-elem marker.example.com self",
		"attr hash":          "style-src-attr 'sha256-??' marker.example.com",
		"repeated late":      manyDirectives(maxCSP-32) + "; a",
		"none after many":    "img-src " + manySources(maxCSP-64) + " 'none'",
		"dotted I name":      "scr\u0130pt-src 'self' marker.example.com",
		"Kelvin name":        "wor\u212Aer-src 'self' marker.example.com",
	}
	for name, input := range cases {
		t.Run(name, func(t devtest.T) {
			p, err := mw.ParseCSP(input)
			t.Must(be.ErrorIs(err, mw.ErrInvalidCSP))
			t.Must(be.True(p.IsZero()))
			t.Mustf(be.False(strings.Contains(err.Error(), "marker")), "errors do not quote the input")
		})
	}
}

// maxCSP is the input bound of ParseCSP.
const maxCSP = 64 << 10

// names yields distinct short tokens of lower-case letters, the numbers 0, 1,
// 2, ... in base 26 with the digits a to z: a, b, ..., z, ba, bb, ...
func names(yield func(string) bool) {
	for n := 0; ; n++ {
		s := strconv.FormatInt(int64(n), 26)
		b := []byte(s)
		for i, c := range b {
			if c <= '9' {
				b[i] = 'a' + (c - '0')
			} else {
				b[i] = 'a' + 10 + (c - 'a')
			}
		}
		if !yield(string(b)) {
			return
		}
	}
}

// manyDirectives returns a policy of distinct value-less directives of about
// size bytes.
func manyDirectives(size int) string {
	var b strings.Builder
	for n := range names {
		if b.Len()+len(n)+1 > size {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(';')
		}
		b.WriteString(n)
	}
	return b.String()
}

// manySources returns distinct host sources, separated by spaces, of about
// size bytes.
func manySources(size int) string {
	var b strings.Builder
	for n := range names {
		if b.Len()+len(n)+1 > size {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(n)
	}
	return b.String()
}

// TestParseCSPCostIsLinear checks that the worst cases of ParseCSP, many
// distinct directives and many distinct sources in one directive, cost about
// as much per byte at the input bound as at a sixteenth of it. A quadratic
// duplicate check costs 16 times more per byte at the bound (256 times as
// much in total); the check allows 6.
func TestParseCSPCostIsLinear(gt *testing.T) {
	if testing.Short() {
		gt.Skip("measures time")
	}
	t := devtest.SerialT(gt)
	inputs := map[string]func(size int) string{
		"directives": manyDirectives,
		"sources":    func(size int) string { return "img-src " + manySources(size-len("img-src ")) },
	}
	for name, input := range inputs {
		t.Run(name, func(t devtest.T) {
			small, large := input(maxCSP/16), input(maxCSP)
			cost := func(s string) time.Duration {
				best := time.Duration(1<<63 - 1)
				for range 5 {
					start := time.Now()
					_, err := mw.ParseCSP(s)
					best = min(best, time.Since(start))
					t.Must(be.NoError(err))
				}
				return best
			}
			smallCost, largeCost := cost(small), cost(large)
			ratio := float64(largeCost) / float64(max(smallCost, time.Microsecond)) / (float64(len(large)) / float64(len(small)))
			t.Mustf(be.True(ratio < 6), "per-byte cost at %d bytes is %.1f times that at %d bytes", len(large), ratio, len(small))
		})
	}
}

func TestHostSource(gt *testing.T) {
	t := devtest.ParallelT(gt)
	valid := map[string]string{
		"https://App.Example.com/callback?state=x#frag": "https://app.example.com",
		"https://app.example.com:443/":                  "https://app.example.com",
		"http://app.example.com:80":                     "http://app.example.com",
		"https://app.example.com:8443/x":                "https://app.example.com:8443",
		"https://app.example.com:08443":                 "https://app.example.com:8443",
		"wss://live.example.com:443/stream":             "wss://live.example.com",
		"ws://localhost:3000":                           "ws://localhost:3000",
		"http://localhost:3000/cb":                      "http://localhost:3000",
		"https://xn--bcher-kva.example":                 "https://xn--bcher-kva.example",
		"https://app.0xbox":                             "https://app.0xbox",
	}
	for in, want := range valid {
		t.Run(in, func(t devtest.T) {
			got, err := mw.HostSource(in)
			t.Must(be.NoError(err))
			t.Must(be.Equal(want, got))
			t.Must(be.NoPanic(func() { mw.StrictCSP().Add(mw.FormAction, got) }))
		})
	}
	invalid := map[string]string{
		"empty":            "",
		"relative":         "/marker",
		"protocol-less":    "//marker.example.com",
		"other scheme":     "ftp://marker.example.com",
		"script URL":       "javascript:marker()",
		"opaque":           "https:marker.example.com",
		"user information": "https://marker:secret@app.example.com",
		"IPv4 literal":     "https://127.0.0.1/marker",
		"IPv6 literal":     "http://[::1]:8080/marker",
		"IPv6 zone":        "http://[fe80::1%25eth0]/marker",
		"underscore":       "https://marker_x.example.com",
		"non-ASCII":        "https://bücher.example/marker",
		"dotted I":         "https://\u0130nfo.example.com/marker",
		"Kelvin sign":      "https://\u212Aexample.com/marker",
		"long s":           "https://\u017Fhop.example/marker",
		"empty label":      "https://a..example.com/marker",
		"trailing dot":     "https://example.com./marker",
		"numeric last":     "https://1.2.3/marker",
		"hex last":         "https://app.0x7f/marker",
		"zero port":        "https://example.com:0/marker",
		"large port":       "https://example.com:65536/marker",
		"long label":       "https://" + strings.Repeat("a", 64) + ".example.com/marker",
		"too long":         "https://example.com/" + strings.Repeat("marker", 1000),
		"control":          "https://example.com/\x7fmarker",
	}
	for name, in := range invalid {
		t.Run(name, func(t devtest.T) {
			got, err := mw.HostSource(in)
			t.Must(be.ErrorIs(err, mw.ErrInvalidCSP))
			t.Must(be.Equal("", got))
			t.Mustf(be.False(strings.Contains(err.Error(), "marker")), "errors do not quote the input")
		})
	}
}

func TestSetCSP(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name         string
		policy       mw.CSP
		wantCSP      string
		frameOptions []string
	}{
		{"zero is strict", mw.CSP{}, strictPolicy, []string{"DENY"}},
		{"widened form-action", mw.StrictCSP().Add(mw.FormAction, "https://other.example.com"),
			strings.Replace(strictPolicy, "form-action 'self'", "form-action 'self' https://other.example.com", 1),
			[]string{"DENY"}},
		{"same-origin framing", mw.StrictCSP().With(mw.FrameAncestors, mw.SourceSelf),
			strings.Replace(strictPolicy, "frame-ancestors 'none'", "frame-ancestors 'self'", 1),
			[]string{"SAMEORIGIN"}},
		{"framing by another origin", mw.StrictCSP().With(mw.FrameAncestors, "https://portal.example.com"),
			strings.Replace(strictPolicy, "frame-ancestors 'none'", "frame-ancestors https://portal.example.com", 1),
			nil},
		{"no frame-ancestors", mustParseCSP("default-src 'self'"), "default-src 'self'", []string{"DENY"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			h := mw.SecurityHeaders(mw.Headers{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mw.SetCSP(w, c.policy)
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			t.Must(be.DeepEqual([]string{c.wantCSP}, rec.Header().Values("Content-Security-Policy")))
			t.Must(be.DeepEqual(c.frameOptions, rec.Header().Values("X-Frame-Options")))
		})
	}
}

func mustParseCSP(s string) mw.CSP {
	p, err := mw.ParseCSP(s)
	if err != nil {
		panic(err)
	}
	return p
}

func TestErrInvalidCSPIsDistinct(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.False(errors.Is(mw.ErrInvalidCSP, mw.ErrInvalidSite)))
}
