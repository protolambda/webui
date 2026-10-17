package server_test

import (
	"net"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/server"
)

// namedAddr is a net.Addr of any network.
type namedAddr struct{ network, addr string }

func (a namedAddr) Network() string { return a.network }
func (a namedAddr) String() string  { return a.addr }

func TestIsLoopbackAddr(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{"IPv4 loopback", tcpAddr("127.0.0.1:8080"), true},
		{"IPv4 loopback range", tcpAddr("127.1.2.3:8080"), true},
		{"IPv6 loopback", tcpAddr("[::1]:8080"), true},
		{"16-byte IPv4 loopback", &net.TCPAddr{IP: net.ParseIP("127.0.0.1").To16(), Port: 8080}, true},
		{"IPv4-mapped loopback", tcpAddr("[::ffff:127.0.0.1]:8080"), true},
		{"unspecified IPv4", tcpAddr("0.0.0.0:8080"), false},
		{"unspecified IPv6", tcpAddr("[::]:8080"), false},
		{"wildcard without IP", &net.TCPAddr{Port: 8080}, false},
		{"network address", tcpAddr("192.0.2.1:8080"), false},
		{"nil", nil, false},
		{"nil TCP address", (*net.TCPAddr)(nil), false},
		{"nil Unix address", (*net.UnixAddr)(nil), false},
		{"Unix socket", &net.UnixAddr{Name: "/run/app.sock", Net: "unix"}, false},
		{"other tcp address type", namedAddr{"tcp", "127.0.0.1:8080"}, true},
		{"other tcp6 address type", namedAddr{"tcp6", "[::1]:8080"}, true},
		{"other non-loopback tcp", namedAddr{"tcp", "192.0.2.1:8080"}, false},
		{"other malformed tcp", namedAddr{"tcp", "localhost:8080"}, false},
		{"udp", namedAddr{"udp", "127.0.0.1:8080"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			t.Must(be.Equal(c.want, server.IsLoopbackAddr(c.addr)))
		})
	}
}

func TestCheckLoopback(gt *testing.T) {
	t := devtest.ParallelT(gt)
	local := mw.MustParseSite("http://localhost:8080")
	public := mw.MustParseSite("https://app.example.com")
	t.Must(be.NoError(server.CheckLoopback(tcpAddr("127.0.0.1:8080"), local)))
	t.Must(be.NoError(server.CheckLoopback(tcpAddr("127.0.0.1:8080"), local, public)))
	t.Must(be.NoError(server.CheckLoopback(tcpAddr("0.0.0.0:443"), public)))
	t.Must(be.NoError(server.CheckLoopback(nil)))

	err := server.CheckLoopback(tcpAddr("0.0.0.0:8080"), public, local)
	t.Must(be.ErrorIs(err, server.ErrLoopbackListener))
	t.Must(be.Substring(err.Error(), "http://localhost:8080"))
	t.Must(be.Substring(err.Error(), "tcp 0.0.0.0:8080"))

	err = server.CheckLoopback(nil, local)
	t.Must(be.ErrorIs(err, server.ErrLoopbackListener))
	t.Must(be.Substring(err.Error(), "an unknown address"))

	t.Must(be.ErrorIs(server.CheckLoopback(&net.UnixAddr{Name: "/run/app.sock", Net: "unix"}, local),
		server.ErrLoopbackListener))

	for _, addr := range []net.Addr{(*net.TCPAddr)(nil), (*net.UnixAddr)(nil)} {
		t.Must(be.NoError(server.CheckLoopback(addr)))
		t.Must(be.NoError(server.CheckLoopback(addr, public)))
		err = server.CheckLoopback(addr, local)
		t.Must(be.ErrorIs(err, server.ErrLoopbackListener))
		t.Must(be.Substring(err.Error(), "the listener address is an unknown address"))
	}
}

func TestCheckLoopbackOnRealListener(gt *testing.T) {
	t := devtest.ParallelT(gt)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	t.Must(be.NoError(err))
	defer ln.Close()
	t.Must(be.NoError(server.CheckLoopback(ln.Addr(), mw.MustParseSite("http://127.0.0.1:8080"))))
	t.Must(be.True(strings.HasPrefix(ln.Addr().String(), "127.0.0.1:")))
}
