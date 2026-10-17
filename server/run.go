package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// DefaultShutdownTimeout is how long [Run] waits for requests in flight at
// shutdown by default.
const DefaultShutdownTimeout = 10 * time.Second

// RunOption configures [Run].
type RunOption func(*runConfig)

type runConfig struct {
	timeout time.Duration
	hooks   []func()
}

// ShutdownTimeout sets how long Run waits for requests in flight once
// shutdown starts, before it closes the remaining connections. A
// non-positive d means [DefaultShutdownTimeout].
func ShutdownTimeout(d time.Duration) RunOption {
	return func(c *runConfig) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// OnShutdown adds functions that Run calls when shutdown starts, in order and
// before it waits for requests in flight, such as one that ends the
// application's event streams: [http.Server.Shutdown] waits for every active
// response to finish, and a stream would otherwise run into the timeout. The
// functions should start ending things and return, not wait for them. Nil
// functions are skipped.
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
// returns once the server stopped. It takes ownership of ln, which is closed
// when Run returns, and of srv, which cannot serve again afterwards.
//
// Run serves TLS when srv.TLSConfig is not nil (as [New] sets it for
// Options.TLS), so ln must be a plain listener; it is wrapped in TLS after
// any listener wrapper, such as [LimitConns], so refused connections cost no
// handshake.
//
// Shutdown, when ctx is done or serving failed:
//
//  1. the [OnShutdown] functions run, in order;
//  2. [http.Server.Shutdown] closes the listener and idle connections, runs
//     the functions registered with [http.Server.RegisterOnShutdown], and
//     waits for requests in flight, up to the [ShutdownTimeout] (by default
//     [DefaultShutdownTimeout]);
//  3. after the timeout, the request contexts are canceled and the remaining
//     connections closed.
//
// When srv.BaseContext is nil, Run sets it so that request contexts carry the
// values of ctx (such as a logger attached with [mw.WithLogger]) but not its
// cancellation: requests in flight may finish during shutdown, and their
// contexts are canceled when shutdown ends (also for hijacked connections,
// which Shutdown does not wait for).
//
// Run returns nil after a graceful shutdown that ctx started. It returns an
// error if serving failed (such as a TLS configuration without certificates,
// or a listener that broke), or if requests were still in flight at the
// timeout (wrapping [context.DeadlineExceeded]).
func Run(ctx context.Context, srv *http.Server, ln net.Listener, opts ...RunOption) error {
	if srv == nil || ln == nil {
		if ln != nil {
			_ = ln.Close()
		}
		return errors.New("server: Run needs a server and a listener")
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

	served := make(chan error, 1)
	go func() {
		if srv.TLSConfig != nil {
			served <- srv.ServeTLS(ln, "", "")
		} else {
			served <- srv.Serve(ln)
		}
	}()

	var serveErr error
	serveDone := false
	select {
	case <-ctx.Done():
	case serveErr = <-served:
		serveDone = true
	}

	for _, hook := range cfg.hooks {
		hook()
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
	// Serve closes ln on return, except when it returns before it began.
	_ = ln.Close()

	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	if serveErr != nil {
		serveErr = fmt.Errorf("server: serve: %w", serveErr)
	}
	return errors.Join(serveErr, shutdownErr)
}
