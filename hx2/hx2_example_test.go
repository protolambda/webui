package hx2_test

import (
	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/hx2"
)

func ExampleGet() {
	core.Dump(button.Button(hx2.Get("/api/data"))(text.Text("Load")))
	// Output: <button hx-get="/api/data">Load</button>
}

func ExamplePost() {
	core.Dump(elem.Name("form").New(hx2.Post("/api/submit")))
	// Output: <form hx-post="/api/submit"></form>
}

func ExampleTarget() {
	core.Dump(button.Button(hx2.Get("/content"), hx2.Target("#result")))
	// Output: <button hx-get="/content" hx-target="#result"></button>
}

func ExampleSwap() {
	core.Dump(div.Div(hx2.Get("/list"), hx2.Swap(hx2.SwapInnerHTML)))
	// Output: <div hx-get="/list" hx-swap="innerHTML"></div>
}

// The swap style constants are untyped strings, so modifiers can follow.
func ExampleSwap_modifiers() {
	core.Dump(div.Div(hx2.Get("/items/7"), hx2.Swap(hx2.SwapOuterHTML+" show:top")))
	// Output: <div hx-get="/items/7" hx-swap="outerHTML show:top"></div>
}

// On writes JavaScript into an attribute: it needs Config.AllowEval and a
// Content-Security-Policy that allows 'unsafe-eval', and its handler must
// never contain untrusted input.
func ExampleOn() {
	core.Dump(button.Button(hx2.On("click", "alert('clicked')")))
	// Output: <button hx-on:click="alert(&#39;clicked&#39;)"></button>
}

// The HTML parser lowercases attribute names, so htmx events are named in
// kebab-case: hx-on:htmx:afterSwap would be read as hx-on:htmx:afterswap,
// which matches no event.
func ExampleOn_htmxEvent() {
	core.Dump(div.Div(hx2.On("htmx:after-swap", "console.log('swapped')")))
	// Output: <div hx-on:htmx:after-swap="console.log(&#39;swapped&#39;)"></div>
}

// htmx reads "hx-on::" as a shorthand for "hx-on:htmx:".
func ExampleOn_shorthand() {
	core.Dump(div.Div(hx2.On(":before-request", "console.log('sending')")))
	// Output: <div hx-on::before-request="console.log(&#39;sending&#39;)"></div>
}

func ExampleInherit() {
	core.Dump(div.Div(hx2.Inherit("hx-target hx-swap")))
	// Output: <div hx-inherit="hx-target hx-swap"></div>
}

func ExampleDisabledElt() {
	core.Dump(button.Button(hx2.DisabledElt("this"))(text.Text("Submit")))
	// Output: <button hx-disabled-elt="this">Submit</button>
}

func ExampleTrigger() {
	core.Dump(div.Div(hx2.Get("/poll"), hx2.Trigger("every 5s")))
	// Output: <div hx-get="/poll" hx-trigger="every 5s"></div>
}

func ExampleIndicator() {
	core.Dump(button.Button(hx2.Post("/slow"), hx2.Indicator("#loader")))
	// Output: <button hx-post="/slow" hx-indicator="#loader"></button>
}

func ExampleExt() {
	core.Dump(div.Div(hx2.Ext("sse"), hx2.SSEConnect("/events")))
	// Output: <div hx-ext="sse" sse-connect="/events"></div>
}

func ExampleSSEConnect() {
	core.Dump(div.Div(hx2.Ext("sse"), hx2.SSEConnect("/events"), hx2.SSESwap("message")))
	// Output: <div hx-ext="sse" sse-connect="/events" sse-swap="message"></div>
}

func ExampleWSConnect() {
	core.Dump(div.Div(hx2.Ext("ws"), hx2.WSConnect("/ws")))
	// Output: <div hx-ext="ws" ws-connect="/ws"></div>
}

func Example_combined() {
	core.Dump(div.Div(
		attr.ID("content"),
		hx2.Get("/api/data"),
		hx2.Target("#content"),
		hx2.Swap(hx2.SwapOuterHTML),
		hx2.Trigger("load"),
		hx2.Indicator("#content-loading"),
	))
	// Output: <div id="content" hx-get="/api/data" hx-target="#content" hx-swap="outerHTML" hx-trigger="load" hx-indicator="#content-loading"></div>
}

// A polling panel that the server answers with 204 No Content while nothing
// changed: htmx swaps nothing for 204.
func Example_polling() {
	core.Dump(div.Div(
		attr.ID("status"),
		hx2.Get("/status?version=3"),
		hx2.Trigger("every 2s"),
		hx2.Swap(hx2.SwapOuterHTML),
		hx2.Sync("this:replace"),
	))
	// Output: <div id="status" hx-get="/status?version=3" hx-trigger="every 2s" hx-swap="outerHTML" hx-sync="this:replace"></div>
}

func ExampleHistory() {
	core.Dump(div.Div(attr.ID("settings"), hx2.History(false)))
	// Output: <div id="settings" hx-history="false"></div>
}

func ExampleValsJSON() {
	type move struct {
		Op   string `json:"op"`
		Item int    `json:"item"`
	}
	core.Dump(button.Button(
		button.Type(button.TypeButton),
		hx2.Post("/tasks/reorder"),
		hx2.ValsJSON(move{Op: "up", Item: 3}),
	)(text.Text("Move up")))
	// Output: <button type="button" hx-post="/tasks/reorder" hx-vals="{&#34;op&#34;:&#34;up&#34;,&#34;item&#34;:3}">Move up</button>
}

// A value that does not encode as a JSON object fails the render instead of
// producing markup that htmx cannot read.
func ExampleValsJSON_invalid() {
	core.Dump(button.Button(hx2.Post("/tasks"), hx2.ValsJSON([]string{"not", "an", "object"})))
	// Output: ERROR: render: at button[0]: attribute: invalid htmx attribute value: hx-vals is not a JSON object
}

func ExampleHeadersJSON() {
	core.Dump(elem.Name("form").New(
		hx2.Post("/notes"),
		hx2.HeadersJSON(map[string]string{"X-Note-Version": "12"}),
	))
	// Output: <form hx-post="/notes" hx-headers="{&#34;X-Note-Version&#34;:&#34;12&#34;}"></form>
}
