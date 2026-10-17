package web

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

// tracker records whether a handler started its response: wrote a final
// status or a byte of the body, flushed, or took over the connection. After
// that, an error can only be logged. It is used by the handler's goroutine
// while the handler runs, and read after the handler returns (or panics).
//
// It implements Unwrap, so [http.ResponseController] reaches the underlying
// writer (deadlines, full duplex), and Flush, FlushError, Hijack and
// ReadFrom, so the common optional interfaces keep working and are tracked.
type tracker struct {
	http.ResponseWriter
	started bool
}

var (
	_ http.Flusher  = (*tracker)(nil)
	_ http.Hijacker = (*tracker)(nil)
	_ io.ReaderFrom = (*tracker)(nil)
)

// track wraps w in a tracker, or returns w itself when it is one.
func track(w http.ResponseWriter) *tracker {
	if t, ok := w.(*tracker); ok {
		return t
	}
	return &tracker{ResponseWriter: w}
}

// maxUnwrap bounds the walk through wrapped writers, in case of a cycle.
const maxUnwrap = 32

// started reports whether the response of w started, as far as a tracker
// in the chain of wrapped writers of w knows. Without a tracker it reports
// false.
func started(w http.ResponseWriter) bool {
	for range maxUnwrap {
		if t, ok := w.(*tracker); ok {
			return t.started
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return false
		}
		w = u.Unwrap()
	}
	return false
}

func (t *tracker) WriteHeader(code int) {
	// Informational responses (1xx) may precede the final status, except
	// 101, which switches the protocol.
	if code >= 200 || code == http.StatusSwitchingProtocols {
		t.started = true
	}
	t.ResponseWriter.WriteHeader(code)
}

func (t *tracker) Write(b []byte) (int, error) {
	t.started = true
	return t.ResponseWriter.Write(b)
}

// ReadFrom copies src to the response, with the underlying writer's
// ReadFrom (sendfile) when it has one.
func (t *tracker) ReadFrom(src io.Reader) (int64, error) {
	t.started = true
	return io.Copy(t.ResponseWriter, src)
}

// FlushError flushes the response, which starts it.
func (t *tracker) FlushError() error {
	t.started = true
	return http.NewResponseController(t.ResponseWriter).Flush()
}

// Flush is FlushError without the error, for [http.Flusher].
func (t *tracker) Flush() { _ = t.FlushError() }

// Hijack hands the connection to the handler.
func (t *tracker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(t.ResponseWriter).Hijack()
	if err == nil {
		t.started = true
	}
	return c, rw, err
}

// Unwrap returns the underlying writer, for [http.ResponseController].
func (t *tracker) Unwrap() http.ResponseWriter { return t.ResponseWriter }
