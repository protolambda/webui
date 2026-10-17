// Package hx1 provides htmx 1.x attributes, for legacy pages that still
// load htmx 1.
//
// webui's theme ships htmx 2, and new code should use
// [github.com/protolambda/webui/hx2], which also provides the htmx request and
// response headers and the htmx-config meta element. This package is kept
// so that pages written for htmx 1 can move to webui unchanged; do not mix
// its htmx 1 only attributes ([SSE], [WS]) into htmx 2 pages.
//
// HTMX allows you to access modern browser features directly from HTML,
// using attributes to define AJAX requests, CSS transitions, WebSockets, and more.
// Each helper returns a chord attribute whose value chord HTML-escapes. HTML
// escaping does not make a value safe where htmx reads JavaScript: hx-vals
// and hx-headers values that start with "js:" or "javascript:", hx-vars,
// and hx-trigger event filters in square brackets. Never build those from
// untrusted input.
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
// Note: The hx-sse and hx-ws attributes are v1-only. In htmx 2, these were
// moved to extensions with different attribute names, which the hx2 package
// provides.
//
// Reference: https://v1.htmx.org/reference/
package hx1
