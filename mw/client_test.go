package mw_test

import (
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

func TestClientIP(gt *testing.T) {
	t := devtest.ParallelT(gt)
	trusted := netip.MustParsePrefix("192.0.2.0/24").Contains
	for _, c := range []struct {
		name, peer string
		header     []string
		trust      func(netip.Addr) bool
		want       string
	}{
		{"direct", "198.51.100.2:1234", nil, nil, "198.51.100.2"},
		{"no proxy policy", "192.0.2.1:1234", []string{"203.0.113.2"}, nil, "192.0.2.1"},
		{"untrusted spoof", "198.51.100.2:1234", []string{"203.0.113.2", "invalid"}, trusted, "198.51.100.2"},
		{"trusted proxy", "192.0.2.1:1234", []string{"203.0.113.2"}, trusted, "203.0.113.2"},
		{"mapped peer", "[::ffff:192.0.2.1]:1234", []string{"203.0.113.2"}, trusted, "203.0.113.2"},
		{"mapped client", "192.0.2.1:1234", []string{"::ffff:203.0.113.2"}, trusted, "203.0.113.2"},
		{"IPv6 client", "192.0.2.1:1234", []string{"2001:db8::2"}, trusted, "2001:db8::2"},
		{"IPv6 peer", "[2001:db8::1]:1234", nil, nil, "2001:db8::1"},
		{"zoned peer", "[fe80::1%eth0]:1234", nil, nil, "fe80::1"},
	} {
		t.Run(c.name, func(t devtest.T) {
			r := httptest.NewRequest("GET", "https://app.example/", nil)
			r.RemoteAddr = c.peer
			r.Header["X-Real-Ip"] = c.header
			addr, err := mw.ClientIP(r, c.trust)
			t.Must(be.NoError(err))
			t.Must(be.Equal(c.want, addr.String()))
			t.Must(be.Equal(c.peer, r.RemoteAddr))
		})
	}
}

func TestClientIPRejectsInvalidProxyHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	trusted := netip.MustParsePrefix("192.0.2.0/24").Contains
	for _, c := range []struct {
		name   string
		header []string
	}{
		{"missing", nil},
		{"empty", []string{""}},
		{"repeated", []string{"203.0.113.2", "203.0.113.3"}},
		{"chain", []string{"203.0.113.2, 203.0.113.3"}},
		{"port", []string{"203.0.113.2:1234"}},
		{"zone", []string{"fe80::1%eth0"}},
		{"hostname", []string{"client.example"}},
		{"malformed", []string{"private request text"}},
	} {
		t.Run(c.name, func(t devtest.T) {
			r := httptest.NewRequest("GET", "https://app.example/", nil)
			r.RemoteAddr = "192.0.2.1:1234"
			r.Header["X-Real-Ip"] = c.header
			// Other forwarding headers cannot supply the missing identity.
			r.Header.Set("X-Forwarded-For", "203.0.113.2")
			r.Header.Set("Forwarded", "for=203.0.113.2")
			addr, err := mw.ClientIP(r, trusted)
			t.Must(be.ErrorIs(err, mw.ErrInvalidClientIP))
			t.Must(be.False(addr.IsValid()))
			for _, value := range c.header {
				if value != "" {
					t.Must(be.False(strings.Contains(err.Error(), value)))
				}
			}
		})
	}
}

func TestClientIPRejectsInvalidPeer(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, peer := range []string{"", "private request text", "203.0.113.2", "client.example:1234"} {
		r := httptest.NewRequest("GET", "https://app.example/", nil)
		r.RemoteAddr = peer
		r.Header.Set("X-Real-IP", "203.0.113.2")
		addr, err := mw.ClientIP(r, func(netip.Addr) bool {
			t.Fatal("invalid peer reached proxy policy")
			return true
		})
		t.Must(be.ErrorIs(err, mw.ErrInvalidClientIP))
		t.Must(be.False(addr.IsValid()))
		if peer != "" {
			t.Must(be.False(strings.Contains(err.Error(), peer)))
		}
	}
}
