package mw

import "net/http"

// Middleware wraps a handler with behavior that applies before, after or
// instead of it.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with mws. The first middleware is the outermost: it sees the
// request first and the response last, so Chain(h, a, b) is a(b(h)).
//
// Nil middlewares are skipped, which lets callers include optional layers
// inline. Chain panics if h is nil or a middleware returns a nil handler; both
// are programmer errors.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	if h == nil {
		panic("mw: Chain needs a handler")
	}
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] == nil {
			continue
		}
		h = mws[i](h)
		if h == nil {
			panic("mw: a middleware returned a nil handler")
		}
	}
	return h
}
