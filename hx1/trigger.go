package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// Trigger sets the hx-trigger attribute to specify what triggers the request.
//
// An event filter in square brackets, such as "click[ctrlKey]", is a
// JavaScript expression. When the page disables eval (htmx.config.allowEval
// false, or a Content-Security-Policy without 'unsafe-eval'), htmx cannot
// evaluate it, drops it, and triggers on every matching event. Never build a
// filter from untrusted input.
func Trigger(v string) attr.Node { return attr.Name("hx-trigger").Value(v) }
