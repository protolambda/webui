// Package sse streams server-sent events whose data is rendered chord HTML,
// for live regions driven by the htmx SSE extension.
//
// A [Stream] writes the text/event-stream format over an
// [http.ResponseWriter]. [Stream.Send] renders a fragment into a buffer
// first, so a fragment that fails to render sends nothing, then frames it as
// one event: an event field with the event name, one data field per line of
// the HTML, and a blank line:
//
//	event: tasks
//	data: <ul><li>Write the docs</li>
//	data: <li>Review the notes</li></ul>
//
// Each write sets its own write deadline (see [WithWriteWindow]) and flushes,
// so a stream outlives a server's WriteTimeout and per-response deadlines set
// by middleware, while a stalled client is still cut off.
//
// [Loop] drives a stream from change notifications: it sends the current
// state at the start, after changes and, for views that change with time
// such as clocks, periodically ([Loop.Every]), coalesced to at most one
// update per interval; it sends keep-alive comments, caps the lifetime of
// the stream, and implements the end-versus-reconnect contract below. [Limiter] bounds
// concurrent streams per key and in total. [Region], [Connect] and [Swap]
// render the htmx attributes of the page side.
//
// # Ending a stream
//
// A live region reconnects without end. When a stream closes, the browser's
// EventSource reconnects after its retry delay (a few seconds, or what
// [Retry] set). When a connection fails (a status other than 200, or a
// response that is not an event stream), the htmx SSE extension opens a new
// one with backoff, up to about a minute apart. Only an event named by the
// sse-close attribute stops it. Two ways to finish a stream follow from
// that:
//
//   - [Stream.End] sends the [EventEnd] event. [Region] names it in both
//     sse-swap and sse-close, so its fragment (a reason and a link onwards)
//     replaces the content of the live region and the browser closes the
//     connection for good. Use it when the stream must not resume: the
//     session ended, the item is gone, the lifetime cap passed, or the
//     request is refused.
//   - Returning from the handler without End makes the browser reconnect.
//     Use it when the server shuts down, so pages resume once it is back.
//
// # Refusing a stream
//
// An EventSource follows redirects but does not show error pages: a login
// page or a 403 response is not an event stream, so the connection fails and
// the extension retries forever. Register stream routes outside redirecting
// authentication guards, check authorization in the handler, and refuse
// with a 200 stream that carries one end event saying why:
//
//	func (a *App) LiveTasks(w http.ResponseWriter, r *http.Request) error {
//		if sse.HandleHEAD(w, r) {
//			return nil
//		}
//		s := sse.New(w)
//		user, ok := a.sessions.User(r)
//		if !ok {
//			return s.End(r.Context(), views.LiveEnd("Your session ended.", "/login", "Log in again"))
//		}
//		...
//	}
//
// Check authorization before reading or rendering anything private, and
// again when it may change: pass the session's Done channel to [Loop.Done],
// which takes precedence over updates.
// Stream requests are passive; they should not extend a session's idle
// timeout.
//
// # Deadlines and keep-alive
//
// Every write first sets the connection's write deadline to now plus the
// write window (30 seconds by default) through [http.ResponseController],
// before the response header is written, and flushes after the event.
// Middleware that wraps the ResponseWriter must implement Unwrap()
// http.ResponseWriter, so the controller reaches the connection, and must
// pass flushes through (compressing or buffering middleware must skip
// text/event-stream responses).
//
// A writer that cannot flush would hold every event back, so the stream
// refuses it: the first write fails, before anything is written, with an
// error wrapping [http.ErrNotSupported] (not [ErrWrite], so it gets logged).
// A writer without deadlines is tolerated, since recorders and CGI have
// none, but the deadlines are then lost silently: behind a wrapper that
// flushes but lacks Unwrap, a stalled client is not cut off by the stream,
// and a per-response deadline set by middleware or the server's
// WriteTimeout cuts the stream when it passes, after which the page
// reconnects.
//
// On HTTP/2 a write deadline is a timer on the stream, which resets the
// stream when it passes even if the handler is only waiting. A stream must
// therefore write something more often than its write window: [Loop] sends a
// keep-alive comment every [Timing.KeepAlive], and rejects a keep-alive
// interval at or above the write window. The comments also keep proxies from
// closing idle connections. [Stream.Start] sets X-Accel-Buffering: no, which
// asks nginx-style reverse proxies not to buffer the stream.
//
// # Concurrency limits
//
// Browsers allow about six concurrent HTTP/1.1 connections per origin, and
// every open live region holds one, so many open tabs over HTTP/1.1 can stall
// ordinary requests; HTTP/2 multiplexes them. Bound streams anyone can open
// with a [Limiter]. Refusing with an end event ("too many live views open")
// stops the page from retrying; answering 429 before the stream starts lets
// the extension retry with backoff.
//
// # Views
//
// Render the first state of a live region on the server, so the page works
// without scripts, and put [Region] on its container. Children marked with
// [Swap] replace their content when their event arrives; the end event
// replaces the content of the container. Swap targets keep their element and
// only change content (the default innerHTML swap), so an aria-live
// attribute on them stays in place and screen readers announce the new
// content: use aria-live="polite" (or role="status" for a short status
// message) on targets whose changes people should hear, and leave it off
// targets that change every second, such as countdowns, which would
// interrupt constantly. Avoid outerHTML swaps on live regions: replacing the
// element itself loses its announcements. A noscript note can tell people
// without scripts to reload.
//
// # Security
//
// Event data is HTML that htmx swaps into the page. [Stream.Send] renders
// chord nodes, which escape text, so it is safe for untrusted text in
// [github.com/protolambda/chord/core/elem.Text] nodes. [Stream.SendText]
// sends its data verbatim: never send untrusted text with it to an sse-swap
// target. Stream URLs are same-origin paths ([Region] rejects others); the
// extension opens them with credentials, so keep the Content-Security-Policy
// connect-src at 'self'. Event names are restricted to a character set that
// htmx attributes can address (see [CheckEventName]). Streams do not support
// resuming with Last-Event-ID: every update re-renders whole fragments, so
// a reconnecting page needs no catch-up.
//
// # Testing
//
// The package uses only the time package for timing, so tests can drive
// [Loop] with fake time in a testing/synctest bubble.
package sse
