package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// StatusStopPolling is the status of an answer that stops a poll: htmx
// cancels the "every" trigger of an element ([Trigger]) when it swaps an
// answer with status 286 to a request of that element. No other status
// stops a poll, error statuses included: the poll keeps asking after a
// failure, and an error answer that is swapped elsewhere, such as the
// alert of [github.com/protolambda/webui/web.Errors] in the alerts region,
// comes again on every tick.
//
// A handler answers a poll whose subject is gone, such as an item deleted
// meanwhile, with this status and the content that takes the polled
// content's place, through [github.com/protolambda/webui/web.Render],
// which takes any status:
//
//	return web.Render(w, r, hx2.StatusStopPolling, text.P()(text.Text("This task was deleted.")))
//
// htmx stops the poll only when it swaps the answer:
// [DefaultResponseHandling] swaps 286 as a 2xx answer, and rules of the
// application's own must swap it too. The answer replaces the target as
// any other answer does: an empty one empties an innerHTML target and
// removes an outerHTML one, and an element in it with an "every" trigger
// starts a new poll.
const StatusStopPolling = 286

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
//
// A poll ("every 2s") asks until the element leaves the page or htmx swaps
// an answer with status [StatusStopPolling] (286); no other status stops
// it, errors included. With webui's error answers, which go to the alerts
// region, a poll that keeps failing puts its alert there again on every
// tick: answer a poll whose subject is gone with StatusStopPolling. An
// answer with status 204 swaps nothing and keeps the poll going, for a
// poll that found no change.
func Trigger(v string) attr.Node { return attr.Name("hx-trigger").Value(v) }
