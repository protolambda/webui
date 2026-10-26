package mw

import "net/http"

// Mounted returns the [Router] of an inner router that an outer one serves
// at pattern without changing the request, as an application mounts its UI
// mux at the catch-all "/" of an outer mux (outer.Handle("/", ui)) that
// also routes other paths, such as health checks or an API. A request that
// outer routes to pattern is resolved by inner, every other one by outer,
// so that [RequestLog.Routes] logs the pattern of the inner mux, such as
// "GET /tasks/{id}", instead of "/" for each request of the inner mux.
// Pattern is compared with the patterns that outer reports, so it is
// spelled as it was registered with outer.
//
// It is for a catch-all mount without path stripping: inner must route
// the request that outer receives, with the same path. A mux behind
// [http.StripPrefix] sees another path than the request has, and Mounted
// would ask it about the wrong one.
//
// Mounted panics if outer or inner is nil.
func Mounted(outer Router, pattern string, inner Router) Router {
	if outer == nil || inner == nil {
		panic("mw: Mounted needs an outer and an inner router")
	}
	return mounted{outer: outer, pattern: pattern, inner: inner}
}

// mounted is the router of [Mounted].
type mounted struct {
	outer   Router
	pattern string
	inner   Router
}

// Handler resolves r with the outer router, and with the inner one when
// the outer one routes it to the mount pattern.
func (m mounted) Handler(r *http.Request) (http.Handler, string) {
	h, pattern := m.outer.Handler(r)
	if pattern != m.pattern {
		return h, pattern
	}
	return m.inner.Handler(r)
}
