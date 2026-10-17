package mw_test

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"time"
)

// pipeListener is an in-memory listener whose connections are net.Pipe
// pairs. Inside a synctest bubble its connections block durably and their
// deadlines follow the bubble's clock, so servers can be tested without real
// sockets or sleeps. net.Pipe is unbuffered: a write blocks until the peer
// reads, which models a client that stops reading.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

// Dial connects to the listener and returns the client end.
func (l *pipeListener) Dial() (net.Conn, error) {
	server, client := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.closed:
		_ = server.Close()
		_ = client.Close()
		return nil, net.ErrClosed
	}
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// serve runs a server for h on a new pipe listener and returns the listener
// and a stop function, which shuts the server down and waits for its
// connections to end (inside a bubble, time stops advancing once the test
// function returns, so nothing may still wait for a timer then).
func serve(h http.Handler) (*pipeListener, func()) {
	ln := newPipeListener()
	srv := &http.Server{Handler: h}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ln)
	}()
	return ln, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
		}
		<-done
	}
}

// deadlineRecorder is a response recorder with connection deadlines, which it
// records.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	mu       sync.Mutex
	reads    []time.Time
	writes   []time.Time
	hijacked bool
	// noHijack makes Hijack fail, as on HTTP/2.
	noHijack bool
}

func newDeadlineRecorder() *deadlineRecorder {
	return &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (d *deadlineRecorder) SetReadDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reads = append(d.reads, t)
	return nil
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writes = append(d.writes, t)
	return nil
}

func (d *deadlineRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.noHijack {
		return nil, nil, http.ErrNotSupported
	}
	if d.hijacked {
		return nil, nil, http.ErrHijacked
	}
	d.hijacked = true
	server, client := net.Pipe()
	_ = client.Close()
	return server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)), nil
}

func (d *deadlineRecorder) EnableFullDuplex() error { return nil }

func (d *deadlineRecorder) Reads() []time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.reads)
}

func (d *deadlineRecorder) Writes() []time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.writes)
}

// captureHandler is a slog.Handler that keeps its records, with the attributes
// of With calls added.
type captureHandler struct {
	sink  *recordSink
	attrs []slog.Attr
	level slog.Level
}

type recordSink struct {
	mu      sync.Mutex
	records []slog.Record
}

func newCapture(level slog.Level) (*slog.Logger, *recordSink) {
	sink := &recordSink{}
	return slog.New(&captureHandler{sink: sink, level: level}), sink
}

func (h *captureHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	r = r.Clone()
	r.AddAttrs(h.attrs...)
	h.sink.mu.Lock()
	defer h.sink.mu.Unlock()
	h.sink.records = append(h.sink.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return &captureHandler{sink: h.sink, attrs: append(slices.Clone(h.attrs), as...), level: h.level}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (s *recordSink) Records() []slog.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.records)
}

// attrs returns the attributes of a record as strings.
func attrs(r slog.Record) map[string]string {
	out := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		out[a.Key] = a.Value.String()
		return true
	})
	return out
}

// errPanicked is the value test handlers panic with.
var errPanicked = errors.New("handler panicked")
