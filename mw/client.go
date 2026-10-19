package mw

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
)

// ErrInvalidClientIP reports a malformed peer address, or a trusted proxy
// that supplied no single valid client address. Its errors quote no input.
var ErrInvalidClientIP = errors.New("mw: invalid client IP")

// ClientIP returns the address to use for per-client limits. With no
// trustedProxy, or a peer that it does not trust, this is the IP address
// from r.RemoteAddr; forwarding headers are ignored.
//
// For a trusted peer, X-Real-IP must contain exactly one IP address, with
// no port or zone. A missing, repeated or malformed header returns an
// error wrapping ErrInvalidClientIP rather than grouping every client
// under the proxy's address. The result is unmapped and has no zone.
//
// Configure trustedProxy from the proxy peers the application actually
// accepts connections from, never from a request header. Each trusted
// proxy must overwrite X-Real-IP on every request with the verified client
// address, replacing any client-supplied value. It resolves any earlier
// forwarding hops itself: ClientIP does not parse forwarding chains.
// trustedProxy must be safe for concurrent calls. ClientIP changes neither
// the request nor the scheme and host checks of this package.
func ClientIP(r *http.Request, trustedProxy func(netip.Addr) bool) (netip.Addr, error) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%w: peer is not an IP address and port", ErrInvalidClientIP)
	}
	addr := peer.Addr().Unmap().WithZone("")
	if trustedProxy == nil || !trustedProxy(addr) {
		return addr, nil
	}
	values := r.Header.Values("X-Real-IP")
	if len(values) != 1 {
		return netip.Addr{}, fmt.Errorf("%w: proxy must supply one X-Real-IP header", ErrInvalidClientIP)
	}
	client, err := netip.ParseAddr(values[0])
	if err != nil || client.Zone() != "" {
		return netip.Addr{}, fmt.Errorf("%w: X-Real-IP is not one IP address", ErrInvalidClientIP)
	}
	return client.Unmap(), nil
}
