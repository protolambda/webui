package server_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/server"
)

func TestNewDefaults(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := http.NotFoundHandler()
	for _, opts := range []server.Options{
		{},
		{ReadHeaderTimeout: -time.Second, IdleTimeout: -time.Second, MaxHeaderBytes: -1, MaxConcurrentStreams: -1},
	} {
		srv := server.New(h, opts)
		t.Must(be.Equal(10*time.Second, srv.ReadHeaderTimeout))
		t.Must(be.Equal(2*time.Minute, srv.IdleTimeout))
		t.Must(be.Equal(64<<10, srv.MaxHeaderBytes))
		t.Must(be.NonNil(srv.HTTP2))
		t.Mustf(be.Equal(100, srv.HTTP2.MaxConcurrentStreams), "not net/http's 250 streams per connection")
		t.Mustf(be.Equal(time.Duration(0), srv.ReadTimeout), "bodies are bounded by mw.BodyDeadline")
		t.Mustf(be.Equal(time.Duration(0), srv.WriteTimeout), "responses are bounded by mw.WriteDeadline")
		t.Must(be.True(srv.DisableGeneralOptionsHandler))
		t.Must(be.NonNil(srv.ErrorLog))
		t.Must(be.Nil(srv.TLSConfig))
		t.Must(be.Nil(srv.BaseContext))
	}
}

func TestNewOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	srv := server.New(http.NotFoundHandler(), server.Options{
		ReadHeaderTimeout:    3 * time.Second,
		IdleTimeout:          time.Minute,
		MaxHeaderBytes:       16 << 10,
		MaxConcurrentStreams: 32,
	})
	t.Must(be.Equal(3*time.Second, srv.ReadHeaderTimeout))
	t.Must(be.Equal(time.Minute, srv.IdleTimeout))
	t.Must(be.Equal(16<<10, srv.MaxHeaderBytes))
	t.Must(be.Equal(32, srv.HTTP2.MaxConcurrentStreams))
}

func TestNewTLS(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct{ given, want uint16 }{
		{0, tls.VersionTLS12},
		{tls.VersionTLS10, tls.VersionTLS12},
		{tls.VersionTLS12, tls.VersionTLS12},
		{tls.VersionTLS13, tls.VersionTLS13},
	}
	for _, c := range cases {
		given := &tls.Config{MinVersion: c.given, ServerName: "app.example.com"}
		srv := server.New(http.NotFoundHandler(), server.Options{TLS: given})
		t.Must(be.Equal(c.want, srv.TLSConfig.MinVersion))
		t.Must(be.Equal("app.example.com", srv.TLSConfig.ServerName))
		t.Mustf(be.Equal(c.given, given.MinVersion), "the caller's configuration is not modified")
		t.Must(be.True(srv.TLSConfig != given))
	}
}

func TestNewPanicsWithoutHandler(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Panic(func() { server.New(nil, server.Options{}) }))
}

func TestErrorLog(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, records := newCapture()
	srv := server.New(http.NotFoundHandler(), server.Options{Logger: logger})
	peer := "192.0.2.1:50000"
	cases := []struct {
		name  string
		line  string
		level slog.Level
	}{
		{"failed handshake", fmt.Sprintf("http: TLS handshake error from %s: EOF", peer), slog.LevelDebug},
		{"panic", fmt.Sprintf("http: panic serving %s: %v\n%s", peer, "boom", "goroutine 1 [running]:"),
			slog.LevelError},
		{"HTTP/2 panic", fmt.Sprintf("http2: panic serving %s: %v\n%s", peer, "boom", "goroutine 1 [running]:"),
			slog.LevelError},
		{"failed accept", fmt.Sprintf("http: Accept error: %v; retrying in %v",
			"accept tcp 127.0.0.1:8080: accept4: too many open files", "5ms"), slog.LevelError},
		{"superfluous WriteHeader", "http: superfluous response.WriteHeader call from app.save (save.go:12)",
			slog.LevelWarn},
		{"write on hijacked", "http: response.Write on hijacked connection from app.ws (ws.go:3)", slog.LevelWarn},
		{"handler's Content-Length", `http: invalid Content-Length of "x"`, slog.LevelWarn},

		// Lines that quote client input stay at debug, whatever the client
		// put in them.
		{"offered ALPN protocol", fmt.Sprintf("http: TLS handshake error from %s: "+
			"tls: client requested unsupported application protocols (%q)", peer,
			[]string{"http: panic serving x"}), slog.LevelDebug},
		{"HTTP/2 preface", fmt.Sprintf("http2: server: error reading preface from client %s: bogus greeting %q",
			peer, "panic serving FAKEPANIC!"), slog.LevelDebug},
		{"accept text later in a line", fmt.Sprintf("http: TLS handshake error from %s: %q", peer,
			"http: Accept error: x"), slog.LevelDebug},
	}
	for _, c := range cases {
		srv.ErrorLog.Print(c.line)
	}
	got := records()
	t.Must(be.Len(got, len(cases)))
	for i, c := range cases {
		t.Mustf(be.Equal(c.level, got[i].Level), "%s", c.name)
		t.Mustf(be.Equal(strings.TrimSuffix(c.line, "\n"), got[i].Message), "%s", c.name)
	}

	t.Mustf(be.NoPanic(func() {
		server.New(http.NotFoundHandler(), server.Options{}).ErrorLog.Print("to the default logger")
	}), "a nil logger means slog.Default")
}

// TestErrorLogOfClientText checks a real handshake: a client that offers an
// ALPN protocol with the text of a panic line gets no error-level record.
func TestErrorLogOfClientText(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cert, pool := selfSigned(t)
	logger, records := newCapture()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	t.Must(be.NoError(err))
	srv := server.New(http.HandlerFunc(hello), server.Options{
		Logger: logger,
		TLS:    &tls.Config{Certificates: []tls.Certificate{cert}},
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx, srv, ln) }()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		RootCAs:    pool,
		NextProtos: []string{"http: panic serving attacker text"},
	})
	if err == nil {
		_ = conn.Close()
	}
	t.Mustf(be.Error(err), "the server knows no such protocol")
	cancel()
	t.Mustf(be.NoError(<-done), "shutdown waits for the connection, and so for its record")

	var handshake bool
	for _, r := range records() {
		t.Mustf(be.Less(r.Level, slog.LevelWarn), "client text stays at debug: %s", r.Message)
		handshake = handshake || strings.Contains(r.Message, "attacker text")
	}
	t.Mustf(be.True(handshake), "the failed handshake was logged")
}

// TestErrorLogOfAcceptError checks a real failed accept: net/http retries it
// without returning from Serve, so the log is the only sign.
func TestErrorLogOfAcceptError(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		logger, records := newCapture()
		pipes := newPipeListener()
		ln := &failingListener{Listener: pipes, failures: 1,
			err: &net.OpError{Op: "accept", Net: "tcp", Err: syscall.EMFILE}}
		srv := server.New(http.HandlerFunc(hello), server.Options{Logger: logger})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, ln) }()
		client := pipes.client()
		defer client.CloseIdleConnections()
		status, _, err := get(client, "http://app.example.com/")
		t.Must(be.NoError(err))
		t.Mustf(be.Equal(http.StatusOK, status), "Serve retried the accept")
		cancel()
		t.Must(be.NoError(<-done))

		got := records()
		t.Must(be.Len(got, 1))
		t.Must(be.Equal(slog.LevelError, got[0].Level))
		t.Must(be.Prefix(got[0].Message, "http: Accept error: "))
	})
}

func TestErrorLogOfPanickingHandler(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logger, records := newCapture()
	ts := httptest.NewUnstartedServer(nil)
	ts.Config = server.New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("handler bug")
	}), server.Options{Logger: logger})
	ts.Start()
	defer ts.Close()
	res, err := ts.Client().Get(ts.URL)
	if err == nil {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
	ts.Close()
	var panics int
	for _, r := range records() {
		if r.Level == slog.LevelError && strings.Contains(r.Message, "handler bug") {
			panics++
		}
	}
	t.Must(be.Equal(1, panics))
}
