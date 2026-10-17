package hx1_test

import (
	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/hx1"
)

func ExampleGet() {
	core.Dump(button.Button(hx1.Get("/api/data"))(text.Text("Load")))
	// Output: <button hx-get="/api/data">Load</button>
}

func ExamplePost() {
	core.Dump(elem.Name("form").New(hx1.Post("/api/submit")))
	// Output: <form hx-post="/api/submit"></form>
}

func ExamplePut() {
	core.Dump(button.Button(hx1.Put("/api/update"))(text.Text("Update")))
	// Output: <button hx-put="/api/update">Update</button>
}

func ExampleDelete() {
	core.Dump(button.Button(hx1.Delete("/api/item/1"))(text.Text("Delete")))
	// Output: <button hx-delete="/api/item/1">Delete</button>
}

func ExampleTarget() {
	core.Dump(button.Button(hx1.Get("/content"), hx1.Target("#result")))
	// Output: <button hx-get="/content" hx-target="#result"></button>
}

func ExampleSwap() {
	core.Dump(div.Div(hx1.Get("/list"), hx1.Swap("innerHTML")))
	// Output: <div hx-get="/list" hx-swap="innerHTML"></div>
}

func ExampleTrigger() {
	core.Dump(div.Div(hx1.Get("/check"), hx1.Trigger("every 2s")))
	// Output: <div hx-get="/check" hx-trigger="every 2s"></div>
}

func ExampleIndicator() {
	core.Dump(button.Button(hx1.Post("/slow"), hx1.Indicator("#spinner")))
	// Output: <button hx-post="/slow" hx-indicator="#spinner"></button>
}

func ExampleBoost() {
	core.Dump(text.A(attr.KV("href", "/page"), hx1.Boost("true")))
	// Output: <a href="/page" hx-boost="true"></a>
}

func ExamplePushURL() {
	core.Dump(button.Button(hx1.Get("/page"), hx1.PushURL("true")))
	// Output: <button hx-get="/page" hx-push-url="true"></button>
}

func ExampleVals() {
	core.Dump(button.Button(hx1.Post("/api"), hx1.Vals(`{"key":"value"}`)))
	// Output: <button hx-post="/api" hx-vals="{&#34;key&#34;:&#34;value&#34;}"></button>
}

func ExampleConfirm() {
	core.Dump(button.Button(hx1.Delete("/item"), hx1.Confirm("Are you sure?")))
	// Output: <button hx-delete="/item" hx-confirm="Are you sure?"></button>
}

func Example_combined() {
	core.Dump(div.Div(
		attr.ID("search-results"),
		hx1.Get("/search"),
		hx1.Trigger("keyup changed delay:500ms from:#search-input"),
		hx1.Target("#search-results"),
		hx1.Swap("innerHTML"),
	))
	// Output: <div id="search-results" hx-get="/search" hx-trigger="keyup changed delay:500ms from:#search-input" hx-target="#search-results" hx-swap="innerHTML"></div>
}

// hx-sse is htmx 1 only; htmx 2 pages use the sse extension through hx2.
func ExampleSSE() {
	core.Dump(div.Div(hx1.SSE("connect:/events swap:message")))
	// Output: <div hx-sse="connect:/events swap:message"></div>
}

// hx-ws is htmx 1 only; htmx 2 pages use the ws extension through hx2.
func ExampleWS() {
	core.Dump(div.Div(hx1.WS("connect:/chat")))
	// Output: <div hx-ws="connect:/chat"></div>
}
