package server

import (
	"crypto/tls"
	"net"
	"net/http"
	"reflect"
	"sync"
)

// freshConns tracks the connections of a server that have not started a
// request yet, through the server's ConnState hook: a connection enters in
// [http.StateNew] and leaves at its next state. [Run] closes them when
// shutdown starts, since [http.Server.Shutdown] counts such a connection as
// busy during its first 5 seconds. The zero value is ready; it is safe for
// concurrent use.
//
// Closing them loses no request. An HTTP/1 connection leaves StateNew when
// the server has read a whole request header, before it serves the request,
// and net/http does not serve a request whose header it finishes reading
// after shutdown started (see TestShutdownDropsLateRequests). An HTTP/2
// connection leaves it when the server has read the client preface, before
// it reads any stream. Connections are closed while the lock is held, so a
// connection that leaves StateNew concurrently waits in the hook until its
// close is done.
type freshConns struct {
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	shutdown bool // set by closeAll: the hook closes new connections at once
}

// hook returns a ConnState hook that tracks connections, then calls next
// when it is not nil.
func (f *freshConns) hook(next func(net.Conn, http.ConnState)) func(net.Conn, http.ConnState) {
	return func(c net.Conn, state http.ConnState) {
		f.track(c, state)
		if next != nil {
			next(c, state)
		}
	}
}

func (f *freshConns) track(c net.Conn, state http.ConnState) {
	// A connection whose value is not comparable would make the map panic
	// in the accept loop; such a connection is left to Shutdown. Listeners
	// return pointers in practice.
	if c == nil || !reflect.ValueOf(c).Comparable() {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case state != http.StateNew:
		delete(f.conns, c)
	case f.shutdown:
		// Accepted while shutdown started: no request on it gets served.
		closeFresh(c)
	default:
		if f.conns == nil {
			f.conns = make(map[net.Conn]struct{})
		}
		f.conns[c] = struct{}{}
	}
}

// closeAll closes the tracked connections, and makes the hook close the
// connections that the server still accepts. Call it only once shutdown
// started (from a function registered with RegisterOnShutdown), so that
// net/http no longer serves requests that it reads on them.
func (f *freshConns) closeAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdown = true
	for c := range f.conns {
		closeFresh(c)
	}
	clear(f.conns)
}

// closeFresh closes a connection that carried no request. A TLS connection
// is closed below TLS, without the close_notify alert: writing it could
// block while the lock is held, and nothing was sent to finish.
func closeFresh(c net.Conn) {
	if tc, ok := c.(*tls.Conn); ok {
		c = tc.NetConn()
	}
	_ = c.Close()
}
