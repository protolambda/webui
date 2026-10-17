package hx2_test

import (
	"fmt"
	"net/http"

	"github.com/protolambda/chord/core"

	"github.com/protolambda/webui/hx2"
)

// The zero Config is webui's secure htmx configuration. chord escapes the
// JSON text in the content attribute.
func ExampleConfig_Meta() {
	core.Dump(hx2.Config{}.Meta())
	// Output: <meta name="htmx-config" content="{&#34;allowEval&#34;:false,&#34;allowScriptTags&#34;:false,&#34;historyCacheSize&#34;:0,&#34;historyRestoreAsHxRequest&#34;:false,&#34;includeIndicatorStyles&#34;:false,&#34;refreshOnHistoryMiss&#34;:true,&#34;responseHandling&#34;:[{&#34;code&#34;:&#34;204&#34;,&#34;swap&#34;:false},{&#34;code&#34;:&#34;[23]..&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;[45]..&#34;,&#34;swap&#34;:true,&#34;error&#34;:true},{&#34;code&#34;:&#34;...&#34;,&#34;swap&#34;:false}],&#34;selfRequestsOnly&#34;:true}"/>
}

func ExampleConfig_JSON() {
	data, err := hx2.Config{}.JSON()
	if err != nil {
		panic(err)
	}
	fmt.Println(data)
	// Output: {"allowEval":false,"allowScriptTags":false,"historyCacheSize":0,"historyRestoreAsHxRequest":false,"includeIndicatorStyles":false,"refreshOnHistoryMiss":true,"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"[45]..","swap":true,"error":true},{"code":"...","swap":false}],"selfRequestsOnly":true}
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
	// Output: {"allowEval":false,"allowScriptTags":false,"defaultSwapStyle":"outerHTML","historyCacheSize":5,"historyRestoreAsHxRequest":false,"includeIndicatorStyles":false,"refreshOnHistoryMiss":true,"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"[45]..","swap":false,"error":true},{"code":"...","swap":false}],"selfRequestsOnly":true,"timeout":10000}
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
	// [45].. swap=true error=true
	// ... swap=false error=false
	// <nil>
}

func ExampleAddVary() {
	h := http.Header{}
	h.Set("Vary", "Accept-Encoding")
	hx2.AddVary(h)
	hx2.AddVary(h) // already present: no change
	fmt.Println(h.Values("Vary"))
	// Output: [Accept-Encoding HX-Request]
}
