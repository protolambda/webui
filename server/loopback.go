package server

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/protolambda/webui/mw"
)

// ErrLoopbackListener reports a loopback site served on a listener that the
// network reaches.
var ErrLoopbackListener = errors.New("server: loopback site on a non-loopback listener")

// IsLoopbackAddr reports whether a listener address is reachable from the
// local machine only: a TCP address on a loopback IP address. Unspecified
// addresses (0.0.0.0, ::), which accept from every interface, are not; nor
// are Unix sockets, which typically sit behind a reverse proxy that forwards
// traffic from the network.
func IsLoopbackAddr(addr net.Addr) bool {
	var ip netip.Addr
	switch a := addr.(type) {
	case nil:
		return false
	case *net.TCPAddr:
		if a == nil {
			return false
		}
		ip = a.AddrPort().Addr()
	default:
		if addr.Network() != "tcp" && addr.Network() != "tcp4" && addr.Network() != "tcp6" {
			return false
		}
		ap, err := netip.ParseAddrPort(addr.String())
		if err != nil {
			return false
		}
		ip = ap.Addr()
	}
	return ip.IsValid() && ip.Unmap().IsLoopback()
}

// CheckLoopback returns an error wrapping [ErrLoopbackListener] if one of the
// sites served on a listener with address addr is a loopback site
// ([mw.Site.Loopback]) while addr is not a loopback address
// ([IsLoopbackAddr]); else nil. Check the listener's actual address
// (net.Listener.Addr), after the host name of a listen address resolved.
//
// Loopback sites relax rules on the grounds that only the local machine
// reaches them: plain http, and requests without origin headers (see
// [mw.OriginPolicy]); applications may relax more, such as authentication
// for a local tool. The Host header, which the client chooses, cannot justify
// that on a listener the network reaches: any remote client could name the
// loopback site. A local reverse proxy that forwards network traffic to a
// loopback listener defeats this check too, so do not configure a loopback
// site behind one.
func CheckLoopback(addr net.Addr, sites ...mw.Site) error {
	if IsLoopbackAddr(addr) {
		return nil
	}
	for _, s := range sites {
		if s.Loopback() {
			listen := "an unknown address"
			if addr != nil {
				listen = addr.Network() + " " + addr.String()
			}
			return fmt.Errorf("%w: %s is a loopback site, but the listener address is %s; "+
				"listen on a loopback address, such as 127.0.0.1, or configure the site's network URL",
				ErrLoopbackListener, s.Origin(), listen)
		}
	}
	return nil
}
