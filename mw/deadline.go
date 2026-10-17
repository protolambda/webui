package mw

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Default bounds for [BodyDeadline] and [WriteDeadline].
const (
	DefaultBodyTimeout  = 30 * time.Second
	DefaultWriteTimeout = 30 * time.Second
)

// BodyDeadline bounds the time a request takes to deliver its body: for
// requests with a body, it sets the connection's read deadline timeout after
// the request reaches it. Without it, a client that announces a body and
// stalls holds its connection and handler forever, also on paths that answer
// without reading the body (the server reads and discards a small unread body
// before it reuses the connection, and that read then ends at the deadline).
//
// The deadline bounds the body only. Once the body is read to its end, net/http
// reads an HTTP/1 connection without a deadline again (to notice when the
// client goes away), and on HTTP/2 the deadline concerns the stream's body
// alone, so long responses and event streams outlast it. Requests without a
// body are left alone: their connections are watched from the start, and a
// deadline would end their responses.
//
// Use it instead of [http.Server.ReadTimeout], which would also bound
// long-lived responses. A handler that expects a slow upload extends the
// deadline itself with [http.ResponseController.SetReadDeadline]. Writers
// without a connection, such as httptest.ResponseRecorder, are left without a
// deadline. BodyDeadline panics if timeout is not positive.
func BodyDeadline(timeout time.Duration) Middleware {
	if timeout <= 0 {
		panic("mw: BodyDeadline needs a positive timeout")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.Body != http.NoBody {
				// The error is http.ErrNotSupported for writers without a
				// connection; there is nothing to bound then.
				_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(timeout))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WriteDeadline bounds the time to send a response: it sets the connection's
// write deadline timeout after the response starts (its header, or its first
// write or flush). A client that stops reading then loses its connection (on
// HTTP/2, the stream) at the deadline, instead of holding the handler and the
// response's buffers.
//
// The deadline starts with the response, so the handler's work before it does
// not count. For an HTTP/1 request that expects 100 Continue, it is also set
// when the request arrives, for that interim response, which the server
// writes when the handler first reads the body; such a handler reads the body
// within timeout or takes the deadline over. (On HTTP/1 a deadline bounds
// writes only, while on HTTP/2 it is a timer that resets the stream, which is
// why it is not started earlier there.)
//
// Responses that stream, such as server-sent events, set their own deadline,
// typically one per event: a handler that calls
// [http.ResponseController.SetWriteDeadline] takes the deadline over, and
// WriteDeadline sets none while it runs. A handler must do so before it waits
// between writes for longer than timeout.
//
// When the handler returns, WriteDeadline sets the deadline once more,
// timeout from then, for what the server still writes (buffered data, the end
// of a chunked body), also after a take-over: a stream's last per-event
// deadline may have passed while it waited. It sets none after a hijack.
// net/http clears the deadline of an HTTP/1 connection after each response.
//
// Use it instead of [http.Server.WriteTimeout], which counts from the request
// and cannot be taken over by a stream. The wrapper implements Unwrap, Flush,
// FlushError and Hijack, so [http.ResponseController] works through it.
// Writers without a connection are left without a deadline. WriteDeadline
// panics if timeout is not positive.
func WriteDeadline(timeout time.Duration) Middleware {
	if timeout <= 0 {
		panic("mw: WriteDeadline needs a positive timeout")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dw := &deadlineWriter{ResponseWriter: w, timeout: timeout}
			if r.ProtoMajor == 1 && expectsContinue(r) {
				dw.set()
			}
			next.ServeHTTP(dw, r)
			dw.end()
		})
	}
}

// expectsContinue reports whether a request asks for 100 Continue before it
// sends its body.
func expectsContinue(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody &&
		strings.Contains(strings.ToLower(r.Header.Get("Expect")), "100-continue")
}

// deadlineWriter sets the write deadline of a response when it starts, unless
// the handler took the deadline over or hijacked the connection. Its methods
// may be called from any goroutine that the handler writes from.
type deadlineWriter struct {
	http.ResponseWriter
	timeout time.Duration
	// done is set once the response started, the handler set a deadline, or
	// the connection was hijacked.
	done atomic.Bool
	// hijacked is set once the handler took the connection.
	hijacked atomic.Bool
}

var (
	_ http.Flusher  = (*deadlineWriter)(nil)
	_ http.Hijacker = (*deadlineWriter)(nil)
	_ io.ReaderFrom = (*deadlineWriter)(nil)
)

// set sets the deadline timeout from now.
func (w *deadlineWriter) set() {
	// The error is http.ErrNotSupported for writers without a connection.
	_ = http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Now().Add(w.timeout))
}

// start sets the deadline of the response, once, unless done.
func (w *deadlineWriter) start() {
	if w.done.CompareAndSwap(false, true) {
		w.set()
	}
}

// end sets the deadline for the rest of the response after the handler
// returned, unless it hijacked the connection.
func (w *deadlineWriter) end() {
	w.done.Store(true)
	if !w.hijacked.Load() {
		w.set()
	}
}

func (w *deadlineWriter) WriteHeader(code int) {
	w.start()
	w.ResponseWriter.WriteHeader(code)
}

func (w *deadlineWriter) Write(b []byte) (int, error) {
	w.start()
	return w.ResponseWriter.Write(b)
}

// ReadFrom starts the response and copies src to it, keeping the underlying
// writer's ReadFrom (sendfile) when it has one.
func (w *deadlineWriter) ReadFrom(src io.Reader) (int64, error) {
	w.start()
	return io.Copy(w.ResponseWriter, src)
}

// FlushError starts the response and flushes it.
func (w *deadlineWriter) FlushError() error {
	w.start()
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// Flush is FlushError without the error, for [http.Flusher].
func (w *deadlineWriter) Flush() { _ = w.FlushError() }

// SetWriteDeadline sets the deadline for a handler, which takes it over.
func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.done.Store(true)
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
}

// Hijack hands the connection to the handler; WriteDeadline sets no deadline
// after that (net/http clears the deadlines of hijacked connections).
func (w *deadlineWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked.Store(true)
		w.done.Store(true)
	}
	return c, rw, err
}

// Unwrap returns the underlying writer, for [http.ResponseController].
func (w *deadlineWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// MaxBody bounds request bodies to n bytes with [http.MaxBytesReader]: a read
// beyond the bound fails with an [*http.MaxBytesError], which handlers should
// answer with 413 Content Too Large, and the server closes the connection
// after the response. A nested MaxBody can lower the bound but not raise it,
// so routes that accept more, such as uploads, are mounted outside of it,
// with a MaxBody of their own. A bound of 0 refuses every non-empty body.
// MaxBody panics if n is negative.
func MaxBody(n int64) Middleware {
	if n < 0 {
		panic("mw: MaxBody needs a non-negative bound")
	}
	return func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, n)
	}
}
