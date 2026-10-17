package server_test

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/server"
)

func TestAddrPrefix(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := map[string]string{
		"192.0.2.1":               "192.0.2.1/32",
		"::ffff:192.0.2.1":        "192.0.2.1/32",
		"2001:db8:1:2:3:4:5:6":    "2001:db8:1:2::/64",
		"2001:db8:1:2::1%eth0":    "2001:db8:1:2::/64",
		"fe80::1%eth0":            "fe80::/64",
		"::1":                     "::/64",
		"2001:db8:1:3:ffff::ffff": "2001:db8:1:3::/64",
	}
	for in, want := range cases {
		t.Must(be.Equal(want, server.AddrPrefix(netip.MustParseAddr(in)).String()))
	}
	t.Must(be.Equal(netip.Prefix{}, server.AddrPrefix(netip.Addr{})))
}

// accept sends conns to ln and returns what Accept returns, one per conn that
// is accepted, in order.
func acceptAll(t mustbe.MT, ln net.Listener, fake *fakeListener, conns ...*fakeConn) []net.Conn {
	for _, c := range conns {
		fake.conns <- c
	}
	close(fake.conns)
	var out []net.Conn
	for {
		c, err := ln.Accept()
		if err != nil {
			t.Must(be.ErrorIs(err, fake.err))
			return out
		}
		out = append(out, c)
	}
}

var errListenerDone = errors.New("listener done")

func TestLimitConns(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	ln := server.LimitConns(fake, server.ConnLimit{PerAddr: 2})
	a := newFakeConn(tcpAddr("192.0.2.1:1001"))
	b := newFakeConn(tcpAddr("192.0.2.1:1002"))
	refused := newFakeConn(tcpAddr("192.0.2.1:1003"))
	mapped := newFakeConn(tcpAddr("[::ffff:192.0.2.1]:1004"))
	other := newFakeConn(tcpAddr("192.0.2.2:1001"))
	got := acceptAll(t, ln, fake, a, b, refused, mapped, other)
	t.Must(be.Len(got, 3))
	t.Must(be.Equal(1, refused.Closes()))
	t.Mustf(be.Equal(1, mapped.Closes()), "an IPv4-mapped address counts as the IPv4 address")
	t.Must(be.Equal(0, a.Closes()))
	t.Must(be.Equal(0, other.Closes()))
	t.Must(be.Equal("192.0.2.2:1001", got[2].RemoteAddr().String()))

	// Closing twice frees one place only.
	t.Must(be.NoError(got[1].Close()))
	t.Must(be.NoError(got[1].Close()))
	t.Must(be.Equal(2, b.Closes()))
	fake = newFakeListener(errListenerDone)
	ln2 := server.LimitConns(fake, server.ConnLimit{PerAddr: 2})
	first := acceptAll(t, ln2, fake, newFakeConn(tcpAddr("192.0.2.1:1")), newFakeConn(tcpAddr("192.0.2.1:2")))
	t.Must(be.NoError(first[1].Close()))
	t.Must(be.NoError(first[1].Close()))
	fake.conns = make(chan net.Conn, 4)
	third, fourth := newFakeConn(tcpAddr("192.0.2.1:3")), newFakeConn(tcpAddr("192.0.2.1:4"))
	t.Must(be.Len(acceptAll(t, ln2, fake, third, fourth), 1))
	t.Must(be.Equal(0, third.Closes()))
	t.Must(be.Equal(1, fourth.Closes()))
}

func TestLimitConnsIPv6Groups(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	ln := server.LimitConns(fake, server.ConnLimit{PerAddr: 1})
	sameA := newFakeConn(tcpAddr("[2001:db8:0:1::1]:1"))
	sameB := newFakeConn(tcpAddr("[2001:db8:0:1:ffff::2]:1"))
	otherNet := newFakeConn(tcpAddr("[2001:db8:0:2::1]:1"))
	got := acceptAll(t, ln, fake, sameA, sameB, otherNet)
	t.Must(be.Len(got, 2))
	t.Mustf(be.Equal(1, sameB.Closes()), "addresses in one /64 share the bound")
	t.Must(be.Equal(0, otherNet.Closes()))
}

func TestLimitConnsExemptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, records := newCapture()
	fake := newFakeListener(errListenerDone)
	proxy := netip.MustParseAddr("10.0.0.1")
	ln := server.LimitConns(fake, server.ConnLimit{
		PerAddr: 1,
		Exempt:  func(peer netip.Addr) bool { return peer == proxy },
		Logger:  logger,
	})
	var conns []*fakeConn
	for range 3 {
		conns = append(conns,
			newFakeConn(tcpAddr("127.0.0.1:1")),
			newFakeConn(tcpAddr("[::1]:1")),
			newFakeConn(tcpAddr("10.0.0.1:1")),
			newFakeConn(nil),
			newFakeConn(&net.UnixAddr{Name: "@", Net: "unix"}),
			newFakeConn((*net.TCPAddr)(nil)),
		)
	}
	limited := newFakeConn(tcpAddr("192.0.2.7:1"))
	refused := newFakeConn(tcpAddr("192.0.2.7:2"))
	conns = append(conns, limited, refused)
	got := acceptAll(t, ln, fake, conns...)
	t.Must(be.Len(got, len(conns)-1))
	for _, c := range conns[:len(conns)-1] {
		t.Must(be.Equal(0, c.Closes()))
	}
	t.Must(be.Equal(1, refused.Closes()))
	recs := records()
	t.Must(be.Len(recs, 1))
	t.Must(be.Substring(recs[0].Message, "too many open connections"))
	var group string
	recs[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "group" {
			group = a.Value.String()
		}
		return true
	})
	t.Must(be.Equal("192.0.2.7/32", group))
}

func TestLimitConnsOtherAddressTypes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	ln := server.LimitConns(fake, server.ConnLimit{PerAddr: 1})
	named := newFakeConn(namedAddr{"custom", "192.0.2.3:1"})
	sameAddr := newFakeConn(tcpAddr("192.0.2.3:2"))
	unparsable := newFakeConn(namedAddr{"custom", "somewhere"})
	got := acceptAll(t, ln, fake, named, sameAddr, unparsable, newFakeConn(namedAddr{"custom", "somewhere"}))
	t.Must(be.Len(got, 3))
	t.Mustf(be.Equal(1, sameAddr.Closes()), "an address parsed from its string counts too")
	t.Mustf(be.Equal(0, unparsable.Closes()), "connections without an IP address are not bounded")
}

func TestLimitConnsWithoutCloseWrite(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	ln := server.LimitConns(fake, server.ConnLimit{})
	srvEnd, client := net.Pipe()
	defer client.Close()
	fake.conns <- addrConn{Conn: srvEnd, remote: tcpAddr("192.0.2.1:1")}
	close(fake.conns)
	c, err := ln.Accept()
	t.Must(be.NoError(err))
	defer c.Close()
	t.Must(be.NoError(c.(interface{ CloseWrite() error }).CloseWrite()))
}

func TestLimitConnsForwards(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	ln := server.LimitConns(fake, server.ConnLimit{})
	c := newFakeConn(tcpAddr("192.0.2.1:1"))
	got := acceptAll(t, ln, fake, c)
	t.Must(be.Len(got, 1))
	cw, ok := got[0].(interface{ CloseWrite() error })
	t.Must(be.True(ok))
	t.Must(be.NoError(cw.CloseWrite()))
	t.Must(be.Equal(1, c.CloseWrites()))
	rf, ok := got[0].(io.ReaderFrom)
	t.Must(be.True(ok))
	n, err := rf.ReadFrom(strings.NewReader("hello"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(int64(5), n))
	t.Must(be.Equal("hello", string(c.Written())))
	t.Must(be.Equal(fake.Addr().String(), ln.Addr().String()))
	t.Must(be.NoError(ln.Close()))
	t.Must(be.Equal(1, fake.closes))
}

func TestLimitConnsConcurrent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const limit, total = 4, 400
	fake := &fakeListener{conns: make(chan net.Conn, total), err: errListenerDone}
	ln := server.LimitConns(fake, server.ConnLimit{PerAddr: limit})
	for i := range total {
		fake.conns <- newFakeConn(tcpAddr("192.0.2.1:" + strconv.Itoa(1000+i)))
	}
	close(fake.conns)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				_ = c.Close()
			}
		})
	}
	wg.Wait()
	// Every place was freed: the bound admits limit new connections again.
	fake.conns = make(chan net.Conn, limit+1)
	var again []*fakeConn
	for i := range limit + 1 {
		again = append(again, newFakeConn(tcpAddr("192.0.2.1:"+strconv.Itoa(i+1))))
	}
	t.Must(be.Len(acceptAll(t, ln, fake, again...), limit))
}

// TestLimitConnsWithServer checks the bound through http.Server: a client
// that holds its keep-alive connection blocks a second connection from its
// address until it closes the first.
func TestLimitConnsWithServer(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "ok")
		}), server.Options{})
		ln := server.LimitConns(pipes, server.ConnLimit{PerAddr: 1})
		done := make(chan error, 1)
		go func() { done <- srv.Serve(ln) }()

		peer := tcpAddr("192.0.2.9:40000")
		get := func(c net.Conn) (*http.Response, error) {
			if _, err := io.WriteString(c, "GET / HTTP/1.1\r\nHost: app.example.com\r\n\r\n"); err != nil {
				return nil, err
			}
			res, err := http.ReadResponse(bufio.NewReader(c), nil)
			if err != nil {
				return nil, err
			}
			_, _ = io.Copy(io.Discard, res.Body)
			return res, res.Body.Close()
		}

		first, err := pipes.Dial(peer)
		t.Must(be.NoError(err))
		res, err := get(first)
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, res.StatusCode))

		second, err := pipes.Dial(peer)
		t.Must(be.NoError(err))
		_, err = get(second)
		t.Mustf(be.Error(err), "the second connection of the address is closed at once")
		_ = second.Close()

		t.Must(be.NoError(first.Close()))
		synctest.Wait() // the server notices the close and frees the place
		third, err := pipes.Dial(peer)
		t.Must(be.NoError(err))
		res, err = get(third)
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, res.StatusCode))
		_ = third.Close()

		t.Must(be.NoError(srv.Close()))
		t.Must(be.ErrorIs(<-done, http.ErrServerClosed))
	})
}
