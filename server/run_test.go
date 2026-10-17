package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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
	t.Must(be.GreaterOrEq(fake.closes, 1))
}

func TestRunTLSWithoutCertificate(gt *testing.T) {
	t := devtest.ParallelT(gt)
	fake := newFakeListener(errListenerDone)
	srv := server.New(http.HandlerFunc(hello), server.Options{TLS: &tls.Config{}})
	err := server.Run(context.Background(), srv, fake)
	t.Must(be.Error(err))
	t.Must(be.Substring(err.Error(), "server: serve"))
	t.Mustf(be.GreaterOrEq(fake.closes, 1), "the listener is closed")
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
