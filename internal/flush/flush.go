// Package flush tells whether a response can flush behind the response
// writer wrappers of web and mw, for the check that an sse stream makes
// before it starts.
//
// A wrapper that tracks a response (whether it started, its status, its
// deadline) has a FlushError method, to see flushes, which passes the
// flush on to the writer it wraps through [http.ResponseController]. The
// method says nothing about whether the response can flush: the
// controller stops at the first FlushError it finds, so a check of the
// types alone would accept the wrapper, and only the flush itself reports
// [http.ErrNotSupported], after the header went out. [Supported] looks
// past such wrappers ([Forwarder]) to the writer that does the flush.
package flush

import "net/http"

// Forwarder is a response writer wrapper whose Flush and FlushError only
// pass the flush on to the writer that Unwrap returns, through
// [http.ResponseController], besides bookkeeping of their own: the wrapper
// can flush exactly when that writer can. ForwardsFlush marks such
// wrappers and does nothing; it is exported because the wrappers are
// types of other packages.
type Forwarder interface {
	Unwrap() http.ResponseWriter
	ForwardsFlush()
}

// maxUnwrap bounds the walk through wrapped writers, in case of a cycle.
const maxUnwrap = 32

// Supported reports whether [http.ResponseController.Flush] of w reaches a
// writer that can flush. It takes the walk of the controller, through
// Unwrap methods to the first writer with a FlushError or Flush method,
// except that it walks on past a [Forwarder], whose flush is that of the
// writer it wraps. It looks at types only, so it writes nothing. A cycle
// of Unwrap methods, which would make the controller loop forever, cannot
// flush.
func Supported(w http.ResponseWriter) bool {
	for range maxUnwrap {
		switch t := w.(type) {
		case Forwarder:
			w = t.Unwrap()
		case interface{ FlushError() error }, http.Flusher:
			return true
		case interface{ Unwrap() http.ResponseWriter }:
			w = t.Unwrap()
		default:
			return false
		}
	}
	return false
}
