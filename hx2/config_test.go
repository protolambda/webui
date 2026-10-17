package hx2_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
)

// renderNode renders an element without indentation.
func renderNode(ctx context.Context, n elem.Node) (string, error) {
	var b strings.Builder
	err := core.Render(ctx, n, &b)
	return b.String(), err
}

// configValues decodes the JSON of c into a map, failing the test on error.
func configValues(t devtest.T, c hx2.Config) map[string]any {
	t.Helper()
	data, err := c.JSON()
	t.Must(be.NoError(err))
	var m map[string]any
	t.Must(be.NoError(json.Unmarshal([]byte(data), &m)))
	return m
}

const zeroConfigJSON = `{"allowEval":false,"allowScriptTags":false,` +
	`"attributesToSettle":["class","width","height"],"historyCacheSize":0,` +
	`"historyRestoreAsHxRequest":false,"includeIndicatorStyles":false,` +
	`"methodsThatUseUrlParams":["get"],"refreshOnHistoryMiss":true,` +
	`"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"422","swap":true},` +
	`{"code":"[45]..","swap":true,"error":true},{"code":"...","swap":false}],"selfRequestsOnly":true}`

// The zero value writes every security-relevant key explicitly, with the
// secure values, in sorted key order.
func TestConfigZeroValue(gt *testing.T) {
	t := devtest.ParallelT(gt)
	data, err := hx2.Config{}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Equal(zeroConfigJSON, data))

	m := configValues(t, hx2.Config{})
	t.Must(be.Equal(false, m["includeIndicatorStyles"].(bool)))
	t.Must(be.Equal(false, m["allowEval"].(bool)))
	t.Must(be.Equal(false, m["allowScriptTags"].(bool)))
	t.Must(be.Equal(0.0, m["historyCacheSize"].(float64)))
	t.Must(be.Equal(true, m["refreshOnHistoryMiss"].(bool)))
	t.Must(be.Equal(false, m["historyRestoreAsHxRequest"].(bool)))
	t.Must(be.Equal(true, m["selfRequestsOnly"].(bool)))
	t.Must(be.DeepEqual[any]([]any{"get"}, m["methodsThatUseUrlParams"]))
	t.Must(be.DeepEqual[any]([]any{"class", "width", "height"}, m["attributesToSettle"]))
	_, ok := m["defaultSwapStyle"]
	t.Must(be.False(ok))
	t.Must(be.Equal(10, len(m)))
}

// A 422 answer to a form with problems swaps without the error flag, so
// htmx does not log it as an error; every other 4xx and 5xx status swaps
// as an error. As in htmx, the first rule whose code, a regular expression
// with the same meaning in Go and JavaScript, matches the status wins.
func TestDefaultResponseHandlingStatuses(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rules := hx2.DefaultResponseHandling()
	match := func(status string) hx2.ResponseRule {
		for _, r := range rules {
			if regexp.MustCompile(r.Code).MatchString(status) {
				return r
			}
		}
		t.Fatalf("no rule for %s", status)
		return hx2.ResponseRule{}
	}
	cases := []struct {
		status        string
		swap, isError bool
	}{
		{"200", true, false},
		{"201", true, false},
		{"204", false, false},
		{"303", true, false},
		{"400", true, true},
		{"403", true, true},
		{"404", true, true},
		{"409", true, true},
		{"421", true, true},
		{"422", true, false},
		{"423", true, true},
		{"429", true, true},
		{"500", true, true},
		{"503", true, true},
		{"100", false, false},
		{"600", false, false},
	}
	for _, c := range cases {
		r := match(c.status)
		t.Must(be.Annotate(be.Equal(c.swap, r.Swap), "status %s swap", c.status))
		t.Must(be.Annotate(be.Equal(c.isError, r.Error), "status %s error", c.status))
	}
}

// Every field flips its key, and only its key.
func TestConfigFields(gt *testing.T) {
	t := devtest.ParallelT(gt)
	zero := configValues(t, hx2.Config{})
	cases := []struct {
		name string
		cfg  hx2.Config
		key  string
		want any
	}{
		{"IndicatorStyles", hx2.Config{IndicatorStyles: true}, "includeIndicatorStyles", true},
		{"AllowEval", hx2.Config{AllowEval: true}, "allowEval", true},
		{"AllowScriptTags", hx2.Config{AllowScriptTags: true}, "allowScriptTags", true},
		{"HistoryCacheSize", hx2.Config{HistoryCacheSize: 10}, "historyCacheSize", 10.0},
		{"NoRefreshOnHistoryMiss", hx2.Config{NoRefreshOnHistoryMiss: true}, "refreshOnHistoryMiss", false},
		{"HistoryRestoreAsHXRequest", hx2.Config{HistoryRestoreAsHXRequest: true}, "historyRestoreAsHxRequest", true},
		{"AllowCrossOrigin", hx2.Config{AllowCrossOrigin: true}, "selfRequestsOnly", false},
		{"DeleteURLParams", hx2.Config{DeleteURLParams: true}, "methodsThatUseUrlParams", []any{"get", "delete"}},
		{"AttributesToSettle", hx2.Config{AttributesToSettle: []string{"class", "style"}}, "attributesToSettle", []any{"class", "style"}},
		{"DefaultSwapStyle", hx2.Config{DefaultSwapStyle: hx2.SwapOuterHTML}, "defaultSwapStyle", "outerHTML"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			got := configValues(t, c.cfg)
			t.Must(be.DeepEqual(c.want, got[c.key]))
			for k, v := range zero {
				if k != c.key {
					t.Must(be.Annotate(be.DeepEqual(v, got[k]), "key %s", k))
				}
			}
		})
	}
}

func TestConfigResponseHandling(gt *testing.T) {
	t := devtest.ParallelT(gt)

	// NoErrorSwap gives htmx's own default rules, without the 422 rule.
	data, err := hx2.Config{NoErrorSwap: true}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Substring(data, `"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},`+
		`{"code":"[45]..","swap":false,"error":true},{"code":"...","swap":false}]`))

	rules := append([]hx2.ResponseRule{{
		Code:         "422",
		Swap:         true,
		Error:        true,
		IgnoreTitle:  true,
		Select:       "#form",
		Target:       "#main",
		SwapOverride: hx2.SwapOuterHTML,
		Event:        "itemRefused",
	}}, hx2.DefaultResponseHandling()...)
	data, err = hx2.Config{ResponseHandling: rules}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Substring(data, `"responseHandling":[{"code":"422","swap":true,"error":true,"ignoreTitle":true,`+
		`"select":"#form","target":"#main","swapOverride":"outerHTML","event":"itemRefused"},{"code":"204","swap":false},`))

	// An empty, non-nil list means the default rules.
	data, err = hx2.Config{ResponseHandling: []hx2.ResponseRule{}}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Equal(zeroConfigJSON, data))
}

// DefaultResponseHandling returns a fresh slice: changing it does not change
// the defaults.
func TestDefaultResponseHandlingIsFresh(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rules := hx2.DefaultResponseHandling()
	for i := range rules {
		rules[i].Swap = !rules[i].Swap
	}
	rules[0].Code = "x"
	settle := hx2.DefaultAttributesToSettle()
	settle[0] = "style"
	data, err := hx2.Config{}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Equal(zeroConfigJSON, data))
	t.Must(be.Equal(true, hx2.DefaultResponseHandling()[1].Swap))
	t.Must(be.Equal("204", hx2.DefaultResponseHandling()[0].Code))
	t.Must(be.DeepEqual([]string{"class", "width", "height"}, hx2.DefaultAttributesToSettle()))
}

// AttributesToSettle replaces the default list when not empty; names must
// be attribute names.
func TestConfigAttributesToSettle(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// An empty, non-nil list means the default.
	data, err := hx2.Config{AttributesToSettle: []string{}}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Equal(zeroConfigJSON, data))

	names := append(hx2.DefaultAttributesToSettle(), "data-state", "aria-expanded", "xlink:href", "viewBox", "x_y.z", "h1")
	got := configValues(t, hx2.Config{AttributesToSettle: names})
	want := make([]any, len(names))
	for i, n := range names {
		want[i] = n
	}
	t.Must(be.DeepEqual[any](want, got["attributesToSettle"]))

	for _, name := range []string{"", " class", "class ", "a b", "a\tb", "a=b", `a"`, "a'", "a>", "a/", "naïve", "\x00"} {
		data, err := hx2.Config{AttributesToSettle: []string{"class", name}}.JSON()
		t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidConfig), "name %q", name))
		t.Must(be.Equal("", data))
	}
}

func TestConfigDefaultSwapStyle(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range []string{
		hx2.SwapInnerHTML, hx2.SwapOuterHTML, hx2.SwapBeforeBegin, hx2.SwapAfterBegin,
		hx2.SwapBeforeEnd, hx2.SwapAfterEnd, hx2.SwapDelete, hx2.SwapNone,
	} {
		got := configValues(t, hx2.Config{DefaultSwapStyle: s})
		t.Must(be.DeepEqual[any](s, got["defaultSwapStyle"]))
	}
	// htmx falls back to the default style for unknown styles, so the default
	// itself must be one that htmx swaps directly.
	for _, s := range []string{hx2.SwapTextContent, "bogus", "innerhtml", "outerHTML show:top", " outerHTML"} {
		_, err := hx2.Config{DefaultSwapStyle: s}.JSON()
		t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidConfig), "style %q", s))
	}
}

func TestConfigExtra(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cfg := hx2.Config{Extra: map[string]any{
		"timeout":            5000,
		"scrollBehavior":     "smooth",
		"defaultSettleDelay": 0,
		"disableSelector":    []string{"[data-off]"},
		"zzz":                nil,
	}}
	data, err := cfg.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Equal(`{"allowEval":false,"allowScriptTags":false,"attributesToSettle":["class","width","height"],`+
		`"defaultSettleDelay":0,"disableSelector":["[data-off]"],`+
		`"historyCacheSize":0,"historyRestoreAsHxRequest":false,"includeIndicatorStyles":false,`+
		`"methodsThatUseUrlParams":["get"],`+
		`"refreshOnHistoryMiss":true,"responseHandling":[{"code":"204","swap":false},`+
		`{"code":"[23]..","swap":true},{"code":"422","swap":true},{"code":"[45]..","swap":true,"error":true},`+
		`{"code":"...","swap":false}],`+
		`"scrollBehavior":"smooth","selfRequestsOnly":true,"timeout":5000,"zzz":null}`, data))
}

// htmx compiles each rule code as a JavaScript regular expression on every
// response, and an invalid one breaks every swap of the page: JSON accepts
// only three positions of digits, '.' and digit classes.
func TestConfigResponseCode(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rules := func(code string) hx2.Config {
		return hx2.Config{ResponseHandling: append([]hx2.ResponseRule{{Code: code, Swap: true}}, hx2.DefaultResponseHandling()...)}
	}
	for _, code := range []string{"204", "[23]..", "[45]..", "...", "422", "[0-4]0[1-3]", "[0-9][0-9][0-9]", "[1-1]..", "[0-24-6].."} {
		data, err := rules(code).JSON()
		t.Must(be.Annotate(be.NoError(err), "code %q", code))
		t.Must(be.Substring(data, `{"code":"`+code+`","swap":true}`))
	}
	for _, code := range []string{
		"", "[45", "[45].", "20", "2000", "[45]...", "[5-0]..", "[]..", "[4-]..", "[-4]..", "[4-",
		`2\d\d`, "200|201", "(?!204)...", "[^4]..", "4x.", " 404", "404 ", `[4\]5]..`, "٤٠٤", "4[0-9-5].",
	} {
		data, err := rules(code).JSON()
		t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidConfig), "code %q", code))
		t.Must(be.Equal("", data))
	}
}

// The same Config always gives the same text, whatever the map order.
func TestConfigJSONDeterministic(gt *testing.T) {
	t := devtest.ParallelT(gt)
	extra := map[string]any{}
	for _, k := range strings.Fields("a b c d e f g h i j k l m n o p q r s t u v w x y z") {
		extra[k] = map[string]int{k + "1": 1, k + "2": 2, k + "0": 0}
	}
	cfg := hx2.Config{Extra: extra, HistoryCacheSize: 3}
	first, err := cfg.JSON()
	t.Must(be.NoError(err))
	for range 50 {
		again, err := cfg.JSON()
		t.Must(be.NoError(err))
		t.Must(be.Equal(first, again))
	}
}

func TestConfigInvalid(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		cfg  hx2.Config
	}{
		{"negative history cache", hx2.Config{HistoryCacheSize: -1}},
		{"min int history cache", hx2.Config{HistoryCacheSize: math.MinInt}},
		{"NoErrorSwap with rules", hx2.Config{NoErrorSwap: true, ResponseHandling: hx2.DefaultResponseHandling()}},
		{"rule without code", hx2.Config{ResponseHandling: []hx2.ResponseRule{{Swap: true}}}},
		{"empty attribute to settle", hx2.Config{AttributesToSettle: []string{""}}},
		{"spaced attribute to settle", hx2.Config{AttributesToSettle: []string{"class", "width height"}}},
		{"empty extra key", hx2.Config{Extra: map[string]any{"": 1}}},
		{"extra channel", hx2.Config{Extra: map[string]any{"x": make(chan int)}}},
		{"extra func", hx2.Config{Extra: map[string]any{"x": func() {}}}},
		{"extra NaN", hx2.Config{Extra: map[string]any{"x": math.NaN()}}},
		{"extra infinity", hx2.Config{Extra: map[string]any{"x": []float64{math.Inf(1)}}}},
	}
	// Extra must not override what a field controls, even with the same
	// value: a field is the only way to weaken the defaults.
	for _, k := range []string{
		"includeIndicatorStyles", "allowEval", "allowScriptTags", "historyCacheSize",
		"refreshOnHistoryMiss", "historyRestoreAsHxRequest", "selfRequestsOnly",
		"methodsThatUseUrlParams", "attributesToSettle", "responseHandling", "defaultSwapStyle",
	} {
		cases = append(cases, struct {
			name string
			cfg  hx2.Config
		}{"extra " + k, hx2.Config{Extra: map[string]any{k: true}}})
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			data, err := c.cfg.JSON()
			t.Must(be.ErrorIs(err, hx2.ErrInvalidConfig))
			t.Must(be.Equal("", data))

			// The meta element fails the render with the same error.
			out, err := renderNode(t.Context(), c.cfg.Meta())
			t.Must(be.ErrorIs(err, hx2.ErrInvalidConfig))
			t.Must(be.Equal("", out))
		})
	}

	// Encoding errors stay inspectable.
	_, err := hx2.Config{Extra: map[string]any{"x": make(chan int)}}.JSON()
	var typeErr *json.UnsupportedTypeError
	t.Must(be.True(errors.As(err, &typeErr)))
}

func TestConfigMeta(gt *testing.T) {
	t := devtest.ParallelT(gt)
	out, err := renderNode(t.Context(), hx2.Config{}.Meta())
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<meta name="htmx-config" content="`+
		strings.ReplaceAll(zeroConfigJSON, `"`, "&#34;")+`"/>`, out))

	page := ct.View(hx2.Config{}.Meta())
	t.Must(page.Find(ct.Tag("meta"), ct.Attr("name", "htmx-config")).Count(1))
}

// The content attribute survives the HTML parser intact, whatever the
// values hold: quotes, angle brackets, ampersands, and markup.
func TestConfigMetaEscaping(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cfg := hx2.Config{Extra: map[string]any{
		"quotes": `"double" 'single'`,
		"markup": `</script><script>alert(1)</script>&amp;<!--`,
		"text":   "naïve ✓ \u2028",
	}}
	want, err := cfg.JSON()
	t.Must(be.NoError(err))
	out, err := renderNode(t.Context(), cfg.Meta())
	t.Must(be.NoError(err))
	t.Must(be.False(strings.Contains(out, "<script")))
	t.Must(be.False(strings.Contains(out, "'")))

	parsed := cthtml.FragmentString("head", out)
	t.Must(parsed.Find(ct.Tag("meta")).Count(1))
	got, ok, err := parsed.Find(ct.Tag("meta")).Attr(t.Context(), "content")
	t.Must(be.NoError(err))
	t.Must(be.True(ok))
	t.Must(be.Equal(want, got))

	var m map[string]any
	t.Must(be.NoError(json.Unmarshal([]byte(got), &m)))
	t.Must(be.DeepEqual[any](`</script><script>alert(1)</script>&amp;<!--`, m["markup"]))
	t.Must(be.DeepEqual[any](`"double" 'single'`, m["quotes"]))
}

// Meta encodes the configuration when it is called: later changes to the
// Config do not show in the node.
func TestConfigMetaSnapshot(gt *testing.T) {
	t := devtest.ParallelT(gt)
	extra := map[string]any{"timeout": 1}
	rules := hx2.DefaultResponseHandling()
	settle := hx2.DefaultAttributesToSettle()
	cfg := hx2.Config{Extra: extra, ResponseHandling: rules, AttributesToSettle: settle}
	node := cfg.Meta()
	before, err := renderNode(t.Context(), node)
	t.Must(be.NoError(err))
	extra["timeout"] = 2
	extra["allowEval"] = true
	rules[0].Swap = true
	settle[0] = "style"
	after, err := renderNode(t.Context(), node)
	t.Must(be.NoError(err))
	t.Must(be.Equal(before, after))
}

// NoErrorSwap is the opt-out for pages whose htmx requests reach handlers
// that may echo request data into error bodies: htmx then swaps no 4xx or
// 5xx body, 422 included, while the zero Config swaps them all. The rules
// are read back from the JSON that htmx gets.
func TestNoErrorSwapStatuses(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rulesOf := func(c hx2.Config) []hx2.ResponseRule {
		data, err := c.JSON()
		t.Must(be.NoError(err))
		var v struct {
			ResponseHandling []hx2.ResponseRule `json:"responseHandling"`
		}
		t.Must(be.NoError(json.Unmarshal([]byte(data), &v)))
		return v.ResponseHandling
	}
	swaps := func(rules []hx2.ResponseRule, status string) bool {
		for _, r := range rules {
			if regexp.MustCompile(r.Code).MatchString(status) {
				return r.Swap
			}
		}
		return false
	}
	zero, off := rulesOf(hx2.Config{}), rulesOf(hx2.Config{NoErrorSwap: true})
	for code := 400; code < 600; code++ {
		status := strconv.Itoa(code)
		t.Must(be.Annotate(be.True(swaps(zero, status)), "zero Config, status %s", status))
		t.Must(be.Annotate(be.False(swaps(off, status)), "NoErrorSwap, status %s", status))
	}
	for _, status := range []string{"200", "201", "303"} {
		t.Must(be.Annotate(be.True(swaps(off, status)), "NoErrorSwap, status %s", status))
	}
	t.Must(be.False(swaps(off, "204")))
}
