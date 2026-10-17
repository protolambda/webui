package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
)

// DefaultConnsPerAddr is the default bound of [LimitConns]. Browsers open up to
// six connections per site, which leaves room for several users behind one
// address.
const DefaultConnsPerAddr = 64

// ConnLimit configures [LimitConns].
type ConnLimit struct {
	// PerAddr bounds the concurrent connections of one client address group
	// ([AddrPrefix]). Zero (or negative) means DefaultConnsPerAddr.
	PerAddr int

	// Exempt, if not nil, names further peers whose connections are not
	// bounded, such as trusted reverse proxies, which carry the connections of
	// many users. Loopback peers are always exempt. It is called once per
	// accepted connection, possibly concurrently, and must be fast.
	Exempt func(peer netip.Addr) bool

	// Logger, if not nil, receives a debug record for every refused
	// connection, with the address group.
	Logger *slog.Logger
}

// LimitConns returns a listener that bounds the concurrent connections of
// each client address group: an IPv4 address, or the /64 of an IPv6 address
// ([AddrPrefix]). A connection counts from its accept to its close, so idle
// keep-alive connections and slow ones count too; one beyond the bound is
// closed at once, before any TLS handshake or HTTP, and Accept moves on to
// the next. This bounds what one client can hold of the server's connection
// slots, buffers and goroutines.
//
// Connections from loopback peers, from peers that cfg.Exempt names, and
// without an IP peer address (such as Unix sockets) are not bounded. Behind a
// reverse proxy every connection comes from the proxy: exempt it, and bound
// clients at the proxy.
//
// The returned listener is safe for concurrent use. Closing it closes ln.
// Its connections forward CloseWrite (which net/http uses before closing a
// connection whose request it did not read to the end) and ReadFrom.
func LimitConns(ln net.Listener, cfg ConnLimit) net.Listener {
	if cfg.PerAddr <= 0 {
		cfg.PerAddr = DefaultConnsPerAddr
	}
	return &limitListener{Listener: ln, cfg: cfg, open: make(map[netip.Prefix]int)}
}

// AddrPrefix returns the group of a client address for per-address bounds:
// the address itself (a /32) for IPv4 and IPv4-mapped IPv6 addresses, and its
// /64 for IPv6, since one host can easily rotate addresses within its /64.
// The zone is dropped. The zero Addr returns the zero Prefix.
func AddrPrefix(a netip.Addr) netip.Prefix {
	if !a.IsValid() {
		return netip.Prefix{}
	}
	a = a.Unmap().WithZone("")
	bits := 64
	if a.Is4() {
		bits = 32
	}
	p, _ := a.Prefix(bits)
	return p
}

// limitListener bounds the connections per address group of a listener.
type limitListener struct {
	net.Listener
	cfg ConnLimit

	mu   sync.Mutex
	open map[netip.Prefix]int // open connections per group; no zero entries
}

// Accept returns the next connection within the bound of its group, and
// closes the others.
func (l *limitListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		peer, ok := peerAddr(c.RemoteAddr())
		if !ok || peer.IsLoopback() || (l.cfg.Exempt != nil && l.cfg.Exempt(peer)) {
			return c, nil
		}
		group := AddrPrefix(peer)
		if !l.acquire(group) {
			if l.cfg.Logger != nil {
				l.cfg.Logger.LogAttrs(context.Background(), slog.LevelDebug,
					"refused a connection: its address has too many open connections",
					slog.String("group", group.String()), slog.Int("limit", l.cfg.PerAddr))
			}
			_ = c.Close()
			continue
		}
		return &limitedConn{Conn: c, release: func() { l.release(group) }}, nil
	}
}

func (l *limitListener) acquire(group netip.Prefix) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.open[group] >= l.cfg.PerAddr {
		return false
	}
	l.open[group]++
	return true
}

func (l *limitListener) release(group netip.Prefix) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.open[group]--; l.open[group] <= 0 {
		delete(l.open, group)
	}
}

// peerAddr returns the IP address of a connection's peer, unmapped and
// without zone.
func peerAddr(a net.Addr) (netip.Addr, bool) {
	var ip netip.Addr
	switch a := a.(type) {
	case nil:
		return netip.Addr{}, false
	case *net.TCPAddr:
		if a == nil {
			return netip.Addr{}, false
		}
		ip = a.AddrPort().Addr()
	default:
		ap, err := netip.ParseAddrPort(a.String())
		if err != nil {
			return netip.Addr{}, false
		}
		ip = ap.Addr()
	}
	if !ip.IsValid() {
		return netip.Addr{}, false
	}
	return ip.Unmap().WithZone(""), true
}

// limitedConn is a connection that counts against the bound of its group
// until it is closed.
type limitedConn struct {
	net.Conn
	closed  atomic.Bool
	release func()
}

// Close closes the connection and frees its place in the bound, once.
func (c *limitedConn) Close() error {
	err := c.Conn.Close()
	if c.closed.CompareAndSwap(false, true) {
		c.release()
	}
	return err
}

// CloseWrite shuts down the writing side of the connection if it supports
// that, as a TCP connection does.
func (c *limitedConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}

// ReadFrom copies from r, keeping the connection's own ReadFrom (sendfile,
// splice) when it has one.
func (c *limitedConn) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(c.Conn, r)
}
