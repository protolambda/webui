package server_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"sync"
	"time"
)

// pipeListener is an in-memory listener whose connections are net.Pipe pairs,
// for servers inside synctest bubbles. Dial may give the server end a remote
// address.
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

// Dial connects to the listener and returns the client end. The server end
// reports remote as its peer address when it is not nil.
func (l *pipeListener) Dial(remote net.Addr) (net.Conn, error) {
	server, client := net.Pipe()
	var sc net.Conn = server
	if remote != nil {
		sc = addrConn{Conn: server, remote: remote}
	}
	select {
	case l.conns <- sc:
		return client, nil
	case <-l.closed:
		_ = server.Close()
		_ = client.Close()
		return nil, net.ErrClosed
	}
}

// IsClosed reports whether the listener was closed.
func (l *pipeListener) IsClosed() bool {
	select {
	case <-l.closed:
		return true
	default:
		return false
	}
}

// client returns an HTTP client whose connections dial ln.
func (l *pipeListener) client() *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return l.Dial(nil) },
	}}
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// addrConn is a connection with a chosen remote address.
type addrConn struct {
	net.Conn
	remote net.Addr
}

func (c addrConn) RemoteAddr() net.Addr { return c.remote }

// fakeConn is a connection that records Close and CloseWrite calls and keeps
// what is written to it.
type fakeConn struct {
	remote net.Addr

	mu         sync.Mutex
	closes     int
	closeWrite int
	written    []byte
}

func newFakeConn(remote net.Addr) *fakeConn { return &fakeConn{remote: remote} }

func (c *fakeConn) Read([]byte) (int, error) { return 0, errors.New("fake connection") }

func (c *fakeConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, b...)
	return len(b), nil
}

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return nil
}

func (c *fakeConn) CloseWrite() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeWrite++
	return nil
}

func (c *fakeConn) Closes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

func (c *fakeConn) CloseWrites() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeWrite
}

func (c *fakeConn) Written() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.written)
}

func (c *fakeConn) LocalAddr() net.Addr              { return pipeAddr{} }
func (c *fakeConn) RemoteAddr() net.Addr             { return c.remote }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

// fakeListener accepts the connections sent on its channel, then fails with
// err once the channel is closed.
type fakeListener struct {
	conns  chan net.Conn
	err    error
	mu     sync.Mutex
	closes int
}

func newFakeListener(err error) *fakeListener {
	return &fakeListener{conns: make(chan net.Conn, 64), err: err}
}

func (l *fakeListener) Accept() (net.Conn, error) {
	c, ok := <-l.conns
	if !ok {
		return nil, l.err
	}
	return c, nil
}

func (l *fakeListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closes++
	return nil
}

func (l *fakeListener) Addr() net.Addr { return tcpAddr("192.0.2.200:443") }

// tcpAddr parses an address:port into a *net.TCPAddr.
func tcpAddr(s string) *net.TCPAddr {
	return net.TCPAddrFromAddrPort(netip.MustParseAddrPort(s))
}

// captureHandler is a slog.Handler that keeps its records.
type captureHandler struct {
	mu      *sync.Mutex
	records *[]slog.Record
}

func newCapture() (*slog.Logger, func() []slog.Record) {
	var mu sync.Mutex
	var records []slog.Record
	h := captureHandler{mu: &mu, records: &records}
	return slog.New(h), func() []slog.Record {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(records)
	}
}

func (h captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, r.Clone())
	return nil
}

func (h captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h captureHandler) WithGroup(string) slog.Handler      { return h }
