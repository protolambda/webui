// Package hx2 provides htmx 2 attributes, the htmx request and response
// headers, and the htmx configuration of a page, for chord views.
//
// webui's theme ships htmx 2 (2.0.10), and this is the package for new
// code. The sibling package [github.com/protolambda/webui/hx1] keeps the
// htmx 1 attributes for legacy pages.
//
// # Attributes
//
// Each attribute helper returns a chord
// [github.com/protolambda/chord/core/attr.Node] whose value chord
// HTML-escapes:
//
//   - [Get], [Post], [Put], [Patch], [Delete]: issue a request.
//   - [Target], [Swap] with the swap style constants ([SwapOuterHTML] and
//     others), [Select], [SwapOOB], [SelectOOB]: place the response.
//   - [Trigger], [Sync], [Indicator], [DisabledElt], [Confirm], [Prompt]:
//     when and how requests run.
//   - [Include], [Params], [Encoding], [ValsJSON], [HeadersJSON] (and the
//     raw [Vals], [Headers]): what a request sends.
//   - [PushURL], [ReplaceURL], [History], [HistoryElt]: browser history.
//   - [Boost], [Preserve], [Disable], [Disinherit], [Inherit], [Request],
//     [Validate], [Ext]: everything else.
//
// The JSON helpers [ValsJSON] and [HeadersJSON] encode their values with
// encoding/json, and fail the render on values that htmx cannot read
// (including member names such as "constructor", which htmx drops),
// instead of inviting JSON built by string formatting.
//
// # Configuration
//
// [Config] is the htmx configuration of a page; its zero value is webui's
// secure configuration, and [Config.Meta] renders it as the
// <meta name="htmx-config"> element of the head. It disables everything in
// htmx that needs a weaker Content-Security-Policy (inline indicator styles,
// eval, settling style attributes) or keeps page content in the browser
// (the history cache), sends DELETE parameters in the body rather than the
// URL, and makes htmx swap 4xx and 5xx responses, so that server-rendered
// error fragments show without script. htmx treats a swapped error like a
// success, so a handler that retargets an error fragment also opts out of
// the history update and of an inherited hx-select: see
// [DefaultResponseHandling]. A 422 answer to a form with problems swaps
// without being logged as an error.
//
// # Error responses are page markup
//
// htmx inserts every response that it swaps as HTML, whatever its
// Content-Type, and processes the hx-* attributes in it, which send
// requests without any script: a strict Content-Security-Policy does not
// stop them. Because the zero [Config] swaps 4xx and 5xx responses too,
// this covers error bodies, where Go code most often echoes the request. A
// plain-text http.Error(w, err.Error(), http.StatusBadRequest) whose err
// quotes the input (strconv, time.Parse and url.Parse errors do) is inert
// in a browser tab but becomes part of the page when htmx swaps it, and an
// injected element such as
//
//	<i hx-post="/archive" hx-include="[name=csrf]" hx-trigger="load"></i>
//
// then sends a same-origin request with the page's CSRF token. On routes
// that htmx requests, never write unescaped request data into an error
// body: answer errors with [github.com/protolambda/webui/web.Errors], whose
// messages never contain request data; wrap handlers that write their own
// error bodies with [github.com/protolambda/webui/web.Errors.Intercept],
// which answers their non-HTML errors to htmx requests with the alert of
// their status; or set [Config.NoErrorSwap] for pages whose htmx requests
// reach handlers that neither covers. See the security note at
// [DefaultResponseHandling].
//
// # Scripting attributes
//
// Some htmx features are JavaScript in attributes: [On] (hx-on:*), hx-vals
// and hx-headers values that start with "js:" or "javascript:", and
// hx-trigger event filters in square brackets. HTML escaping does not
// protect a JavaScript context, so never build them from untrusted input.
// htmx evaluates them only with [Config.AllowEval], and the browser only
// with a Content-Security-Policy that allows 'unsafe-eval'; webui's defaults
// allow neither. Without them, hx-on and "js:" values do nothing, but htmx
// drops an event filter and triggers on every matching event (see
// [Trigger]). The helpers stay available for applications that opt in.
//
// # Extensions
//
// Server-sent events and WebSockets are htmx 2 extensions: enable them with
// [Ext]("sse") or [Ext]("ws") on an ancestor, and use [SSEConnect],
// [SSESwap], [SSEClose] or [WSConnect], [WSSend]. webui's theme assets
// include the sse extension script by default; the ws extension script is
// opt-in.
//
// # Request and response headers
//
// [ParseRequestHeaders] reads the headers that htmx sends with a request
// into [RequestHeaders], for example to answer an htmx request with a
// fragment and other requests with a full page:
//
//	hx2.AddVary(w.Header())
//	if hx2.ParseRequestHeaders(r.Header).Request { ... }
//
// It decodes the values that htmx sends as Latin-1 bytes or percent-encoded
// (for characters above U+00FF), so that ids, names and prompt answers read
// as the user wrote them. The client sets these headers: use them to choose
// a representation, never for authorization. [AddVary] adds HX-Request and
// HX-History-Restore-Request, the headers that decide whether htmx handles
// an answer, to the Vary header of a response that depends on them, so
// that caches keep the representations apart, and [AddVaryHeaders] adds
// other names too, such as HX-Boosted for a response that boosted requests
// get in full:
//
//	hx2.AddVary(w.Header())
//	hx2.AddVaryHeaders(w.Header(), hx2.HeaderBoosted)
//
// The Set functions write the response headers that htmx reads, on the header
// of a response before it is written: [SetRedirect], [SetRefresh],
// [SetLocation], [SetPushURL], [SetReplaceURL], [SetReswap], [SetRetarget],
// [SetReselect], and [SetTrigger], [SetTriggerAfterSwap], and
// [SetTriggerAfterSettle] with [Event] values. Each replaces an existing
// value of its header. Plain values are set as given: URLs must be
// percent-encoded, and no value may contain control characters; JSON values
// are encoded with ASCII escapes. htmx ignores these headers on 3xx
// responses. The Header constants name every header, and
// [StatusStopPolling] is the status of an answer that stops a poll. A
// redirect or location taken from the request must be checked to be local
// before it is set, or it is an open redirect.
//
// # Changes from htmx 1
//
//   - Event binding uses [On] with hx-on:event syntax (kebab-case event names).
//   - SSE and WebSocket moved to extensions, with the attributes above
//     instead of hx-sse and hx-ws.
//   - DELETE requests send parameters in the URL, not a form-encoded body,
//     by default; the zero [Config] keeps the htmx 1 behavior, a body (see
//     [Config.DeleteURLParams]).
//   - Requests to other origins are disabled by default.
//
// Reference: https://htmx.org/reference/
//
// Migration guide: https://htmx.org/migration-guide-htmx-1/
package hx2
