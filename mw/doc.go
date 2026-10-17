// Package mw holds net/http middleware for server-rendered web user interfaces:
// security headers with a typed Content-Security-Policy builder, the check of
// the Host header against configured sites, a same-origin policy for
// state-changing requests (a CSRF layer), body and response deadlines, a body
// size limit, and request logging with a context logger.
//
// The package depends on the standard library only. Every middleware is a
// [Middleware], a func(http.Handler) http.Handler, and [Chain] composes them.
//
// # Order
//
// A typical stack, outermost first:
//
//	handler := mw.Chain(mux,
//		mw.LogRequests(mw.RequestLog{Routes: mux}),
//		mw.BodyDeadline(mw.DefaultBodyTimeout),
//		mw.WriteDeadline(mw.DefaultWriteTimeout),
//		mw.SecurityHeaders(mw.Headers{}),
//		mw.HostCheck(site),
//		mw.SameOrigin(mw.OriginPolicy{}),
//		mw.MaxBody(1<<20),
//	)
//
// The log sees every response, rejections included. The deadlines bound the
// rejections too: a refused request with a stalled body would otherwise hold
// its connection. The security headers are set before any rejection is
// written, so error responses carry them. [HostCheck] must run before
// [SameOrigin], which compares requests with the site that HostCheck matched.
//
// # Origins come from configuration
//
// The scheme and host of a site are configured ([ParseSite]), never taken from
// the request: behind a TLS-terminating proxy a request says http although the
// browser used https, and the Host header is chosen by the client. HostCheck
// refuses Host headers that name no configured site, which keeps the origin
// comparisons of SameOrigin meaningful under DNS rebinding.
//
// # Response writers
//
// The middleware that wraps the [http.ResponseWriter] ([WriteDeadline] and
// [LogRequests]) implements Unwrap, Flush, FlushError and Hijack, so
// [http.ResponseController] and the [http.Flusher] and [http.Hijacker]
// assertions keep working through it. Other optional interfaces are reached
// with http.ResponseController.
package mw
