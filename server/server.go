package server

import (
	"context"
	"crypto/tls"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Defaults of [Options].
const (
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultIdleTimeout       = 2 * time.Minute
	DefaultMaxHeaderBytes    = 64 << 10

	// DefaultMaxConcurrentStreams bounds the concurrent requests of one
	// HTTP/2 connection: the lower end of what RFC 9113 recommends, and less
	// than the 250 that net/http uses otherwise. Every stream runs its own
	// handler, so this multiplies the per-connection bound of [LimitConns].
	DefaultMaxConcurrentStreams = 100
)

// Options configures [New]. The zero value is the hardened default; fields
// that are zero (or negative) take the default that their documentation
// names.
type Options struct {
	// Logger receives the server's own error log, at a level by who causes
	// a line: panics of handlers, and failed accepts (such as running out of
	// file descriptors, which net/http retries without returning from
	// Serve), at error level; misuse of the ResponseWriter by handlers (such
	// as a superfluous WriteHeader call) at warn level; everything else,
	// mostly caused by clients (failed TLS handshakes, HTTP/2 protocol
	// errors), at debug level. A line counts as one of the former only by the fixed
	// prefix that net/http starts it with, since the rest of a line may quote
	// what a client sent. Nil means [slog.Default] at the time of each
	// record.
	Logger *slog.Logger

	// TLS, if not nil, is the TLS configuration, which [Run] then serves. New
	// uses a clone, with MinVersion raised to TLS 1.2 if it is lower. It needs
	// a certificate source (Certificates, GetCertificate or
	// GetConfigForClient); Run fails without one.
	TLS *tls.Config

	// ReadHeaderTimeout bounds the time to read a request's header. Zero
	// means DefaultReadHeaderTimeout (10 s).
	ReadHeaderTimeout time.Duration

	// IdleTimeout bounds the time a keep-alive connection waits for its next
	// request. Zero means DefaultIdleTimeout (2 min).
	IdleTimeout time.Duration

	// MaxHeaderBytes bounds the size of a request's header. Zero means
	// DefaultMaxHeaderBytes (64 KiB).
	MaxHeaderBytes int

	// MaxConcurrentStreams bounds the concurrent requests (streams) of one
	// HTTP/2 connection, which [Run] serves with TLS. Zero means
	// DefaultMaxConcurrentStreams (100).
	MaxConcurrentStreams int
}

// New returns an HTTP server for handler with hardened defaults (see
// [Options]): bounded header reads, idle connections, header sizes and
// concurrent HTTP/2 streams per connection (srv.HTTP2), the error log through
// slog, TLS 1.2 or later when TLS is configured, and no ReadTimeout or
// WriteTimeout (see the package documentation for the middleware that bounds
// bodies and responses instead).
//
// It also sets DisableGeneralOptionsHandler, so that "OPTIONS *" requests
// reach handler (and its security middleware) like every other request.
//
// The caller may adjust the returned server before serving it, and serves it
// with [Run] or the methods of [http.Server]. New panics if handler is nil:
// http.Server would serve [http.DefaultServeMux] instead, which other packages
// may have registered handlers on.
func New(handler http.Handler, opts Options) *http.Server {
	if handler == nil {
		panic("server: New needs a handler")
	}
	srv := &http.Server{
		Handler:                      handler,
		ReadHeaderTimeout:            positive(opts.ReadHeaderTimeout, DefaultReadHeaderTimeout),
		IdleTimeout:                  positive(opts.IdleTimeout, DefaultIdleTimeout),
		MaxHeaderBytes:               positive(opts.MaxHeaderBytes, DefaultMaxHeaderBytes),
		ErrorLog:                     log.New(errorLogWriter{logger: opts.Logger}, "", 0),
		DisableGeneralOptionsHandler: true,
		HTTP2: &http.HTTP2Config{
			MaxConcurrentStreams: positive(opts.MaxConcurrentStreams, DefaultMaxConcurrentStreams),
		},
	}
	if opts.TLS != nil {
		cfg := opts.TLS.Clone()
		if cfg.MinVersion < tls.VersionTLS12 {
			cfg.MinVersion = tls.VersionTLS12
		}
		srv.TLSConfig = cfg
	}
	return srv
}

func positive[T int | time.Duration](v, def T) T {
	if v <= 0 {
		return def
	}
	return v
}

// errorLogWriter forwards the lines of http.Server's error log to slog, at
// the level that [errorLogLevel] chooses. The server's log.Logger has no
// prefix and makes one Write per line, so a line starts with the text that
// net/http chose.
type errorLogWriter struct {
	logger *slog.Logger // nil: slog.Default()
}

func (w errorLogWriter) Write(p []byte) (int, error) {
	l := w.logger
	if l == nil {
		l = slog.Default()
	}
	msg := strings.TrimSuffix(string(p), "\n")
	l.Log(context.Background(), errorLogLevel(msg), msg)
	return len(p), nil
}

// errorLogLevel classifies a line of http.Server's error log by who causes
// it. Only fixed prefixes count: the rest of a line may quote client input,
// such as an offered ALPN protocol or an HTTP/2 preface, which would
// otherwise let a client raise its own text to error level.
func errorLogLevel(msg string) slog.Level {
	switch {
	case strings.HasPrefix(msg, "http: panic serving "),
		strings.HasPrefix(msg, "http2: panic serving "),
		// Serve retries temporary accept errors, such as EMFILE and ENFILE,
		// with a backoff instead of returning: new connections stall.
		strings.HasPrefix(msg, "http: Accept error: "):
		return slog.LevelError
	case strings.HasPrefix(msg, "http: superfluous response.WriteHeader call"),
		strings.HasPrefix(msg, "http: response.WriteHeader on hijacked connection"),
		strings.HasPrefix(msg, "http: response.Write on hijacked connection"),
		strings.HasPrefix(msg, "http: invalid Content-Length of "),
		strings.HasPrefix(msg, "http: WriteHeader called with both Transfer-Encoding"):
		return slog.LevelWarn
	default:
		return slog.LevelDebug
	}
}
