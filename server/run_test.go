package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/server"
)

func hello(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "hello") }

// get fetches a URL and returns the status and body.
func get(c *http.Client, url string) (int, string, error) {
	res, err := c.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	return res.StatusCode, string(body), err
}

func TestRunServesUntilCanceled(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		var order []string
		done := make(chan error, 1)
		go func() {
			done <- server.Run(ctx, srv, pipes,
				server.OnShutdown(func() { order = append(order, "first") }, nil),
				nil,
				server.OnShutdown(func() { order = append(order, "second") }),
			)
		}()
		client := pipes.client()
		defer client.CloseIdleConnections()
		status, body, err := get(client, "http://app.example.com/")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, status))
		t.Must(be.Equal("hello", body))

		cancel()
		t.Must(be.NoError(<-done))
		t.Must(be.Equal("first second", strings.Join(order, " ")))
		t.Must(be.True(pipes.IsClosed()))
		_, _, err = get(client, "http://app.example.com/")
		t.Must(be.Error(err))
	})
}

// streamHandler starts a response, flushes it, and waits until stop is
// closed or the request ends.
func streamHandler(stop <-chan struct{}, ended chan<- error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: started\n\n")
		_ = http.NewResponseController(w).Flush()
		select {
		case <-stop:
			ended <- nil
		case <-r.Context().Done():
			ended <- r.Context().Err()
		}
	})
}

func TestRunHooksEndStreams(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		stop := make(chan struct{})
		ended := make(chan error, 1)
		srv := server.New(streamHandler(stop, ended), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- server.Run(ctx, srv, pipes,
				server.ShutdownTimeout(time.Minute),
				server.OnShutdown(func() { close(stop) }))
		}()
		client := pipes.client()
		defer client.CloseIdleConnections()
		res, err := client.Get("http://app.example.com/events")
		t.Must(be.NoError(err))
		defer res.Body.Close()
		bodies := make(chan string, 1)
		go func() {
			// The client reads on, as browsers do; the pipe has no buffer.
			body, _ := io.ReadAll(res.Body)
			bodies <- string(body)
		}()

		start := time.Now()
		cancel()
		t.Must(be.NoError(<-done))
		t.Mustf(be.Less(time.Since(start), time.Second), "the stream ended at once, without the timeout")
		t.Must(be.NoError(<-ended))
		t.Must(be.Equal("data: started\n\n", <-bodies))
	})
}

func TestRunShutdownTimeout(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		ended := make(chan error, 1)
		srv := server.New(streamHandler(nil, ended), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, pipes, server.ShutdownTimeout(5*time.Second)) }()
		client := pipes.client()
		defer client.CloseIdleConnections()
		res, err := client.Get("http://app.example.com/events")
		t.Must(be.NoError(err))
		defer res.Body.Close()

		start := time.Now()
		cancel()
		err = <-done
		t.Must(be.ErrorIs(err, context.DeadlineExceeded))
		t.Must(be.Substring(err.Error(), "server: shutdown"))
		t.Must(be.Equal(5*time.Second, time.Since(start)))
		t.Mustf(be.ErrorIs(<-ended, context.Canceled), "the stream's context is canceled at the timeout")
		_, err = io.ReadAll(res.Body)
		t.Must(be.Error(err))
	})
}

func TestRunLetsRequestsFinish(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		started, release := make(chan struct{}), make(chan struct{})
		var ctxErr error
		var logger *slog.Logger
		srv := server.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			ctxErr = r.Context().Err()
			logger = mw.Logger(r.Context())
			_, _ = io.WriteString(w, "finished")
		}), server.Options{})
		appLogger := slog.New(slog.DiscardHandler)
		ctx, cancel := context.WithCancel(mw.WithLogger(context.Background(), appLogger))
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, pipes) }()
		client := pipes.client()
		defer client.CloseIdleConnections()

		type result struct {
			body string
			err  error
		}
		results := make(chan result, 1)
		go func() {
			_, body, err := get(client, "http://app.example.com/save")
			results <- result{body, err}
		}()
		<-started
		cancel()
		synctest.Wait() // Run waits in Shutdown for the request
		close(release)
		r := <-results
		t.Must(be.NoError(r.err))
		t.Must(be.Equal("finished", r.body))
		t.Mustf(be.NoError(ctxErr), "the request context outlives the canceled Run context")
		t.Mustf(be.Same(appLogger, logger), "request contexts carry the values of the Run context")
		t.Must(be.NoError(<-done))
	})
}

func TestRunKeepsBaseContext(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		type key struct{}
		var value any
		srv := server.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value = r.Context().Value(key{})
		}), server.Options{})
		srv.BaseContext = func(net.Listener) context.Context {
			return context.WithValue(context.Background(), key{}, "own")
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- server.Run(ctx, srv, pipes) }()
		client := pipes.client()
		defer client.CloseIdleConnections()
		_, _, err := get(client, "http://app.example.com/")
		t.Must(be.NoError(err))
		t.Must(be.Equal[any]("own", value))
		cancel()
		t.Must(be.NoError(<-done))
	})
}

func TestRunServeError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	close(fake.conns)
	hooked := false
	err := server.Run(context.Background(), server.New(http.HandlerFunc(hello), server.Options{}), fake,
		server.OnShutdown(func() { hooked = true }))
	t.Must(be.ErrorIs(err, errListenerDone))
	t.Must(be.Substring(err.Error(), "server: serve"))
	t.Must(be.True(hooked))
	t.Mustf(be.Equal(1, fake.closes), "the listener is closed once")
}

func TestRunTLSWithoutCertificate(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	srv := server.New(http.HandlerFunc(hello), server.Options{TLS: &tls.Config{}})
	err := server.Run(context.Background(), srv, fake)
	t.Must(be.ErrorIs(err, server.ErrNoCertificate))
	t.Must(be.Substring(err.Error(), "GetCertificate"))
	t.Mustf(be.Equal(1, fake.closes), "the listener is closed once")
}

// TestRunClosesListenerOnce checks every path with a listener whose second
// Close panics.
func TestRunClosesListenerOnce(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cert, _ := selfSigned(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name    string
		opts    server.Options
		wantErr error
	}{
		{"plain", server.Options{}, nil},
		{"TLS", server.Options{TLS: &tls.Config{Certificates: []tls.Certificate{cert}}}, nil},
		{"TLS without certificate", server.Options{TLS: &tls.Config{}}, server.ErrNoCertificate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			ln := newStrictListener()
			srv := server.New(http.HandlerFunc(hello), c.opts)
			var err error
			t.Must(be.NoPanic(func() { err = server.Run(canceled, srv, ln) }))
			if c.wantErr != nil {
				t.Must(be.ErrorIs(err, c.wantErr))
			} else {
				t.Must(be.NoError(err))
			}
			t.Must(be.Equal(int32(1), ln.closes.Load()))
		})
	}
}

func TestRunReusedServer(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cert, _ := selfSigned(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	srv := server.New(http.HandlerFunc(hello), server.Options{TLS: &tls.Config{Certificates: []tls.Certificate{cert}}})
	t.Must(be.NoError(server.Run(canceled, srv, newStrictListener())))
	ln := newStrictListener()
	var err error
	t.Must(be.NoPanic(func() { err = server.Run(t.Context(), srv, ln) }))
	t.Mustf(be.ErrorIs(err, http.ErrServerClosed), "Run does not return nil without serving")
	t.Must(be.Substring(err.Error(), "server: serve"))
	t.Must(be.Equal(int32(1), ln.closes.Load()))

	// Serve gave the plain server an empty TLSConfig, without certificate.
	plain := server.New(http.HandlerFunc(hello), server.Options{})
	t.Must(be.NoError(server.Run(canceled, plain, newStrictListener())))
	ln = newStrictListener()
	t.Must(be.NoPanic(func() { err = server.Run(t.Context(), plain, ln) }))
	t.Must(be.Error(err))
	t.Must(be.Equal(int32(1), ln.closes.Load()))
}

func TestRunServerShutDownElsewhere(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		hooks := 0
		done := make(chan error, 1)
		go func() {
			done <- server.Run(context.Background(), srv, pipes, server.OnShutdown(func() { hooks++ }))
		}()
		synctest.Wait()
		t.Must(be.NoError(srv.Shutdown(context.Background())))
		err := <-done
		t.Mustf(be.ErrorIs(err, http.ErrServerClosed), "ctx did not start this shutdown")
		t.Mustf(be.Equal(1, hooks), "the hooks run once, for this Shutdown too")
		t.Must(be.True(pipes.IsClosed()))
	})
}

// TestRunHooksAfterListenerClosed checks that the hooks run once the listener
// is closed, so no new connection starts a stream after them.
func TestRunHooksAfterListenerClosed(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		pipes := newPipeListener()
		srv := server.New(http.HandlerFunc(hello), server.Options{})
		ctx, cancel := context.WithCancel(context.Background())
		var closedInHook bool
		done := make(chan error, 1)
		go func() {
			done <- server.Run(ctx, srv, pipes, server.OnShutdown(func() { closedInHook = pipes.IsClosed() }))
		}()
		synctest.Wait() // Serve waits in Accept
		cancel()
		t.Must(be.NoError(<-done))
		t.Must(be.True(closedInHook))
	})
}

func TestRunInvalidArguments(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	t.Must(be.Error(server.Run(context.Background(), nil, fake)))
	t.Mustf(be.Equal(1, fake.closes), "Run owns the listener")
	t.Must(be.Error(server.Run(context.Background(), server.New(http.HandlerFunc(hello), server.Options{}), nil)))
}

// selfSigned returns a certificate for 127.0.0.1 and a pool that trusts it.
func selfSigned(t mustbe.MT) (tls.Certificate, *x509.CertPool) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	t.Must(be.NoError(err))
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	t.Must(be.NoError(err))
	leaf, err := x509.ParseCertificate(der)
	t.Must(be.NoError(err))
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

func TestRunTLS(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cert, pool := selfSigned(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	t.Must(be.NoError(err))
	limited := server.LimitConns(ln, server.ConnLimit{})
	srv := server.New(http.HandlerFunc(hello), server.Options{TLS: &tls.Config{Certificates: []tls.Certificate{cert}}})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx, srv, limited) }()
	url := "https://" + ln.Addr().String() + "/"

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	res, err := client.Get(url)
	t.Must(be.NoError(err))
	body, err := io.ReadAll(res.Body)
	t.Must(be.NoError(err))
	t.Must(be.NoError(res.Body.Close()))
	t.Must(be.Equal("hello", string(body)))
	t.Must(be.GreaterOrEq(res.TLS.Version, uint16(tls.VersionTLS12)))
	client.CloseIdleConnections()

	old := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: pool, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11,
	}}}
	_, err = old.Get(url)
	t.Mustf(be.Error(err), "TLS 1.1 is refused")

	cancel()
	t.Must(be.NoError(<-done))
}

// TestRunHTTP2StreamLimit checks the SETTINGS frame that the server sends
// over HTTP/2: it advertises New's bound on concurrent streams.
func TestRunHTTP2StreamLimit(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cert, pool := selfSigned(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	t.Must(be.NoError(err))
	srv := server.New(http.HandlerFunc(hello), server.Options{TLS: &tls.Config{Certificates: []tls.Certificate{cert}}})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx, srv, ln) }()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{RootCAs: pool, NextProtos: []string{"h2"}})
	t.Must(be.NoError(err))
	t.Must(be.Equal("h2", conn.ConnectionState().NegotiatedProtocol))
	// The client preface, then an empty SETTINGS frame.
	_, err = io.WriteString(conn, "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n\x00\x00\x00\x04\x00\x00\x00\x00\x00")
	t.Must(be.NoError(err))
	settings := readHTTP2Settings(t, conn)
	const maxConcurrentStreams = 0x3
	t.Must(be.Equal(uint32(server.DefaultMaxConcurrentStreams), settings[maxConcurrentStreams]))
	t.Must(be.NoError(conn.Close()))

	cancel()
	t.Must(be.NoError(<-done))
}

// readHTTP2Settings reads HTTP/2 frames from r up to the first SETTINGS frame
// that is not an acknowledgment, and returns its parameters.
func readHTTP2Settings(t mustbe.MT, r io.Reader) map[uint16]uint32 {
	const typeSettings, flagAck = 0x4, 0x1
	for range 8 {
		var header [9]byte
		_, err := io.ReadFull(r, header[:])
		t.Must(be.NoError(err))
		size := int(header[0])<<16 | int(header[1])<<8 | int(header[2])
		t.Must(be.LessOrEq(size, 1<<14))
		payload := make([]byte, size)
		_, err = io.ReadFull(r, payload)
		t.Must(be.NoError(err))
		if header[3] != typeSettings || header[4]&flagAck != 0 {
			continue
		}
		t.Must(be.Equal(0, size%6))
		settings := make(map[uint16]uint32)
		for p := payload; len(p) > 0; p = p[6:] {
			settings[binary.BigEndian.Uint16(p)] = binary.BigEndian.Uint32(p[2:])
		}
		return settings
	}
	t.Mustf(be.Failed(), "no SETTINGS frame")
	return nil
}
