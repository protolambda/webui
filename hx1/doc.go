// Package hx1 provides HTMX v1 attributes for building hypermedia-driven applications.
//
// HTMX allows you to access modern browser features directly from HTML,
// using attributes to define AJAX requests, CSS transitions, WebSockets, and more.
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
// # Real-time (v1 only)
//
//   - [SSE]: Server-Sent Events via hx-sse (removed in v2, use extension)
//   - [WS]: WebSocket via hx-ws (removed in v2, use extension)
//
// Note: The hx-sse and hx-ws attributes are v1-only. In HTMX v2, these were
// moved to extensions with different attribute names. See [hx2] package.
//
// Reference: https://v1.htmx.org/reference/
//
// For HTMX v2, see the [hx2] package.
package hx1
