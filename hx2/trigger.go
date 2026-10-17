package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Trigger sets the hx-trigger attribute to specify what triggers the request,
// e.g. Trigger("load"), Trigger("every 2s"), or
// Trigger("input changed delay:250ms, search").
//
// An event filter in square brackets, such as "click[ctrlKey]", is a
// JavaScript expression, which htmx evaluates only with [Config.AllowEval]
// and a Content-Security-Policy that allows 'unsafe-eval'. Never build one
// from untrusted input.
func Trigger(v string) attr.Node { return attr.Name("hx-trigger").Value(v) }
