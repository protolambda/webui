package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Swap styles, for [Swap], [SetReswap], [ResponseRule.SwapOverride], and
// [Config.DefaultSwapStyle]. They are untyped string constants, so that
// modifiers can follow them: Swap(SwapOuterHTML + " show:top").
//
// Reference: https://htmx.org/attributes/hx-swap/
const (
	// SwapInnerHTML replaces the content of the target element. It is the
	// htmx default.
	SwapInnerHTML = "innerHTML"
	// SwapOuterHTML replaces the target element itself.
	SwapOuterHTML = "outerHTML"
	// SwapTextContent replaces the content of the target element with the
	// text of the response, without parsing it as HTML.
	SwapTextContent = "textContent"
	// SwapBeforeBegin inserts the response before the target element.
	SwapBeforeBegin = "beforebegin"
	// SwapAfterBegin inserts the response before the first child of the
	// target element.
	SwapAfterBegin = "afterbegin"
	// SwapBeforeEnd inserts the response after the last child of the target
	// element.
	SwapBeforeEnd = "beforeend"
	// SwapAfterEnd inserts the response after the target element.
	SwapAfterEnd = "afterend"
	// SwapDelete deletes the target element, whatever the response.
	SwapDelete = "delete"
	// SwapNone swaps nothing; out-of-band elements of the response are still
	// swapped.
	SwapNone = "none"
)

// Target sets the hx-target attribute to specify where to swap response content.
// The value is a CSS selector, or an htmx extended selector such as "this",
// "closest tr" or "find .result".
func Target(v string) attr.Node { return attr.Name("hx-target").Value(v) }

// Swap sets the hx-swap attribute to control how response content is swapped:
// a swap style, such as [SwapOuterHTML], optionally followed by modifiers,
// e.g. Swap(SwapOuterHTML + " show:top").
func Swap(v string) attr.Node { return attr.Name("hx-swap").Value(v) }

// SwapOOB sets the hx-swap-oob attribute for out-of-band swaps: an element
// of a response with hx-swap-oob="true" replaces the element with the same
// id anywhere in the page.
func SwapOOB(v string) attr.Node { return attr.Name("hx-swap-oob").Value(v) }

// Select sets the hx-select attribute to select a subset of response content.
func Select(v string) attr.Node { return attr.Name("hx-select").Value(v) }

// SelectOOB sets the hx-select-oob attribute for out-of-band selection.
func SelectOOB(v string) attr.Node { return attr.Name("hx-select-oob").Value(v) }
