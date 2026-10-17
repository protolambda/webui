package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Boost sets the hx-boost attribute to convert links and forms to AJAX.
func Boost(v string) attr.Node { return attr.Name("hx-boost").Value(v) }

// Preserve sets the hx-preserve boolean attribute to preserve element across swaps.
func Preserve() attr.Node { return attr.Name("hx-preserve").Bool() }

// Confirm sets the hx-confirm attribute to show confirmation dialog before request.
func Confirm(v string) attr.Node { return attr.Name("hx-confirm").Value(v) }

// Prompt sets the hx-prompt attribute to show prompt dialog before request.
func Prompt(v string) attr.Node { return attr.Name("hx-prompt").Value(v) }

// Disable sets the hx-disable boolean attribute to disable HTMX processing.
func Disable() attr.Node { return attr.Name("hx-disable").Bool() }

// DisabledElt sets the hx-disabled-elt attribute to disable elements during request.
func DisabledElt(v string) attr.Node { return attr.Name("hx-disabled-elt").Value(v) }

// Disinherit sets the hx-disinherit attribute to prevent inheritance of attributes.
func Disinherit(v string) attr.Node { return attr.Name("hx-disinherit").Value(v) }

// Encoding sets the hx-encoding attribute to set request encoding.
func Encoding(v string) attr.Node { return attr.Name("hx-encoding").Value(v) }

// Ext sets the hx-ext attribute to enable HTMX extensions.
// Multiple extensions can be comma-separated: Ext("sse,ws").
// In v2, SSE and WebSocket require extensions: use Ext("sse") with [SSEConnect],
// or Ext("ws") with [WSConnect].
// Other extensions: "json-enc", "class-tools", "preload", "path-deps", etc.
func Ext(v string) attr.Node { return attr.Name("hx-ext").Value(v) }
