package mw_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

func TestLoggerContext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	ctx := context.Background()
	t.Must(be.Same(slog.Default(), mw.Logger(ctx)))
	t.Must(be.Equal(ctx, mw.WithLogger(ctx, nil)))
	l, _ := newCapture(slog.LevelInfo)
	ctx = mw.WithLogger(ctx, l)
	t.Must(be.Same(l, mw.Logger(ctx)))
	t.Must(be.Same(l, mw.Logger(mw.WithLogger(ctx, nil))))
}

// itemsMux is an application mux for the log tests.
func itemsMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		mw.Logger(r.Context()).Info("loaded item")
		_, _ = io.WriteString(w, "item "+r.PathValue("id"))
	})
	mux.HandleFunc("POST /items", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("GET /empty", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /copy", func(w http.ResponseWriter, _ *http.Request) {
		// A LimitedReader has no WriteTo, so io.Copy uses the writer's ReadFrom.
		_, _ = io.Copy(w, io.LimitReader(strings.NewReader("copied and more"), 6))
	})
	mux.HandleFunc("GET /flush", func(w http.ResponseWriter, _ *http.Request) {
		_ = http.NewResponseController(w).Flush()
	})
	return mux
}

func TestLogRequests(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name, method, target string
		want                 map[string]string
	}{
		{"route with value", http.MethodGet, "/items/secret-handle?token=hidden",
			map[string]string{"method": "GET", "pattern": "GET /items/{id}", "status": "200", "bytes": "18"}},
		{"informational then created", http.MethodPost, "/items",
			map[string]string{"method": "POST", "pattern": "POST /items", "status": "201", "bytes": "0"}},
		{"nothing written", http.MethodGet, "/empty",
			map[string]string{"method": "GET", "pattern": "GET /empty", "status": "200", "bytes": "0"}},
		{"copy", http.MethodGet, "/copy",
			map[string]string{"method": "GET", "pattern": "GET /copy", "status": "200", "bytes": "6"}},
		{"flush", http.MethodGet, "/flush",
			map[string]string{"method": "GET", "pattern": "GET /flush", "status": "200", "bytes": "0"}},
		{"not found", http.MethodGet, "/missing/secret",
			map[string]string{"method": "GET", "pattern": "", "status": "404", "bytes": "19"}},
		{"method not allowed", http.MethodDelete, "/items/1",
			map[string]string{"method": "DELETE", "pattern": "", "status": "405", "bytes": "19"}},
		{"lower-case method", "get", "/items/1",
			map[string]string{"method": "(other)", "pattern": "", "status": "405", "bytes": "19"}},
		{"long method", "VERYLONGMETHODNAME", "/items/1",
			map[string]string{"method": "(other)", "pattern": "", "status": "405", "bytes": "19"}},
	}
	for _, c := range cases {
		for _, routed := range []bool{false, true} {
			name := c.name
			if routed {
				name += " with routes"
			}
			t.Run(name, func(t devtest.T) {
				logger, sink := newCapture(slog.LevelInfo)
				mux := itemsMux()
				cfg := mw.RequestLog{Logger: logger}
				if routed {
					cfg.Routes = mux
				}
				h := mw.LogRequests(cfg)(mux)
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.target, nil))
				records := sink.Records()
				last := records[len(records)-1]
				t.Must(be.Equal("http request", last.Message))
				t.Must(be.Equal(slog.LevelInfo, last.Level))
				got := attrs(last)
				t.Must(be.NonEmpty([]string{got["duration"]}))
				delete(got, "duration")
				t.Must(be.DeepEqual(c.want, got))
				for _, r := range records {
					for k, v := range attrs(r) {
						t.Mustf(be.False(strings.Contains(v, "secret") || strings.Contains(v, "hidden")),
							"attribute %s leaks the path or query", k)
					}
				}
			})
		}
	}
}

func TestLogRequestsScopedLogger(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	mux := itemsMux()
	h := mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux})(mux)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/7", nil))
	records := sink.Records()
	t.Must(be.Len(records, 2))
	t.Must(be.Equal("loaded item", records[0].Message))
	t.Must(be.DeepEqual(map[string]string{"method": "GET", "pattern": "GET /items/{id}"}, attrs(records[0])))
}

func TestLogRequestsContextLogger(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	h := mw.LogRequests(mw.RequestLog{})(itemsMux())
	req := httptest.NewRequest(http.MethodGet, "/items/7", nil)
	h.ServeHTTP(httptest.NewRecorder(), req.WithContext(mw.WithLogger(req.Context(), logger)))
	records := sink.Records()
	t.Must(be.Len(records, 2))
	t.Mustf(be.DeepEqual(map[string]string{"method": "GET"}, attrs(records[0])),
		"without routes, the scoped logger has no pattern")
	t.Must(be.Equal("GET /items/{id}", attrs(records[1])["pattern"]))
}

func TestLogRequestsPatternLostWithoutRoutes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	mux := itemsMux()
	site := mw.MustParseSite("https://app.example.com")
	for _, routes := range []mw.Router{nil, mux} {
		h := mw.Chain(mux, mw.LogRequests(mw.RequestLog{Logger: logger, Routes: routes}), mw.HostCheck(site))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "https://app.example.com/items/1", nil))
	}
	records := sink.Records()
	t.Must(be.Len(records, 4))
	t.Mustf(be.Equal("", attrs(records[1])["pattern"]), "HostCheck derives a new request")
	t.Must(be.Equal("GET /items/{id}", attrs(records[3])["pattern"]))
}

func TestLogRequestsPath(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	h := mw.LogRequests(mw.RequestLog{Logger: logger, Path: true})(http.NotFoundHandler())
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/a%20b/c?q=hidden", nil))
	long := "/" + strings.Repeat("x", 300)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, long, nil))
	records := sink.Records()
	t.Must(be.Equal("/a%20b/c", attrs(records[0])["path"]))
	t.Must(be.Equal(long[:256], attrs(records[1])["path"]))
}

func TestLogRequestsLevel(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	var scoped *slog.Logger
	var writer http.ResponseWriter
	h := mw.LogRequests(mw.RequestLog{Logger: logger, Level: slog.LevelDebug})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writer = w
			scoped = mw.Logger(r.Context())
		}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Len(sink.Records(), 0))
	t.Mustf(be.Equal[http.ResponseWriter](rec, writer), "a disabled level passes the writer through")
	t.Must(be.NotEqual(logger, scoped))
	scoped.Warn("still scoped")
	t.Must(be.DeepEqual(map[string]string{"method": "GET"}, attrs(sink.Records()[0])))

	logger, sink = newCapture(slog.LevelDebug)
	mw.LogRequests(mw.RequestLog{Logger: logger, Level: slog.LevelDebug})(http.NotFoundHandler()).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal(slog.LevelDebug, sink.Records()[0].Level))
}

func TestLogRequestsPanic(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	h := mw.LogRequests(mw.RequestLog{Logger: logger})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(errPanicked)
	}))
	func() {
		defer func() {
			t.Must(be.Equal[any](errPanicked, recover()))
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	got := attrs(sink.Records()[0])
	t.Must(be.Equal("true", got["panic"]))
	t.Must(be.Equal("0", got["status"]))
}

func TestLogRequestsHijack(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	h := mw.LogRequests(mw.RequestLog{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		c, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = c.Close()
		}
	}))
	h.ServeHTTP(newDeadlineRecorder(), httptest.NewRequest(http.MethodGet, "/ws", nil))
	got := attrs(sink.Records()[0])
	t.Must(be.Equal("true", got["hijacked"]))
	t.Must(be.Equal("0", got["status"]))

	rec := newDeadlineRecorder()
	rec.noHijack = true
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))
	got = attrs(sink.Records()[1])
	t.Must(be.Equal("", got["hijacked"]))
	t.Must(be.Equal("200", got["status"]))
}

func TestLogRequestsConnect(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, sink := newCapture(slog.LevelInfo)
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnel/", func(http.ResponseWriter, *http.Request) {})
	for _, routes := range []mw.Router{nil, mux} {
		h := mw.LogRequests(mw.RequestLog{Logger: logger, Routes: routes})(mux)
		req := httptest.NewRequest(http.MethodConnect, "/tunnel", nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	for _, r := range sink.Records() {
		t.Mustf(be.Equal("", attrs(r)["pattern"]), "CONNECT reports a path, not a pattern")
	}
}

// errReader is a source whose every Read returns err, and which has no
// WriteTo, so that io.Copy into a response writer calls its ReadFrom.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// writerOnly hides the methods of a writer other than Write.
type writerOnly struct{ io.Writer }

// noFlushWriter is a response writer that cannot flush.
type noFlushWriter struct{ rec *httptest.ResponseRecorder }

func (w noFlushWriter) Header() http.Header         { return w.rec.Header() }
func (w noFlushWriter) Write(b []byte) (int, error) { return w.rec.Write(b) }
func (w noFlushWriter) WriteHeader(code int)        { w.rec.WriteHeader(code) }

// brokenFlusher is a recorder whose flush fails after it wrote the status,
// as the flush of net/http on a broken connection does.
type brokenFlusher struct{ *httptest.ResponseRecorder }

func (w brokenFlusher) FlushError() error {
	w.ResponseRecorder.Flush()
	return errors.New("connection broken")
}

// readFromRecorder is a recorder with a ReadFrom, as net/http's writer has
// for sendfile, which counts the bytes it copied.
type readFromRecorder struct {
	*httptest.ResponseRecorder
	copied int64
}

func (w *readFromRecorder) ReadFrom(src io.Reader) (int64, error) {
	n, err := io.Copy(writerOnly{w.ResponseRecorder}, src)
	w.copied += n
	return n, err
}

// TestLogRequestsStatusAfterNothingSent checks that a copy of nothing and a
// flush of a writer that cannot flush do not record a status: net/http has
// sent none, and the status that follows is the real one.
func TestLogRequestsStatusAfterNothingSent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	failing := func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.Copy(w, errReader{errors.New("read failed")}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	cases := []struct {
		name    string
		handler http.HandlerFunc
		writer  func() http.ResponseWriter
		want    map[string]string
	}{
		{"failing copy", failing, nil, map[string]string{"status": "500", "bytes": "12"}},
		{"empty copy", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.Copy(w, errReader{io.EOF})
			w.WriteHeader(http.StatusNoContent)
		}, nil, map[string]string{"status": "204", "bytes": "0"}},
		{"buffered failing copy", func(w http.ResponseWriter, _ *http.Request) {
			// bufio.Reader has a WriteTo, which hands its source to the
			// ReadFrom of the writer.
			if _, err := io.Copy(w, bufio.NewReader(errReader{errors.New("read failed")})); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}, nil, map[string]string{"status": "500", "bytes": "12"}},
		{"unsupported flush", func(w http.ResponseWriter, _ *http.Request) {
			if err := http.NewResponseController(w).Flush(); !errors.Is(err, http.ErrNotSupported) {
				panic("the flush must not be supported")
			}
			w.(http.Flusher).Flush()
			http.Error(w, "busy", http.StatusServiceUnavailable)
		}, func() http.ResponseWriter { return noFlushWriter{httptest.NewRecorder()} },
			map[string]string{"status": "503", "bytes": "5"}},
		{"failed flush", func(w http.ResponseWriter, _ *http.Request) {
			_ = http.NewResponseController(w).Flush()
			http.Error(w, "late", http.StatusInternalServerError)
		}, func() http.ResponseWriter { return brokenFlusher{httptest.NewRecorder()} },
			map[string]string{"status": "200", "bytes": "5"}},
	}
	for _, c := range cases {
		logger, sink := newCapture(slog.LevelInfo)
		var w http.ResponseWriter = httptest.NewRecorder()
		if c.writer != nil {
			w = c.writer()
		}
		mw.LogRequests(mw.RequestLog{Logger: logger})(c.handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		records := sink.Records()
		t.Mustf(be.Len(records, 1), "%s: records", c.name)
		got := attrs(records[0])
		t.Mustf(be.Equal(c.want["status"], got["status"]), "%s: status", c.name)
		t.Mustf(be.Equal(c.want["bytes"], got["bytes"]), "%s: bytes", c.name)
	}

	// On a real server, whose writer starts a response with its first byte.
	logger, sink := newCapture(slog.LevelInfo)
	ln, stop := serve(mw.LogRequests(mw.RequestLog{Logger: logger})(http.HandlerFunc(failing)))
	c, err := ln.Dial()
	t.Must(be.NoError(err))
	_, err = io.WriteString(c, "GET / HTTP/1.1\r\nHost: app.example.com\r\nConnection: close\r\n\r\n")
	t.Must(be.NoError(err))
	res, err := http.ReadResponse(bufio.NewReader(c), nil)
	t.Must(be.NoError(err))
	body, err := io.ReadAll(res.Body)
	t.Must(be.NoError(err))
	_ = c.Close()
	stop()
	t.Must(be.Equal(http.StatusInternalServerError, res.StatusCode))
	t.Must(be.Equal("read failed\n", string(body)))
	records := sink.Records()
	t.Must(be.Len(records, 1))
	t.Must(be.Equal("500", attrs(records[0])["status"]))
}

// TestLogRequestsLongCopy checks that a copy keeps the ReadFrom of the
// underlying writer for all but its start, and counts every byte once.
func TestLogRequestsLongCopy(gt *testing.T) {
	t := devtest.ParallelT(gt)
	content := strings.Repeat("0123456789", 300)
	logger, sink := newCapture(slog.LevelInfo)
	w := &readFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	var n int64
	var err error
	mw.LogRequests(mw.RequestLog{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A LimitedReader has no WriteTo, so io.Copy uses the ReadFrom of w.
		n, err = io.Copy(w, io.LimitReader(strings.NewReader(content), int64(len(content))))
	})).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.NoError(err))
	t.Must(be.Equal(int64(len(content)), n))
	t.Must(be.True(w.copied > 0 && w.copied < n))
	t.Must(be.Equal(content, w.Body.String()))
	got := attrs(sink.Records()[0])
	t.Must(be.Equal("200", got["status"]))
	t.Must(be.Equal("3000", got["bytes"]))
}
