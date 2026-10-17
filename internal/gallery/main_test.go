package main

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/server"
	"github.com/protolambda/webui/webuitest"
)

func TestGallerySites(gt *testing.T) {
	t := devtest.ParallelT(gt)
	loopback := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	sites, err := gallerySites(loopback, nil)
	t.Must(be.NoError(err))
	t.Must(be.Len(sites, 2))
	t.Must(be.Equal("http://127.0.0.1:8080", sites[0].Origin()))
	t.Must(be.Equal("http://localhost:8080", sites[1].Origin()))

	// A listener that the network reaches needs the configured origin.
	public := &net.TCPAddr{IP: net.IPv4zero, Port: 8080}
	_, err = gallerySites(public, nil)
	t.Must(be.Error(err))
	sites, err = gallerySites(public, []string{"https://gallery.example.com"})
	t.Must(be.NoError(err))
	t.Must(be.Equal("https://gallery.example.com", sites[0].Origin()))
	_, err = gallerySites(public, []string{"http://gallery.example.com"})
	t.Must(be.Error(err))
}

// TestRun serves the gallery on a loopback port, and shuts it down while a
// live stream is open: the shutdown hook ends the stream, so the shutdown
// is graceful instead of waiting for the timeout.
func TestRun(gt *testing.T) {
	t := devtest.ParallelT(gt)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	t.Must(be.NoError(err))
	origin := "http://" + ln.Addr().String()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	t.Must(be.NoError(err))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, webuitest.NewLogs().Logger(), ln, nil) }()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// get sends a GET request for path to the listener, naming host in
	// the Host header (the listen address when empty).
	get := func(path, host string, header http.Header) *http.Response {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin+path, nil)
		t.Must(be.NoError(err))
		if host != "" {
			req.Host = host
		}
		for k, v := range header {
			req.Header[k] = v
		}
		res, err := client.Do(req)
		t.Must(be.NoError(err))
		return res
	}
	status := func(path, host string) int {
		res := get(path, host, nil)
		t.Must(be.NoError(res.Body.Close()))
		return res.StatusCode
	}

	res := get("/", "", nil)
	t.Must(be.Equal(http.StatusOK, res.StatusCode))
	t.Must(be.Prefix(res.Header.Get("Content-Security-Policy"), "default-src 'none'"))
	t.Must(be.NoError(res.Body.Close()))
	// Plain-http loopback sites get cookies without the __Host- prefix.
	res = get("/login", "", nil)
	t.Must(be.Prefix(res.Header.Get("Set-Cookie"), "preauth="))
	t.Must(be.NoError(res.Body.Close()))
	// The default sites are the listen address and localhost; other hosts
	// are refused.
	t.Must(be.Equal(http.StatusOK, status("/", "localhost:"+port)))
	t.Must(be.Equal(http.StatusBadRequest, status("/", "gallery.example.com")))
	t.Must(be.Equal(http.StatusBadRequest, status("/", "localhost:1")))

	stream := get("/live/clock", "", http.Header{"Accept": {"text/event-stream"}})
	defer stream.Body.Close()
	t.Must(be.Equal(http.StatusOK, stream.StatusCode))
	line, err := bufio.NewReader(stream.Body).ReadString('\n')
	t.Must(be.NoError(err))
	t.Must(be.Equal("event: clock\n", line))

	// A connection that the client dialed but never used counts as busy
	// for http.Server.Shutdown during its first 5 seconds: close them.
	client.CloseIdleConnections()
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		t.Must(be.NoError(err))
		// Far below the shutdown timeout, which an open stream would
		// otherwise reach.
		t.Must(be.Less(time.Since(start), server.DefaultShutdownTimeout/2))
	case <-t.Context().Done():
		t.Fatal("run did not return")
	}
	// The listener is closed.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/", nil)
	t.Must(be.NoError(err))
	_, err = client.Do(req)
	t.Must(be.Error(err))
}
