// Package hx2 provides HTMX v2 attributes for building hypermedia-driven applications.
//
// HTMX v2 is a major update with breaking changes from v1. This package provides
// the v2 attribute API independently from the v1 package.
//
// # Key Changes from v1
//
//   - Event binding uses [On] with hx-on:event syntax (kebab-case event names)
//   - SSE/WebSocket moved to extensions: use [Ext]("sse") or [Ext]("ws")
//   - SSE uses [SSEConnect], [SSESwap], [SSEClose] instead of hx-sse
//   - WebSocket uses [WSConnect], [WSSend] instead of hx-ws
//   - DELETE requests use URL params by default (not form-encoded body)
//   - Cross-domain requests disabled by default
//   - IE support removed
//
// # Core Request Attributes
//
//   - [Get], [Post], [Put], [Patch], [Delete]: HTTP request triggers
//
// # Common Attributes
//
//   - [Target]: CSS selector for response target
//   - [Swap]: How to swap the response into the DOM
//   - [Trigger]: Event that triggers the request
//   - [Indicator]: Element to show during request
//   - [Vals]: Additional values to submit
//
// # Event Binding (v2 syntax)
//
//   - [On]: hx-on:event-name="handler" (replaces v1's hx-on="event: handler")
//
// # Extensions
//
//   - [Ext]: Enable HTMX extensions (hx-ext attribute)
//   - [SSEConnect], [SSESwap], [SSEClose]: SSE extension attributes
//   - [WSConnect], [WSSend]: WebSocket extension attributes
//
// # Request and response headers
//
// [ParseRequestHeaders] reads the headers that htmx sends with a request
// into [RequestHeaders], for example to answer an htmx request with a
// fragment and other requests with a full page:
//
//	if hx2.ParseRequestHeaders(r.Header).Request { ... }
//
// It decodes the values that htmx sends as Latin-1 bytes or percent-encoded
// (for characters above U+00FF), so that ids, names and prompt answers read
// as the user wrote them.
//
// The Set functions write the response headers that htmx reads, on the header
// of a response before it is written: [SetRedirect], [SetRefresh],
// [SetLocation], [SetPushURL], [SetReplaceURL], [SetReswap], [SetRetarget],
// [SetReselect], and [SetTrigger], [SetTriggerAfterSwap], and
// [SetTriggerAfterSettle] with [Event] values. Each replaces an existing
// value of its header. Plain values are set as given: URLs must be
// percent-encoded, and no value may contain control characters; JSON values
// are encoded with ASCII escapes. htmx ignores these headers on 3xx
// responses. The Header constants name every header.
//
// Reference: https://htmx.org/reference/
// Migration guide: https://htmx.org/migration-guide-htmx-1/
package hx2
