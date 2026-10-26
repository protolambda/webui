package sse_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/web"
)

// streamFunc streams to s; it is served by [streamFunc.handle] or
// [streamFunc.plain].
type streamFunc func(r *http.Request, s *sse.Stream) error

// handle serves fn through errs.Handle, which answers its error with the
// error page unless the response started.
func (fn streamFunc) handle(errs web.Errors) http.Handler {
	return errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return fn(r, sse.New(w))
	})
}

// plain serves fn without web, answering its error with 500 when the
// stream did not start.
func (fn streamFunc) plain() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := sse.New(w)
		if err := fn(r, s); err != nil && !s.Started() {
			http.Error(w, "stream unavailable", http.StatusInternalServerError)
		}
	})
}

// recording returns fn, which also sends its error to got.
func recording(got chan<- error, fn streamFunc) streamFunc {
	return func(r *http.Request, s *sse.Stream) error {
		err := fn(r, s)
		got <- err
		return err
	}
}

// hideFlush is application middleware that wraps the response writer the
// classic way ([embedWriter], without Flush or Unwrap): the writers inside
// it cannot flush.
func hideFlush(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(embedWriter{w}, r)
	})
}

// getOnServer serves one GET request with h on a real net/http server
// and returns the response and its body. The records of h go to a logger
// that logs at info level, so that mw.LogRequests wraps the writer.
func getOnServer(t devtest.T, h http.Handler) (*http.Response, string) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(mw.WithLogger(r.Context(), quiet)))
	}))
	defer srv.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/live", nil)
	t.Must(be.NoError(err))
	res, err := srv.Client().Do(req)
	t.Must(be.NoError(err))
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	t.Must(be.NoError(err))
	return res, string(body)
}

// The writers of web and mw pass flushes on, so a stream behind them
// looks at the writer they wrap: behind application middleware that hides
// the flush, the stream fails before anything is written, and the handler
// still answers with an error instead of an event stream that delivers
// nothing.
func TestUnflushableBehindWrappers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{}
	const (
		event  = "event: tasks\ndata: Write the docs\n\n"      // of send
		events = event + "event: end\ndata: No more tasks\n\n" // of loop
		html   = "text/html; charset=utf-8"                    // of the error page
		text   = "text/plain; charset=utf-8"                   // of streamFunc.plain
		failed = "stream unavailable\n"                        // of streamFunc.plain
	)
	refusal := web.DefaultMessage(http.StatusInternalServerError) // of the error page
	update := func(ctx context.Context, s *sse.Stream) error {
		return s.Send(ctx, "tasks", elem.Text("Write the docs"))
	}
	send := func(r *http.Request, s *sse.Stream) error { return update(r.Context(), s) }
	loop := func(r *http.Request, s *sse.Stream) error {
		changes := make(chan struct{})
		close(changes) // the feed ends after the first update
		return sse.Loop{
			Update:  update,
			End:     func(sse.Reason) elem.Node { return elem.Text("No more tasks") },
			Changes: changes,
		}.Run(r.Context(), s)
	}
	cases := []struct {
		name  string
		stack func(fn streamFunc) http.Handler
		fn    streamFunc
		// events is the body of the stream; refusedType and refused are
		// the Content-Type and part of the body of the error answer.
		events, refusedType, refused string
	}{
		{"Errors.Handle with Send", func(fn streamFunc) http.Handler {
			return fn.handle(errs)
		}, send, event, html, refusal},
		{"Errors.Handle with Loop.Run", func(fn streamFunc) http.Handler {
			return fn.handle(errs)
		}, loop, events, html, refusal},
		{"Errors.Recover", func(fn streamFunc) http.Handler {
			return errs.Recover(fn.plain())
		}, send, event, text, failed},
		{"Errors.Intercept", func(fn streamFunc) http.Handler {
			return errs.Intercept(fn.plain())
		}, send, event, text, failed},
		{"LogRequests", func(fn streamFunc) http.Handler {
			return mw.LogRequests(mw.RequestLog{})(fn.plain())
		}, send, event, text, failed},
		{"WriteDeadline", func(fn streamFunc) http.Handler {
			return mw.WriteDeadline(mw.DefaultWriteTimeout)(fn.plain())
		}, send, event, text, failed},
		{"the recommended chain", func(fn streamFunc) http.Handler {
			return mw.Chain(fn.handle(errs),
				mw.LogRequests(mw.RequestLog{}),
				errs.Recover,
				errs.Intercept,
				mw.BodyDeadline(mw.DefaultBodyTimeout),
				mw.WriteDeadline(mw.DefaultWriteTimeout),
				mw.SecurityHeaders(mw.Headers{}),
			)
		}, loop, events, html, refusal},
	}
	for _, c := range cases {
		got := make(chan error, 1)
		res, body := getOnServer(t, hideFlush(c.stack(recording(got, c.fn))))
		t.Mustf(be.ErrorIs(<-got, http.ErrNotSupported), "%s: stream error", c.name)
		t.Mustf(be.Equal(http.StatusInternalServerError, res.StatusCode), "%s: status", c.name)
		t.Mustf(be.Equal(c.refusedType, res.Header.Get("Content-Type")), "%s: content type", c.name)
		t.Mustf(be.Substring(body, c.refused), "%s: body", c.name)
		t.Mustf(be.False(strings.Contains(body, "event:")), "%s: no event", c.name)

		// Without the hiding middleware, the same stack streams.
		res, body = getOnServer(t, c.stack(recording(got, c.fn)))
		t.Mustf(be.NoError(<-got), "%s: stream error", c.name)
		t.Mustf(be.Equal(http.StatusOK, res.StatusCode), "%s: status", c.name)
		t.Mustf(be.Equal(sse.ContentType, res.Header.Get("Content-Type")), "%s: content type", c.name)
		t.Mustf(be.Equal(c.events, body), "%s: events", c.name)
	}
}
