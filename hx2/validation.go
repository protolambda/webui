package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Validate sets the hx-validate attribute to control form validation:
// Validate("true") makes htmx check the constraints (such as required and
// pattern) of the fields that a request of the element sends before it
// sends it, as htmx always does for the requests of a form without
// novalidate: the element itself (or the fields of a form), the fields of
// its enclosing form for a request other than GET, and those of
// hx-include.
//
// With webui's htmx configuration, a request that fails the check stops
// silently: the zero [Config] leaves htmx's reportValidityOfForms off, so
// htmx fires htmx:validation:failed and htmx:validation:halted, which
// only script shows, and the browser shows no message and moves no focus.
// On an element that is not a form, such as an htmx button, a click then
// seems to do nothing.
//
// A click on an htmx button inside a form (the button has hx-post, not
// the form) also skips the browser's own check, which the browser makes
// only when it submits a form: htmx cancels that submission and sends its
// own request, with whatever the fields hold unless the button has
// Validate("true"). An ordinary submit of a form with hx-post is not
// affected: the browser checks the fields, and reports the first invalid
// one, before htmx sees the submit event.
//
// Either have htmx report the first invalid field of a request that it
// stops (the browser's message, and the focus on the field):
//
//	hx2.Config{Extra: map[string]any{"reportValidityOfForms": true}}
//
// or validate on the server, as every handler must anyway, and answer
// with the form and its messages.
func Validate(v string) attr.Node { return attr.Name("hx-validate").Value(v) }
