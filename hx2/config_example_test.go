package hx2_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/protolambda/chord/core"

	"github.com/protolambda/webui/hx2"
)

// The zero Config is webui's secure htmx configuration. chord escapes the
// JSON text in the content attribute.
func ExampleConfig_Meta() {
	core.Dump(hx2.Config{}.Meta())
	// Output: <meta name="htmx-config" content="{&#34;allowEval&#34;:false,&#34;allowScriptTags&#34;:false,&#34;attributesToSettle&#34;:[&#34;class&#34;,&#34;width&#34;,&#34;height&#34;],&#34;historyCacheSize&#34;:0,&#34;historyRestoreAsHxRequest&#34;:false,&#34;includeIndicatorStyles&#34;:false,&#34;methodsThatUseUrlParams&#34;:[&#34;get&#34;],&#34;refreshOnHistoryMiss&#34;:true,&#34;responseHandling&#34;:[{&#34;code&#34;:&#34;204&#34;,&#34;swap&#34;:false},{&#34;code&#34;:&#34;[23]..&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;422&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;[45]..&#34;,&#34;swap&#34;:true,&#34;error&#34;:true},{&#34;code&#34;:&#34;...&#34;,&#34;swap&#34;:false}],&#34;selfRequestsOnly&#34;:true}"/>
}

func ExampleConfig_JSON() {
	data, err := hx2.Config{}.JSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(data)
	// Output: {"allowEval":false,"allowScriptTags":false,"attributesToSettle":["class","width","height"],"historyCacheSize":0,"historyRestoreAsHxRequest":false,"includeIndicatorStyles":false,"methodsThatUseUrlParams":["get"],"refreshOnHistoryMiss":true,"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"422","swap":true},{"code":"[45]..","swap":true,"error":true},{"code":"...","swap":false}],"selfRequestsOnly":true}
}

// Fields opt into htmx behaviors; Extra adds other htmx config keys, in
// sorted order with the rest.
func ExampleConfig_JSON_optIn() {
	cfg := hx2.Config{
		HistoryCacheSize: 5,
		NoErrorSwap:      true,
		DefaultSwapStyle: hx2.SwapOuterHTML,
		Extra:            map[string]any{"timeout": 10000},
	}
	data, err := cfg.JSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(data)
	// Output: {"allowEval":false,"allowScriptTags":false,"attributesToSettle":["class","width","height"],"defaultSwapStyle":"outerHTML","historyCacheSize":5,"historyRestoreAsHxRequest":false,"includeIndicatorStyles":false,"methodsThatUseUrlParams":["get"],"refreshOnHistoryMiss":true,"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"[45]..","swap":false,"error":true},{"code":"...","swap":false}],"selfRequestsOnly":true,"timeout":10000}
}

// DeleteURLParams restores htmx 2's default of DELETE parameters in the URL
// (no body), and AttributesToSettle replaces the attributes that htmx
// settles. Settling "style" needs a style-src that allows inline styles.
func ExampleConfig_JSON_htmxDefaults() {
	cfg := hx2.Config{
		DeleteURLParams:    true,
		AttributesToSettle: []string{"class", "style", "width", "height"},
	}
	data, err := cfg.JSON()
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		panic(err)
	}
	fmt.Println(m["methodsThatUseUrlParams"], m["attributesToSettle"])
	// Output: [get delete] [class style width height]
}

// Extra cannot override a key that a field controls.
func ExampleConfig_JSON_conflict() {
	_, err := hx2.Config{Extra: map[string]any{"allowEval": true}}.JSON()
	fmt.Println(err)
	// Output: invalid htmx config: extra key "allowEval" is controlled by a Config field
}

// Rules in front of the defaults take precedence: here a 404 response is
// not swapped, while other errors still are.
func ExampleDefaultResponseHandling() {
	rules := append([]hx2.ResponseRule{{Code: "404", Swap: false, Error: true}}, hx2.DefaultResponseHandling()...)
	for _, r := range rules {
		fmt.Printf("%s swap=%t error=%t\n", r.Code, r.Swap, r.Error)
	}
	_, err := hx2.Config{ResponseHandling: rules}.JSON()
	fmt.Println(err)
	// Output:
	// 404 swap=false error=true
	// 204 swap=false error=false
	// [23].. swap=true error=false
	// 422 swap=true error=false
	// [45].. swap=true error=true
	// ... swap=false error=false
	// <nil>
}

// A settled attribute can be added to webui's list, which leaves out
// "style".
func ExampleDefaultAttributesToSettle() {
	settle := append(hx2.DefaultAttributesToSettle(), "data-state")
	fmt.Println(settle)
	_, err := hx2.Config{AttributesToSettle: settle}.JSON()
	fmt.Println(err)
	_, err = hx2.Config{AttributesToSettle: []string{"class width"}}.JSON()
	fmt.Println(err)
	// Output:
	// [class width height data-state]
	// <nil>
	// invalid htmx config: attribute to settle 0: "class width" is not an attribute name
}

// A handler that answers a failed htmx request with an alert fragment for
// the page's alerts region. htmx swaps it like a success, so the handler
// also turns off the inherited hx-select and the history update.
func ExampleDefaultResponseHandling_errorFragment() {
	notFound := func(w http.ResponseWriter, r *http.Request) {
		if hx2.ParseRequestHeaders(r.Header).Request {
			h := w.Header()
			hx2.AddVary(h)
			hx2.SetRetarget(h, "#alerts")
			hx2.SetReswap(h, hx2.SwapInnerHTML)
			hx2.SetReselect(h, "unset")
			hx2.SetPushURL(h, "false")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `<div class="alert alert-danger" role="alert">Item not found.</div>`)
	}

	r := httptest.NewRequest(http.MethodGet, "/items/7", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Boosted", "true")
	w := httptest.NewRecorder()
	notFound(w, r)
	for _, name := range []string{"HX-Retarget", "HX-Reswap", "HX-Reselect", "HX-Push-Url", "Vary"} {
		fmt.Printf("%s: %s\n", name, w.Header().Get(name))
	}
	fmt.Println(w.Code)
	// Output:
	// HX-Retarget: #alerts
	// HX-Reswap: innerHTML
	// HX-Reselect: unset
	// HX-Push-Url: false
	// Vary: HX-Request
	// 404
}

// A rule code is three positions of digits, '.' and digit classes: an
// expression that htmx could not compile, or one that matches too much,
// fails before it reaches the page.
func ExampleResponseRule() {
	for _, code := range []string{"422", "[45]..", "[45", "20"} {
		rules := append([]hx2.ResponseRule{{Code: code, Swap: true}}, hx2.DefaultResponseHandling()...)
		_, err := hx2.Config{ResponseHandling: rules}.JSON()
		fmt.Printf("%q: %v\n", code, err)
	}
	// Output:
	// "422": <nil>
	// "[45]..": <nil>
	// "[45": invalid htmx config: response rule 0: code "[45" is not three digits, '.' or digit classes
	// "20": invalid htmx config: response rule 0: code "20" is not three digits, '.' or digit classes
}

func ExampleAddVary() {
	h := http.Header{}
	h.Set("Vary", "Accept-Encoding")
	hx2.AddVary(h)
	hx2.AddVary(h) // already present: no change
	fmt.Println(h.Values("Vary"))
	// Output: [Accept-Encoding HX-Request]
}
