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
// and a Content-Security-Policy that allows 'unsafe-eval'. Without both, as
// with webui's defaults, htmx drops the filter (firing
// htmx:evalDisallowedError or htmx:syntax:error) and triggers on every
// matching event: Trigger("keyup[key=='Enter']") then requests on every key,
// and a filter is never a guard. Filter in a same-origin script that
// dispatches a custom event to trigger on instead, or, for Enter, use a form
// and trigger on submit. Never build a filter from untrusted input.
func Trigger(v string) attr.Node { return attr.Name("hx-trigger").Value(v) }
