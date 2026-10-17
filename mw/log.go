package mw

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type loggerKey struct{}

// WithLogger returns a context that carries l for [Logger]. A nil l returns ctx
// unchanged, so the logger already attached (or the default) stays in effect.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// Logger returns the logger that ctx carries ([WithLogger]), or, when it
// carries none, [slog.Default] at the time of the call. It never returns nil.
func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// Router resolves the route pattern of a request without serving it.
// [*http.ServeMux] implements it.
type Router interface {
	Handler(r *http.Request) (h http.Handler, pattern string)
}

// RequestLog configures [LogRequests]. The zero value logs every request at
// info level to the context logger, with the pattern that the mux sets on the
// request it serves.
type RequestLog struct {
	// Logger receives the records and is the base of the request-scoped
	// logger. Nil means [Logger] of the request context.
	Logger *slog.Logger

	// Level is the level of the records. The zero value is slog.LevelInfo.
	Level slog.Level

	// Routes resolves the route pattern before the request is served, so the
	// request-scoped logger carries it and the record has it whatever the
	// inner layers do. Pass the application's mux. When nil, the pattern is
	// read from the request after the handler returns ([http.Request.Pattern]),
	// which the mux sets only on the request value it receives: it is lost
	// when an inner middleware derives a new request, as [HostCheck] does.
	Routes Router

	// Path adds the escaped URL path (at most 256 bytes) to the records. Off
	// by default: paths may carry capabilities, such as the random token of a
	// share link or a one-time page. The query is never logged.
	Path bool
}

// maxLoggedPath bounds the path in request records.
const maxLoggedPath = 256

// LogRequests logs one record per request, "http request", when the response
// is done, with:
//
//   - method: the method if it is 1 to 16 upper-case ASCII letters, else
//     "(other)", since any byte sequence arrives;
//   - pattern: the route pattern ("" when no route matched), never the path
//     or query, which may carry capabilities and personal data;
//   - status, bytes (of the body written) and duration;
//   - hijacked or panic, when that happened instead of a normal response (a
//     panic is logged and propagated, not recovered);
//   - path, only when cfg.Path is set.
//
// It also attaches a request-scoped logger to the request context: the base
// logger with the method and, when cfg.Routes is set, the pattern, so that
// [Logger] in handlers tags every record with the request.
//
// The wrapper of the response writer records the status and size; it
// implements Unwrap, Flush, FlushError and Hijack (see the package
// documentation). When the base logger does not log at cfg.Level, requests
// are passed through without the wrapper.
func LogRequests(cfg RequestLog) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			base := cfg.Logger
			if base == nil {
				base = Logger(ctx)
			}
			method := sanitizeMethod(r.Method)
			pattern := ""
			if cfg.Routes != nil && r.Method != http.MethodConnect {
				// For CONNECT, the mux reports a path instead of a pattern.
				_, pattern = cfg.Routes.Handler(r)
			}
			scoped := base.With(slog.String("method", method))
			if pattern != "" {
				scoped = scoped.With(slog.String("pattern", pattern))
			}
			r = r.WithContext(WithLogger(ctx, scoped))
			if !base.Enabled(ctx, cfg.Level) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			completed := false
			defer func() {
				if cfg.Routes == nil && r.Method != http.MethodConnect {
					pattern = r.Pattern
				}
				attrs := make([]slog.Attr, 0, 7)
				attrs = append(attrs,
					slog.String("method", method),
					slog.String("pattern", pattern),
					slog.Int("status", sw.status),
					slog.Int64("bytes", sw.bytes),
					slog.Duration("duration", time.Since(start)),
				)
				if sw.hijacked {
					attrs = append(attrs, slog.Bool("hijacked", true))
				}
				if !completed {
					attrs = append(attrs, slog.Bool("panic", true))
				}
				if cfg.Path {
					p := r.URL.EscapedPath()
					if len(p) > maxLoggedPath {
						p = p[:maxLoggedPath]
					}
					attrs = append(attrs, slog.String("path", p))
				}
				base.LogAttrs(ctx, cfg.Level, "http request", attrs...)
			}()
			next.ServeHTTP(sw, r)
			if sw.status == 0 && !sw.hijacked {
				// net/http answers 200 for a handler that wrote nothing.
				sw.status = http.StatusOK
			}
			completed = true
		})
	}
}

// sanitizeMethod bounds a request method for logs: methods are tokens, but any
// byte sequence arrives.
func sanitizeMethod(m string) string {
	if m == "" || len(m) > 16 || !allBytes(m, func(c byte) bool { return 'A' <= c && c <= 'Z' }) {
		return "(other)"
	}
	return m
}

// statusWriter records the status and body size of a response for the
// request log. It is used by the handler's goroutines only while the handler
// runs, and read after it returns.
type statusWriter struct {
	http.ResponseWriter
	status   int
	bytes    int64
	hijacked bool
}

var (
	_ http.Flusher  = (*statusWriter)(nil)
	_ http.Hijacker = (*statusWriter)(nil)
	_ io.ReaderFrom = (*statusWriter)(nil)
)

func (w *statusWriter) WriteHeader(code int) {
	// 1xx responses are informational and may precede the final status.
	if w.status == 0 && code >= 200 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

// ReadFrom copies src to the response, keeping the underlying writer's
// ReadFrom (sendfile) when it has one. Without a status yet, it records 200
// with the first byte copied only: net/http starts the response of a copy
// with its first byte, so after a copy of nothing (an empty source, or one
// that fails at once) the status that the handler writes next is the real
// one.
func (w *statusWriter) ReadFrom(src io.Reader) (int64, error) {
	if w.status != 0 {
		n, err := io.Copy(w.ResponseWriter, src)
		w.bytes += n
		return n, err
	}
	// The first bytes go through Write, which records the status and
	// counts them; the LimitedReader hides a WriteTo of src, so that no
	// empty write starts the response of net/http unseen.
	n, err := io.Copy(writerOnly{w}, io.LimitReader(src, firstChunk))
	if err != nil || n < firstChunk {
		return n, err
	}
	m, err := io.Copy(w.ResponseWriter, src)
	w.bytes += m
	return n + m, err
}

// firstChunk is the length of the start of a copy that
// [statusWriter.ReadFrom] copies through Write, the length that net/http
// sniffs.
const firstChunk = 512

// writerOnly hides the methods of a writer other than Write.
type writerOnly struct{ io.Writer }

// FlushError flushes the response, which starts it with 200 if it had no
// status, unless the underlying writer cannot flush. net/http writes the
// status before it flushes, so a flush that fails otherwise has started
// the response.
func (w *statusWriter) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if w.status == 0 && !errors.Is(err, http.ErrNotSupported) {
		w.status = http.StatusOK
	}
	return err
}

// Flush is FlushError without the error, for [http.Flusher].
func (w *statusWriter) Flush() { _ = w.FlushError() }

// Hijack hands the connection to the handler, and records that it did.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked = true
	}
	return c, rw, err
}

// Unwrap returns the underlying writer, for [http.ResponseController].
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
