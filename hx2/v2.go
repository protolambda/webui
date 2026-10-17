package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// On sets an hx-on:event attribute: htmx runs handler, JavaScript code, when
// the event fires on the element. On("click", "...") renders
// hx-on:click="...".
//
// On writes JavaScript into an attribute. chord HTML-escapes the value, but
// HTML escaping does not protect a JavaScript context: never interpolate
// untrusted input into handler. On("click", "load('"+name+"')") is script
// injection when name comes from a user.
//
// htmx compiles the handler with the Function constructor, so it runs only
// when the page enables [Config.AllowEval] (off in webui's default Config)
// and its Content-Security-Policy allows 'unsafe-eval' in script-src, which
// webui's default policy does not. Without both, htmx ignores the attribute.
// Prefer a same-origin script that listens for the event, or a response
// header such as HX-Trigger (see [SetTrigger]).
//
// The HTML parser lowercases attribute names, so a camelCase event name
// cannot match: hx-on:htmx:afterSwap is read as hx-on:htmx:afterswap, which
// is no htmx event. Use the kebab-case names, which htmx 2 dispatches next to
// the camelCase ones: "htmx:after-swap", "htmx:before-request". htmx also
// reads the shorthand "::after-swap" (event ":after-swap") as
// hx-on:htmx:after-swap.
//
// The attribute name "hx-on:"+event is validated when the node is
// evaluated: an event that is not safe in an attribute name, such as one
// with a space or a quote, fails the render with [attr.ErrInvalidName].
//
// On is distinct from the chord html/on helpers, which render plain DOM
// handler attributes such as onclick="..."; a strict Content-Security-Policy
// blocks those too.
func On(event, handler string) attr.Node { return attr.KV("hx-on:"+event, handler) }

// Inherit sets the hx-inherit attribute to control attribute inheritance:
// a space-separated list of the attributes that descendants inherit, or "*".
func Inherit(v string) attr.Node { return attr.Name("hx-inherit").Value(v) }
