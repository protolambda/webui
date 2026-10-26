package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// DefaultShutdownTimeout is how long [Run] waits for requests in flight at
// shutdown by default.
const DefaultShutdownTimeout = 10 * time.Second

// ErrNoCertificate reports a server with a TLS configuration but without a
// certificate source, which [Run] refuses to serve.
var ErrNoCertificate = errors.New("server: TLS configuration has no certificate " +
	"(Certificates, GetCertificate or GetConfigForClient)")

// RunOption configures [Run].
type RunOption func(*runConfig)

type runConfig struct {
	timeout time.Duration
	hooks   []func()
}

// ShutdownTimeout sets how long Run waits for requests in flight once
// shutdown starts, before it closes the remaining connections, whose
// handlers may outlast Run (see [Run]). A non-positive d means
// [DefaultShutdownTimeout].
func ShutdownTimeout(d time.Duration) RunOption {
	return func(c *runConfig) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// OnShutdown adds functions that Run calls when shutdown starts, in order,
// once the listener is closed and while it waits for requests in flight, such
// as one that ends the application's event streams: [http.Server.Shutdown]
// waits for every active response to finish, and a stream would otherwise run
// into the timeout. The functions should start ending things and return, not
// wait for them; Run returns only after they returned.
//
// Each function should set a stop signal that lasts, such as closing a
// channel or canceling a context that every stream also checks when it
// starts, rather than end only the streams open at that moment: requests
// already on their way while shutdown starts may still start streams, which
// would otherwise run into the timeout. Nil functions are skipped.
func OnShutdown(hooks ...func()) RunOption {
	return func(c *runConfig) {
		for _, h := range hooks {
			if h != nil {
				c.hooks = append(c.hooks, h)
			}
		}
	}
}

// Run serves srv on ln until ctx is done, then shuts down gracefully, and
// returns once the server stopped: its listener and its connections are
// closed, which need not mean that every handler returned (see below). It
// takes ownership of ln, which is closed (once) when Run returns, and of
// srv, which cannot serve again afterwards: use a fresh server for every
// Run.
//
// Run serves TLS when srv.TLSConfig is not nil (as [New] sets it for
// Options.TLS), so ln must be a plain listener; it is wrapped in TLS after
// any listener wrapper, such as [LimitConns], so refused connections cost no
// handshake. A TLS configuration needs a certificate source; without one, Run
// returns [ErrNoCertificate] at once, without serving. Note that
// [http.Server.Serve] and golang.org/x/net/http2.ConfigureServer set an empty
// TLSConfig on a plain server, which then looks like such a configuration:
// configure HTTP/2 without TLS through srv.Protocols instead.
//
// Shutdown, when ctx is done or serving failed:
//
//  1. [http.Server.Shutdown] closes the listener;
//  2. Run closes the connections that have not started a request (see
//     below), then the [OnShutdown] functions run, in order, alongside the
//     functions registered with [http.Server.RegisterOnShutdown] (such as
//     the one that sends HTTP/2 GOAWAY frames), while Shutdown closes idle
//     connections and waits for requests in flight, up to the
//     [ShutdownTimeout] (by default [DefaultShutdownTimeout]);
//  3. after the timeout, the request contexts are canceled and the remaining
//     connections closed ([http.Server.Close]).
//
// Run does not wait for every handler: Close does not wait for the
// handlers of the connections that it closes after the timeout, and
// Shutdown never waits for those of hijacked connections. A handler that
// is slow to notice the cancellation of its context, such as one in a
// long computation or in a call that ignores the context, may therefore
// still run after Run returned, and race with what the program does next,
// such as closing a database or another service that the handler uses.
// After a timeout, Run's error wraps [context.DeadlineExceeded]; a
// hijacked connection's handler leaves no trace in it. Make handlers stop
// on the request's context, and close shared services only once no
// handler can use them any more: for example after a [sync.WaitGroup]
// that the handlers using them join, or only as the process exits.
//
// Shutdown alone counts a connection that has not started a request as busy
// during its first 5 seconds, so a browser that opened a connection ahead of
// time (a preconnect) would hold up every restart. Run therefore wraps
// srv.ConnState, calling a hook that was set before, to track the accepted
// connections in [http.StateNew]: during the TLS handshake, before the first
// request header is read in full, and for HTTP/2 before the client preface.
// When shutdown starts, Run closes them, and closes any such connection that
// the server still accepts. That loses no request: net/http does not serve a
// request whose header it finishes reading after shutdown started, and an
// HTTP/2 connection carries no stream before its preface. Connections with
// requests in flight, idle keep-alive connections (which Shutdown closes
// itself), HTTP/2 connections past the preface (which get a GOAWAY frame) and
// hijacked connections are left to Shutdown, and so are connections whose
// value is not comparable (listeners return pointers in practice). Leave
// srv.ConnState as Run set it until Run returns.
//
// When srv.BaseContext is nil, Run sets it so that request contexts carry the
// values of ctx (such as a logger attached with [mw.WithLogger]) but not its
// cancellation: requests in flight may finish during shutdown, and their
// contexts are canceled when shutdown ends (also for hijacked connections,
// which Shutdown does not wait for).
//
// Run returns nil after a graceful shutdown that ctx started. It returns an
// error if serving failed (such as a listener that broke), if srv was shut
// down or closed by other means than ctx, such as an earlier Run (wrapping
// [http.ErrServerClosed]), or if requests were still in flight at the timeout
// (wrapping [context.DeadlineExceeded]).
func Run(ctx context.Context, srv *http.Server, ln net.Listener, opts ...RunOption) error {
	if srv == nil || ln == nil {
		if ln != nil {
			_ = ln.Close()
		}
		return errors.New("server: Run needs a server and a listener")
	}
	useTLS := srv.TLSConfig != nil
	if c := srv.TLSConfig; useTLS && len(c.Certificates) == 0 && c.GetCertificate == nil && c.GetConfigForClient == nil {
		// http.Server would fall back to loading a key pair from the empty
		// file names, and fail with a confusing error.
		_ = ln.Close()
		return ErrNoCertificate
	}
	cfg := runConfig{timeout: DefaultShutdownTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	base, cancelBase := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelBase()
	if srv.BaseContext == nil {
		srv.BaseContext = func(net.Listener) context.Context { return base }
	}

	// Shutdown counts a connection without a request as busy during its
	// first 5 seconds, such as one that a browser opened ahead of time;
	// fresh closes them once shutdown started.
	var fresh freshConns
	srv.ConnState = fresh.hook(srv.ConnState)

	// Shutdown calls this after it closed the listener, so that no new
	// connection starts a stream after the hooks. A Shutdown called by
	// other means runs it too; the Once keeps it to one run.
	hooksDone := make(chan struct{})
	var once sync.Once
	srv.RegisterOnShutdown(func() {
		once.Do(func() {
			defer close(hooksDone)
			fresh.closeAll()
			for _, hook := range cfg.hooks {
				hook()
			}
		})
	})

	// Serve closes the listener it gets on return. ServeTLS may return before
	// it calls Serve, so Run closes ln too on that path, through a wrapper
	// that closes only once: Close of a net.Listener need not be idempotent.
	// The plain path passes ln as is, so that a BaseContext of the caller
	// sees the caller's listener.
	var tlsLn *onceCloseListener
	served := make(chan error, 1)
	if useTLS {
		tlsLn = &onceCloseListener{Listener: ln}
		go func() { served <- srv.ServeTLS(tlsLn, "", "") }()
	} else {
		go func() { served <- srv.Serve(ln) }()
	}

	var serveErr error
	serveDone := false
	select {
	case <-ctx.Done():
	case serveErr = <-served:
		serveDone = true
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancelShutdown()
	var shutdownErr error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("server: shutdown: %w", err)
		cancelBase()
		if err := srv.Close(); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("server: close: %w", err))
		}
	}
	cancelBase()
	if !serveDone {
		serveErr = <-served
	}
	if tlsLn != nil {
		_ = tlsLn.Close()
	}
	<-hooksDone

	// ErrServerClosed is the result of the Shutdown that ctx started; when
	// serving ended with it before, the server was closed by other means.
	if errors.Is(serveErr, http.ErrServerClosed) && (!serveDone || ctx.Err() != nil) {
		serveErr = nil
	}
	if serveErr != nil {
		serveErr = fmt.Errorf("server: serve: %w", serveErr)
	}
	return errors.Join(serveErr, shutdownErr)
}

// onceCloseListener closes the listener it wraps only once, and returns the
// result of that close to every call.
type onceCloseListener struct {
	net.Listener
	once sync.Once
	err  error
}

func (l *onceCloseListener) Close() error {
	l.once.Do(func() { l.err = l.Listener.Close() })
	return l.err
}
