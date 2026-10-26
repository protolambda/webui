package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// Boost sets the hx-boost attribute to convert links and forms to AJAX.
func Boost(v string) attr.Node { return attr.Name("hx-boost").Value(v) }

// Preserve sets the hx-preserve boolean attribute to preserve an element
// across swaps: when a swap brings in an element with the attribute, htmx
// keeps the element of the page with the same id in its place, as it is,
// and drops the new one. The element keeps what the user did with it, such
// as an open details disclosure or the values typed into its fields, and
// what the answer has for it is ignored: an answer that is to replace the
// element renders it without the attribute.
//
// The element needs an id that is the same in the page and in the new
// content: htmx finds the element to keep by the id of the new one, and
// without an id it silently does nothing, so the element is replaced like
// the rest. htmx 1 takes the element out of the page during the swap, so
// the focus does not survive it: it falls to the body.
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
func Ext(v string) attr.Node { return attr.Name("hx-ext").Value(v) }
