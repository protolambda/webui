package server_test

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/server"
)

func TestNewDefaults(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := http.NotFoundHandler()
	for _, opts := range []server.Options{
		{},
		{ReadHeaderTimeout: -time.Second, IdleTimeout: -time.Second, MaxHeaderBytes: -1},
	} {
		srv := server.New(h, opts)
		t.Must(be.Equal(10*time.Second, srv.ReadHeaderTimeout))
		t.Must(be.Equal(2*time.Minute, srv.IdleTimeout))
		t.Must(be.Equal(64<<10, srv.MaxHeaderBytes))
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
		ReadHeaderTimeout: 3 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    16 << 10,
	})
	t.Must(be.Equal(3*time.Second, srv.ReadHeaderTimeout))
	t.Must(be.Equal(time.Minute, srv.IdleTimeout))
	t.Must(be.Equal(16<<10, srv.MaxHeaderBytes))
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
	srv.ErrorLog.Printf("http: TLS handshake error from %s: EOF", "192.0.2.1:50000")
	srv.ErrorLog.Printf("http: panic serving %s: %v\n%s", "192.0.2.1:50000", "boom", "goroutine 1 [running]:")
	got := records()
	t.Must(be.Len(got, 2))
	t.Must(be.Equal(slog.LevelDebug, got[0].Level))
	t.Must(be.Equal("http: TLS handshake error from 192.0.2.1:50000: EOF", got[0].Message))
	t.Must(be.Equal(slog.LevelError, got[1].Level))
	t.Must(be.Prefix(got[1].Message, "http: panic serving 192.0.2.1:50000: boom"))

	t.Mustf(be.NoPanic(func() {
		server.New(http.NotFoundHandler(), server.Options{}).ErrorLog.Print("to the default logger")
	}), "a nil logger means slog.Default")
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
