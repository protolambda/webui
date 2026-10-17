package server_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/server"
)

func ExampleNew() {
	srv := server.New(http.NotFoundHandler(), server.Options{})
	fmt.Println("read header timeout:", srv.ReadHeaderTimeout)
	fmt.Println("idle timeout:", srv.IdleTimeout)
	fmt.Println("max header bytes:", srv.MaxHeaderBytes)
	fmt.Println("read and write timeouts:", srv.ReadTimeout, srv.WriteTimeout)
	fmt.Println("HTTP/2 streams per connection:", srv.HTTP2.MaxConcurrentStreams)
	// Output:
	// read header timeout: 10s
	// idle timeout: 2m0s
	// max header bytes: 65536
	// read and write timeouts: 0s 0s
	// HTTP/2 streams per connection: 100
}

// A complete local application: the listener is checked and bounded, the
// handler carries the middleware of package mw, and Run serves until the
// context ends. A real program derives ctx from signal.NotifyContext.
func ExampleRun() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println(err)
		return
	}
	site, err := mw.ParseSite("http://" + ln.Addr().String())
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := server.CheckLoopback(ln.Addr(), site); err != nil {
		fmt.Println(err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "home") })
	handler := mw.Chain(mux,
		mw.LogRequests(mw.RequestLog{Routes: mux}),
		mw.BodyDeadline(mw.DefaultBodyTimeout),
		mw.WriteDeadline(mw.DefaultWriteTimeout),
		mw.SecurityHeaders(mw.Headers{}),
		mw.HostCheck(site),
		mw.SameOrigin(mw.OriginPolicy{}),
		mw.MaxBody(1<<20),
	)
	logger := slog.New(slog.DiscardHandler)
	srv := server.New(handler, server.Options{Logger: logger})

	ctx, stop := context.WithCancel(mw.WithLogger(context.Background(), logger))
	defer stop()
	streams, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx, srv, server.LimitConns(ln, server.ConnLimit{}), server.OnShutdown(endStreams))
	}()

	client := &http.Client{}
	res, err := client.Get(site.Origin() + "/")
	if err != nil {
		fmt.Println(err)
		return
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	client.CloseIdleConnections()
	fmt.Println(res.StatusCode, string(body), res.Header.Get("X-Frame-Options"))

	stop()
	fmt.Println("stopped:", <-done, "streams ended:", streams.Err() != nil)
	// Output:
	// 200 home DENY
	// stopped: <nil> streams ended: true
}

// Behind a reverse proxy, exempt the proxy: every connection comes from it.
func ExampleLimitConns() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println(err)
		return
	}
	proxies := netip.MustParsePrefix("10.0.0.0/24")
	ln = server.LimitConns(ln, server.ConnLimit{
		PerAddr: 32,
		Exempt:  proxies.Contains,
	})
	defer ln.Close()
	fmt.Println(ln.Addr().Network())
	// Output:
	// tcp
}

func ExampleAddrPrefix() {
	for _, a := range []string{"192.0.2.7", "::ffff:192.0.2.7", "2001:db8:aa:bb:1:2:3:4"} {
		fmt.Println(a, "->", server.AddrPrefix(netip.MustParseAddr(a)))
	}
	// Output:
	// 192.0.2.7 -> 192.0.2.7/32
	// ::ffff:192.0.2.7 -> 192.0.2.7/32
	// 2001:db8:aa:bb:1:2:3:4 -> 2001:db8:aa:bb::/64
}

func ExampleCheckLoopback() {
	site := mw.MustParseSite("http://localhost:8080")
	fmt.Println(server.CheckLoopback(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}, site))
	fmt.Println(server.CheckLoopback(&net.TCPAddr{IP: net.IPv4zero, Port: 8080}, site))
	// Output:
	// <nil>
	// server: loopback site on a non-loopback listener: http://localhost:8080 is a loopback site, but the listener address is tcp 0.0.0.0:8080; listen on a loopback address, such as 127.0.0.1, or configure the site's network URL
}
