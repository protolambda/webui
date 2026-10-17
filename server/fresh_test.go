package server_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/server"
)

// readEnd reads from c until it fails, in a goroutine, and returns the
// channel that receives the error.
func readEnd(c net.Conn) <-chan error {
	end := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, c)
		if err == nil {
			err = io.EOF
		}
		end <- err
	}()
	return end
}

// ended reports whether the reader of readEnd has finished.
func ended(end <-chan error) bool {
	select {
	case err := <-end:
		return err != nil
	default:
		return false
	}
}

// Connections without a request are closed at once when shutdown starts,
// while requests in flight finish: a connection dialed but never used, and
// one whose request header is incomplete.
func TestRunClosesFreshConns(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		started, release := make(chan struct{}), make(chan struct{})
		srv := server.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/slow" {
				close(started)
				<-release
			}
			_, _ = io.WriteString(w, "finished")
		}), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, pipes, server.ShutdownTimeout(time.Minute)) }()

		// An idle keep-alive connection, which Shutdown closes itself.
		client := pipes.client()
		defer client.CloseIdleConnections()
		status, _, err := get(client, "http://app.example.com/")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, status))

		unused, err := pipes.Dial(nil)
		t.Must(be.NoError(err))
		unusedEnd := readEnd(unused)
		partial, err := pipes.Dial(nil)
		t.Must(be.NoError(err))
		_, err = io.WriteString(partial, "GET / HTTP/1.1\r\nHost: app.example.com\r\n")
		t.Must(be.NoError(err))
		partialEnd := readEnd(partial)

		type result struct {
			body string
			err  error
		}
		slow := make(chan result, 1)
		go func() {
			_, body, err := get(pipes.client(), "http://app.example.com/slow")
			slow <- result{body, err}
		}()
		<-started

		start := time.Now()
		cancel()
		synctest.Wait()
		t.Mustf(be.True(ended(unusedEnd)), "the unused connection is closed")
		t.Mustf(be.True(ended(partialEnd)), "the incomplete request is not served")
		select {
		case err := <-done:
			t.Mustf(be.Failed(), "Run returned with a request in flight: %v", err)
		default:
		}
		time.Sleep(time.Second)
		close(release)
		r := <-slow
		t.Must(be.NoError(r.err))
		t.Must(be.Equal("finished", r.body))
		t.Must(be.NoError(<-done))
		// Shutdown polls for idle connections at most every 500ms.
		t.Mustf(be.Less(time.Since(start), 2*time.Second), "Run waited for the request only")
	})
}

// Without requests in flight, shutdown does not wait for unused
// connections.
func TestRunUnusedConnDoesNotDelay(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, pipes) }()
		unused, err := pipes.Dial(nil)
		t.Must(be.NoError(err))
		end := readEnd(unused)
		synctest.Wait()
		start := time.Now()
		cancel()
		t.Must(be.NoError(<-done))
		t.Must(be.Less(time.Since(start), 100*time.Millisecond))
		t.Must(be.ErrorIs(<-end, io.EOF))
	})
}

// lateListener accepts one more connection after Close, once release is
// closed: a connection that the server accepts just as shutdown starts.
type lateListener struct {
	*pipeListener
	release chan struct{}
	late    net.Conn
	used    bool // only the Serve goroutine calls Accept
}

func (l *lateListener) Accept() (net.Conn, error) {
	c, err := l.pipeListener.Accept()
	if err == nil || l.used {
		return c, err
	}
	l.used = true
	<-l.release
	return l.late, nil
}

// A connection accepted after the unused ones were closed is closed at
// once too. The OnShutdown functions run after that close, so the one here
// lets the listener return the late connection only then.
func TestRunClosesLateConns(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		serverEnd, clientEnd := net.Pipe()
		ln := &lateListener{pipeListener: newPipeListener(), release: make(chan struct{}), late: serverEnd}
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, ln, server.OnShutdown(func() { close(ln.release) })) }()
		end := readEnd(clientEnd)
		synctest.Wait()
		start := time.Now()
		cancel()
		t.Must(be.NoError(<-done))
		t.Must(be.True(ln.used))
		t.Must(be.Less(time.Since(start), 100*time.Millisecond))
		t.Must(be.ErrorIs(<-end, io.EOF))
	})
}

// Run keeps a ConnState hook that was set, and calls it for every state.
func TestRunChainsConnState(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		var mu sync.Mutex
		var states []http.ConnState
		srv.ConnState = func(_ net.Conn, state http.ConnState) {
			mu.Lock()
			defer mu.Unlock()
			states = append(states, state)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, pipes) }()
		client := pipes.client()
		defer client.CloseIdleConnections()
		_, _, err := get(client, "http://app.example.com/")
		t.Must(be.NoError(err))
		cancel()
		t.Must(be.NoError(<-done))
		synctest.Wait()
		mu.Lock()
		defer mu.Unlock()
		t.Must(be.DeepEqual([]http.ConnState{http.StateNew, http.StateActive, http.StateIdle, http.StateClosed}, states))
	})
}

// valueConn is a connection whose value is not comparable.
type valueConn struct {
	net.Conn
	_ []string // makes the type not comparable
}

// valueListener returns its connections as values that are not comparable.
type valueListener struct{ *pipeListener }

func (l valueListener) Accept() (net.Conn, error) {
	c, err := l.pipeListener.Accept()
	if err != nil {
		return nil, err
	}
	return valueConn{Conn: c}, nil
}

// A listener whose connections cannot key a map works as before: such
// connections are left to Shutdown, which waits for them.
func TestRunUncomparableConns(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, valueListener{pipes}) }()
		client := pipes.client()
		defer client.CloseIdleConnections()
		status, body, err := get(client, "http://app.example.com/")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, status))
		t.Must(be.Equal("hello", body))
		unused, err := pipes.Dial(nil)
		t.Must(be.NoError(err))
		end := readEnd(unused)
		synctest.Wait()
		start := time.Now()
		cancel()
		t.Must(be.NoError(<-done))
		t.Must(be.GreaterOrEq(time.Since(start), 5*time.Second))
		t.Must(be.ErrorIs(<-end, io.EOF))
	})
}

// TestShutdownDropsLateRequests pins the net/http behavior that makes
// closing unused connections at shutdown safe: a request whose header the
// server finishes reading after shutdown started is not served.
func TestShutdownDropsLateRequests(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		var served atomic.Bool
		srv := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served.Store(true) })}
		go func() { _ = srv.Serve(pipes) }()
		c, err := pipes.Dial(nil)
		t.Must(be.NoError(err))
		_, err = io.WriteString(c, "GET / HTTP/1.1\r\nHost: app.example.com\r\n")
		t.Must(be.NoError(err))
		shutdown := make(chan error, 1)
		go func() { shutdown <- srv.Shutdown(context.Background()) }()
		synctest.Wait()
		_, err = io.WriteString(c, "\r\n")
		t.Must(be.NoError(err))
		response, err := io.ReadAll(c)
		t.Must(be.NoError(err))
		t.Must(be.Empty(response))
		t.Must(be.False(served.Load()))
		t.Must(be.NoError(<-shutdown))
	})
}

// Over TLS on a real loopback listener, connections without a request
// close at once: one in the handshake, one after an HTTP/1.1 handshake,
// and one after an HTTP/2 handshake without the client preface. Shutdown
// alone would wait 5 seconds for the first two and the preface timeout for
// the third, longer than the shutdown timeout here.
func TestRunClosesFreshTLSConns(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cert, pool := selfSigned(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	t.Must(be.NoError(err))
	srv := server.New(http.HandlerFunc(hello), server.Options{TLS: &tls.Config{Certificates: []tls.Certificate{cert}}})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx, srv, ln, server.ShutdownTimeout(3*time.Second)) }()
	addr := ln.Addr().String()

	// A request on its own connection, closed after it.
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}, DisableKeepAlives: true}}
	status, body, err := get(client, "https://"+addr+"/")
	t.Must(be.NoError(err))
	t.Must(be.Equal(http.StatusOK, status))
	t.Must(be.Equal("hello", body))

	raw, err := net.Dial("tcp", addr)
	t.Must(be.NoError(err))
	conns := []net.Conn{raw}
	for _, proto := range []string{"http/1.1", "h2"} {
		c, err := tls.Dial("tcp", addr, &tls.Config{RootCAs: pool, NextProtos: []string{proto}})
		t.Must(be.NoError(err))
		t.Must(be.Equal(proto, c.ConnectionState().NegotiatedProtocol))
		conns = append(conns, c)
	}
	var ends []<-chan error
	for _, c := range conns {
		defer c.Close()
		// Bounds the test if a connection stays open.
		t.Must(be.NoError(c.SetReadDeadline(time.Now().Add(time.Minute))))
		ends = append(ends, readEnd(c))
	}

	cancel()
	t.Must(be.NoError(<-done))
	for i, end := range ends {
		err := <-end
		t.Mustf(be.False(errors.Is(err, os.ErrDeadlineExceeded)), "connection %d closed", i)
	}
}
