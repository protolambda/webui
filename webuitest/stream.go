package webuitest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/mustbe/assertion"
)

// DefaultStreamTimeout is how long a [Stream] waits for a response header,
// an event or the end of the stream, unless [StreamTimeout] says
// otherwise.
const DefaultStreamTimeout = 10 * time.Second

// StreamOption configures a [Stream].
type StreamOption func(*streamConfig)

type streamConfig struct {
	timeout time.Duration
}

// StreamTimeout sets how long the stream waits for a response header, an
// event or its end; d <= 0 means [DefaultStreamTimeout].
//
// Inside a testing/synctest bubble the wait is in fake time, which advances
// only when every goroutine of the bubble is blocked: a wait then fails
// exactly when the handler waits longer than d (for a keep-alive interval,
// a timer or a lifetime), so choose d above the handler's longest wait, or
// advance time (time.Sleep in the test) before waiting.
func StreamTimeout(d time.Duration) StreamOption {
	return func(c *streamConfig) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// errClosed is the end of a stream that the test closed.
var errClosed = errors.New("the stream was closed")

// Stream reads the events of a live text/event-stream response as a
// browser's EventSource does, for [ServeStream] and [Browser.Stream]. A
// reader goroutine parses the body with the rules of [ParseEvents] and
// hands over one event at a time, so a test that stops reading stalls the
// stream as a stalled browser would. Keep-alive comments are counted
// ([Stream.Comments]) and skipped.
//
// Next, Expect, ExpectEnd and Returned wait at most the stream's timeout
// (see [StreamTimeout]) and until the test's context ends, and fail the
// test otherwise. A Stream is meant for the test goroutine; Close may be
// called from any goroutine.
type Stream struct {
	desc    string
	resp    *http.Response
	timeout time.Duration

	events   chan Event
	done     chan struct{} // closed by Close
	err      error         // why the body ended; written before events is closed
	comments atomic.Int64

	closeOnce sync.Once
	closeFn   func()

	// For ServeStream: closed when the handler returned; panicValue and
	// panicStack are written before.
	handlerDone <-chan struct{}
	panicValue  any
	panicStack  []byte
	reported    bool
}

// newStream starts reading body.
func newStream(desc string, resp *http.Response, body io.Reader, timeout time.Duration, closeFn func()) *Stream {
	s := &Stream{
		desc:    desc,
		resp:    resp,
		timeout: timeout,
		events:  make(chan Event),
		done:    make(chan struct{}),
		closeFn: closeFn,
	}
	go s.read(body)
	return s
}

// read parses the body until it ends, the stream is closed, or it fails.
func (s *Stream) read(body io.Reader) {
	defer close(s.events)
	p := eventParser{max: MaxEventSize}
	emit := func(e Event) bool {
		// Count the comments before the event before handing it over.
		s.comments.Store(int64(p.comments))
		select {
		case s.events <- e:
			return true
		case <-s.done:
			return false
		}
	}
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			ferr := p.feed(buf[:n], emit)
			s.comments.Store(int64(p.comments))
			if ferr != nil {
				if errors.Is(ferr, errStopped) {
					ferr = errClosed
				}
				s.err = ferr
				return
			}
		}
		if err != nil {
			select {
			case <-s.done:
				s.err = errClosed
				return
			default:
			}
			if errors.Is(err, io.EOF) {
				s.err = p.end()
			} else {
				s.err = fmt.Errorf("read the stream: %w", err)
			}
			return
		}
	}
}

// Response returns the response of the stream: its status and header.
// Its Body belongs to the stream and must not be read.
func (s *Stream) Response() *http.Response { return s.resp }

// Comments returns the number of comment lines (such as keep-alives) read
// so far.
func (s *Stream) Comments() int { return int(s.comments.Load()) }

// Close ends the stream from the client side, as a browser that leaves the
// page: it cancels the request context and closes the connection. It is
// idempotent and does not wait for the handler; see [Stream.Returned].
func (s *Stream) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		if s.closeFn != nil {
			s.closeFn()
		}
	})
}

// errEnded is the clean end of a stream: the body ended at an event
// boundary.
var errEnded = errors.New("the stream ended")

// next waits for the next event. The error describes why there is none:
// errEnded, or another end, a timeout or the end of ctx.
func (s *Stream) next(ctx context.Context) (Event, error) {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case e, ok := <-s.events:
		if !ok {
			return Event{}, s.endErr()
		}
		return e, nil
	case <-timer.C:
		return Event{}, fmt.Errorf("no event within %v", s.timeout)
	case <-ctx.Done():
		return Event{}, fmt.Errorf("the test context ended: %w", ctx.Err())
	}
}

// endErr describes why the body ended. It must only be called after events
// was closed, which orders it after the reader's write of s.err.
func (s *Stream) endErr() error {
	switch {
	case s.err == nil:
		return errEnded
	case errors.Is(s.err, ErrUnterminated):
		return errors.New("the stream ended inside an event")
	}
	return s.err
}

// checkPanic fails the test if the handler panicked (ServeStream only).
func (s *Stream) checkPanic(t TB) bool {
	t.Helper()
	if s.handlerDone == nil || s.reported {
		return false
	}
	select {
	case <-s.handlerDone:
	default:
		return false
	}
	if s.panicValue == nil {
		return false
	}
	s.reported = true
	fatalf(t, "webuitest: %s: the handler panicked: %v\n%s", s.desc, s.panicValue, s.panicStack)
	return true
}

// Next returns the next event, and fails the test if the stream ends or no
// event arrives within the timeout.
func (s *Stream) Next(t TB) Event {
	t.Helper()
	e, err := s.next(t.Context())
	if err != nil {
		if !s.checkPanic(t) {
			fatalf(t, "webuitest: %s: want an event: %v", s.desc, err)
		}
		return Event{}
	}
	return e
}

// Expect returns the next event and fails the test unless it is named
// name.
func (s *Stream) Expect(t TB, name string) Event {
	t.Helper()
	e, err := s.next(t.Context())
	switch {
	case err != nil:
		if !s.checkPanic(t) {
			fatalf(t, "webuitest: %s: want event %q: %v", s.desc, name, err)
		}
		return Event{}
	case e.Name != name:
		fatalf(t, "webuitest: %s: got event %q, want %q", s.desc, e.Name, name)
		return Event{}
	}
	return e
}

// ExpectEnd waits for the server to end the stream cleanly, after comments
// at most, and fails the test if an event arrives, the stream ends inside
// an event or fails, or it does not end within the timeout.
func (s *Stream) ExpectEnd(t TB) {
	t.Helper()
	e, err := s.next(t.Context())
	switch {
	case err == nil:
		fatalf(t, "webuitest: %s: got event %q, want the end of the stream", s.desc, e.Name)
	case s.checkPanic(t):
	case !errors.Is(err, errEnded):
		fatalf(t, "webuitest: %s: want the end of the stream: %v", s.desc, err)
	}
}

// Returned waits for the handler of a [ServeStream] stream to return, for
// tests of how a stream ends (for example after [Stream.Close]), and fails
// the test if it does not return within the timeout or panicked. For a
// [Browser] stream it fails: the handler runs in another server.
func (s *Stream) Returned(t TB) {
	t.Helper()
	if s.handlerDone == nil {
		fatalf(t, "webuitest: %s: Returned needs a stream of ServeStream", s.desc)
		return
	}
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case <-s.handlerDone:
		s.checkPanic(t)
	case <-timer.C:
		fatalf(t, "webuitest: %s: the handler did not return within %v", s.desc, s.timeout)
	case <-t.Context().Done():
		fatalf(t, "webuitest: %s: the test context ended before the handler returned", s.desc)
	}
}

// Status asserts the status code of the stream's response.
func (s *Stream) Status(code int) assertion.Assertion {
	return streamCheck{desc: fmt.Sprintf("status(%s, %d)", s.desc, code), check: func() error {
		if s.resp.StatusCode != code {
			return fmt.Errorf("%w: %s has status %d, want %d", ct.ErrMismatch, s.desc, s.resp.StatusCode, code)
		}
		return nil
	}}
}

// Header asserts that a value of the response header name matches want.
// Values of Set-Cookie are not shown in the failure.
func (s *Stream) Header(name string, want ct.ValueMatch) assertion.Assertion {
	return streamCheck{desc: fmt.Sprintf("header(%s, %q, %s)", s.desc, name, want), check: func() error {
		values := s.resp.Header.Values(name)
		for _, v := range values {
			if want.Match(v) {
				return nil
			}
		}
		shown := fmt.Sprintf("%q", values)
		if http.CanonicalHeaderKey(name) == "Set-Cookie" {
			shown = fmt.Sprintf("%d values", len(values))
		}
		return fmt.Errorf("%w: header %s of %s is %s, want %s", ct.ErrMismatch, name, s.desc, shown, want)
	}}
}

type streamCheck struct {
	desc  string
	check func() error
}

func (c streamCheck) String() string              { return c.desc }
func (c streamCheck) Check(context.Context) error { return c.check() }

// ServeStream runs a streaming handler, such as a server-sent event
// stream, in a goroutine over an in-memory connection, and returns a
// [Stream] of its response once the handler has written the response
// header (or returned). No network is involved, only net.Pipe and the
// time package, so it works inside testing/synctest bubbles, where the
// handler's timers run in fake time.
//
// The handler gets a ResponseWriter that buffers like net/http's (4 KiB)
// and implements Flush, FlushError and SetWriteDeadline (enforced on the
// connection: a write that the client does not read in time fails), so
// http.ResponseController works; SetReadDeadline is accepted and does
// nothing, as the request body is in memory. Without a Content-Type, the
// first write sniffs one; HEAD responses discard the body, and 204 and 304
// responses refuse one with http.ErrBodyNotAllowed.
//
// req's context is wrapped: [Stream.Close] cancels it and closes the
// connection, as a browser leaving the page does, and it is canceled when
// the handler returns. At the end of the test the stream is closed, and
// the test fails if the handler does not return within the timeout or
// panicked. A handler that panics with http.ErrAbortHandler just ends the
// stream, abruptly.
func ServeStream(t TB, h http.Handler, req *http.Request, opts ...StreamOption) *Stream {
	t.Helper()
	if h == nil || req == nil {
		fatalf(t, "webuitest.ServeStream: nil handler or request")
		return nil
	}
	cfg := streamConfig{timeout: DefaultStreamTimeout}
	for _, o := range opts {
		o(&cfg)
	}
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	srv, cli := net.Pipe()
	w := &streamWriter{
		conn:        srv,
		buf:         bufio.NewWriterSize(srv, 4096),
		header:      make(http.Header),
		head:        req.Method == http.MethodHead,
		headerReady: make(chan struct{}),
	}
	handlerDone := make(chan struct{})
	s := &Stream{
		desc:        req.Method + " " + req.URL.RequestURI(),
		timeout:     cfg.timeout,
		events:      make(chan Event),
		done:        make(chan struct{}),
		handlerDone: handlerDone,
		closeFn: func() {
			cancel()
			_ = cli.Close()
		},
	}
	go func() {
		aborted := true
		defer func() {
			if aborted {
				if v := recover(); v != nil && v != http.ErrAbortHandler {
					s.panicValue, s.panicStack = v, debug.Stack()
				}
			}
			// The handler returned: as net/http does, cancel the request
			// context, then send what is buffered and close the connection.
			// The stream's reader sees the end only after handlerDone.
			if !aborted {
				w.WriteHeader(http.StatusOK)
			}
			cancel()
			close(handlerDone)
			w.finish(aborted)
		}()
		h.ServeHTTP(w, req)
		aborted = false
	}()
	t.Cleanup(func() {
		s.Close()
		timer := time.NewTimer(cfg.timeout)
		defer timer.Stop()
		select {
		case <-handlerDone:
			if s.panicValue != nil && !s.reported {
				t.Error(fmt.Sprintf("webuitest: %s: the handler panicked: %v\n%s", s.desc, s.panicValue, s.panicStack))
			}
		case <-timer.C:
			t.Error(fmt.Sprintf("webuitest: %s: the handler did not return within %v after the client left", s.desc, cfg.timeout))
		}
	})
	timer := time.NewTimer(cfg.timeout)
	defer timer.Stop()
	select {
	case <-w.headerReady:
	case <-handlerDone:
	case <-timer.C:
		s.Close()
		fatalf(t, "webuitest: %s: the handler wrote no response header within %v", s.desc, cfg.timeout)
		return nil
	case <-t.Context().Done():
		s.Close()
		fatalf(t, "webuitest: %s: the test context ended before the response header", s.desc)
		return nil
	}
	status, header, ok := w.response()
	if !ok {
		// The handler panicked before it wrote a header: net/http would
		// close the connection without a response.
		s.Close()
		if !s.checkPanic(t) {
			fatalf(t, "webuitest: %s: the handler aborted the response", s.desc)
		}
		return nil
	}
	s.resp = &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Body:       http.NoBody,
		Request:    req,
	}
	go s.read(cli)
	return s
}

// streamWriter is the ResponseWriter of [ServeStream]. mu guards the
// response header state and is never held during I/O, so ServeStream can
// read the header while a flush waits for the reader; ioMu serializes the
// body writes.
type streamWriter struct {
	mu          sync.Mutex
	header      http.Header
	head        bool
	wrote       bool
	status      int
	snapshot    http.Header
	headerReady chan struct{}

	ioMu sync.Mutex
	conn net.Conn
	buf  *bufio.Writer
	err  error
}

var (
	_ http.ResponseWriter = (*streamWriter)(nil)
	_ http.Flusher        = (*streamWriter)(nil)
)

func (w *streamWriter) Header() http.Header { return w.header }

func (w *streamWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeHeader(code)
}

// writeHeader records the response header; w.mu must be held.
func (w *streamWriter) writeHeader(code int) {
	if w.wrote {
		return
	}
	if code < 100 || code > 999 {
		// As net/http does.
		panic(fmt.Sprintf("invalid WriteHeader code %v", code))
	}
	if code < 200 && code != http.StatusSwitchingProtocols {
		// Informational responses precede the response; they are not kept.
		return
	}
	w.wrote = true
	w.status = code
	w.snapshot = w.header.Clone()
	close(w.headerReady)
}

// started writes the header with status 200 if it was not written, and
// returns the status. Without a Content-Type, it sniffs one from p.
func (w *streamWriter) started(p []byte) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.wrote {
		if w.header.Get("Content-Type") == "" && len(p) > 0 {
			w.header.Set("Content-Type", http.DetectContentType(p))
		}
		w.writeHeader(http.StatusOK)
	}
	return w.status
}

func bodyAllowed(status int) bool {
	return status != http.StatusNoContent && status != http.StatusNotModified && (status < 100 || status >= 200)
}

func (w *streamWriter) Write(p []byte) (int, error) {
	status := w.started(p)
	if w.head {
		return len(p), nil
	}
	if !bodyAllowed(status) {
		return 0, http.ErrBodyNotAllowed
	}
	w.ioMu.Lock()
	defer w.ioMu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.buf.Write(p)
	if err != nil {
		w.err = err
	}
	return n, err
}

// FlushError writes the buffered body to the connection.
func (w *streamWriter) FlushError() error {
	w.started(nil)
	w.ioMu.Lock()
	defer w.ioMu.Unlock()
	if w.err != nil {
		return w.err
	}
	if err := w.buf.Flush(); err != nil {
		w.err = err
		return err
	}
	return nil
}

// Flush is FlushError without the error.
func (w *streamWriter) Flush() { _ = w.FlushError() }

// SetWriteDeadline sets the deadline of the connection's writes.
func (w *streamWriter) SetWriteDeadline(t time.Time) error { return w.conn.SetWriteDeadline(t) }

// SetReadDeadline does nothing: the request body is in memory.
func (w *streamWriter) SetReadDeadline(time.Time) error { return nil }

// response returns the status and header, once written.
func (w *streamWriter) response() (int, http.Header, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status, w.snapshot, w.wrote
}

// finish ends the response when the handler returned: it flushes the body
// and closes the connection. An aborted response (a panic) is cut off
// without a flush.
func (w *streamWriter) finish(aborted bool) {
	w.ioMu.Lock()
	defer w.ioMu.Unlock()
	if !aborted && w.err == nil {
		_ = w.buf.Flush()
	}
	_ = w.conn.Close()
}
