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
//	site := mw.MustParseSite("https://app.example.com")
//	handler := mw.Chain(mux,
//		mw.LogRequests(mw.RequestLog{Routes: mux}),
//		mw.BodyDeadline(mw.DefaultBodyTimeout),
//		mw.WriteDeadline(mw.DefaultWriteTimeout),
//		mw.SecurityHeaders(mw.Headers{HSTS: mw.HSTSFor(site)}),
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
// Application middleware that reads cookies into the request context, such
// as the session store's Middleware (package session), goes last, after
// MaxBody and right around the mux: requests that these checks refuse then
// never count as the use of a session, and every handler and per-route
// guard sees the session. An error policy's Recover and Intercept (package
// web) go first, right inside LogRequests.
//
// Strict-Transport-Security is the one header that the zero [Headers] does
// not send: [HSTSFor] turns it on for https sites (and leaves it off for
// loopback ones), unless a TLS-terminating proxy sends it already.
//
// # Refusals
//
// [HostCheck] and [SameOrigin] answer refused requests with a fixed
// plain-text body that quotes nothing of the request, served as text/plain
// with nosniff, and log them at debug level with a fixed reason. The
// package depends on no rendering, so that is its default. An application
// with webui's error policy (package web) answers SameOrigin refusals with
// it instead, through [OriginPolicy.Deny] and
// [github.com/protolambda/webui/web.Errors.Answer]: htmx requests then get
// an alert rather than text swapped into their target. The policy's
// [github.com/protolambda/webui/web.Errors.Intercept] does the same for
// the plain-text refusals of HostCheck, and for the plain-text errors of
// any other code, that reach an htmx request.
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
// with http.ResponseController. [MaxBody] follows Unwrap to the server's own
// writer, which it needs to close the connection after an oversized body.
package mw
