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
)

// Options configures [New]. The zero value is the hardened default; fields
// that are zero (or negative) take the default that their documentation
// names.
type Options struct {
	// Logger receives the server's own error log (failed TLS handshakes,
	// malformed requests, and the like) at debug level, since clients cause
	// most of it, and panics of handlers at error level. Nil means
	// [slog.Default] at the time of each record.
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
}

// New returns an HTTP server for handler with hardened defaults (see
// [Options]): bounded header reads, idle connections and header sizes, the
// error log through slog, TLS 1.2 or later when TLS is configured, and no
// ReadTimeout or WriteTimeout (see the package documentation for the
// middleware that bounds bodies and responses instead).
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

// errorLogWriter forwards the lines of http.Server's error log to slog: panics
// of handlers at error level, everything else at debug level.
type errorLogWriter struct {
	logger *slog.Logger // nil: slog.Default()
}

func (w errorLogWriter) Write(p []byte) (int, error) {
	l := w.logger
	if l == nil {
		l = slog.Default()
	}
	msg := strings.TrimSuffix(string(p), "\n")
	level := slog.LevelDebug
	// net/http logs "http: panic serving ..." and "http2: panic serving ...".
	if strings.Contains(msg, "panic serving") {
		level = slog.LevelError
	}
	l.Log(context.Background(), level, msg)
	return len(p), nil
}
