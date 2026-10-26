package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// Validate sets the hx-validate attribute to control form validation:
// Validate("true") makes htmx check the constraints (such as required and
// pattern) of the fields that a request of the element sends before it
// sends it, as htmx does for the requests of a form without novalidate.
//
// htmx 1 stops a request that fails the check without reporting it: it
// fires htmx:validation:failed and htmx:validation:halted, which only
// script shows, and the browser shows no message and moves no focus, so a
// click on an htmx button with Validate("true") seems to do nothing. A
// click on an htmx button inside a form (the button has hx-post, not the
// form) also skips the browser's own check, which the browser makes only
// when it submits a form: htmx cancels that submission and sends its own
// request. An ordinary submit of a form with hx-post is not affected: the
// browser checks the fields, and reports, before htmx sees the submit
// event. Validate on the server, and answer with the form and its
// messages; htmx 2 can report the field instead (see
// [github.com/protolambda/webui/hx2.Validate]).
func Validate(v string) attr.Node { return attr.Name("hx-validate").Value(v) }
