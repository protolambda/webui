package web_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/protolambda/devtest"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
)

// logs collects log records; it is safe for concurrent use.
type logs struct {
	mu      sync.Mutex
	records []record
}

// record is a log record with its attributes formatted as text.
type record struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

func (l *logs) handler() slog.Handler { return &logHandler{logs: l} }

// all returns the records so far.
func (l *logs) all() []record {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]record(nil), l.records...)
}

// text returns every record as one line of text, for checks that nothing
// leaked into the log.
func (l *logs) text() string {
	var b strings.Builder
	for _, r := range l.all() {
		fmt.Fprintf(&b, "%s %s", r.level, r.msg)
		for k, v := range r.attrs {
			fmt.Fprintf(&b, " %s=%s", k, v)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

type logHandler struct {
	logs  *logs
	attrs []slog.Attr
}

func (h *logHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *logHandler) Handle(_ context.Context, r slog.Record) error {
	rec := record{level: r.Level, msg: r.Message, attrs: make(map[string]string)}
	for _, a := range h.attrs {
		rec.attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.Resolve().String()
		return true
	})
	h.logs.mu.Lock()
	h.logs.records = append(h.logs.records, rec)
	h.logs.mu.Unlock()
	return nil
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &logHandler{logs: h.logs, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}

func (h *logHandler) WithGroup(string) slog.Handler { return h }

// request returns a request with the test's context, which carries a logger
// that records into l, or discards when l is nil.
func request(t devtest.T, l *logs, method, target string, body io.Reader) *http.Request {
	handler := slog.DiscardHandler
	if l != nil {
		handler = l.handler()
	}
	ctx := mw.WithLogger(t.Context(), slog.New(handler))
	return httptest.NewRequestWithContext(ctx, method, target, body)
}

// asHTMX marks r as an htmx request from the page at current.
func asHTMX(r *http.Request, current string) *http.Request {
	r.Header.Set(hx2.HeaderRequest, "true")
	if current != "" {
		r.Header.Set(hx2.HeaderCurrentURL, current)
	}
	return r
}

// fakeWriter is a response writer that records what happens to it, with
// optional failures, and the deadline methods of http.ResponseController.
type fakeWriter struct {
	header   http.Header
	status   int
	body     strings.Builder
	writeErr error
	deadline time.Time
	duplex   bool
}

func newFakeWriter() *fakeWriter { return &fakeWriter{header: make(http.Header)} }

func (w *fakeWriter) Header() http.Header { return w.header }

func (w *fakeWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *fakeWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.body.Write(b)
}

func (w *fakeWriter) SetWriteDeadline(t time.Time) error {
	w.deadline = t
	return nil
}

func (w *fakeWriter) EnableFullDuplex() error {
	w.duplex = true
	return nil
}
