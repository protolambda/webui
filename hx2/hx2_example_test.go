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
	core.Dump(div.Div(hx2.Get("/list"), hx2.Swap("innerHTML")))
	// Output: <div hx-get="/list" hx-swap="innerHTML"></div>
}

func ExampleOn() {
	core.Dump(button.Button(hx2.On("click", "alert('clicked')")))
	// Output: <button hx-on:click="alert(&#39;clicked&#39;)"></button>
}

func ExampleOn_htmxEvent() {
	core.Dump(div.Div(hx2.On("htmx:afterSwap", "console.log('swapped')")))
	// Output: <div hx-on:htmx:afterSwap="console.log(&#39;swapped&#39;)"></div>
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
		hx2.Swap("outerHTML"),
		hx2.Trigger("load"),
		hx2.On("htmx:before-request", "showSpinner()"),
		hx2.On("htmx:after-request", "hideSpinner()"),
	))
	// Output: <div id="content" hx-get="/api/data" hx-target="#content" hx-swap="outerHTML" hx-trigger="load" hx-on:htmx:before-request="showSpinner()" hx-on:htmx:after-request="hideSpinner()"></div>
}
