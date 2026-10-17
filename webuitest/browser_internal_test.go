package webuitest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// TestBrowserPinsTheTestCertificate checks that a Browser, which skips the
// name check because httptest's certificate does not name the site, still
// refuses a server with another certificate.
func TestBrowserPinsTheTestCertificate(gt *testing.T) {
	t := devtest.ParallelT(gt)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	t.Must(be.NoError(err))
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "app.example.test"},
		DNSNames:     []string{"app.example.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	t.Must(be.NoError(err))
	impostor := httptest.NewUnstartedServer(http.NotFoundHandler())
	impostor.Config.ErrorLog = log.New(io.Discard, "", 0) // the refused handshake
	impostor.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	impostor.StartTLS()
	defer impostor.Close()

	b := NewBrowser(t, http.NotFoundHandler(), BrowserOptions{})
	t.Must(b.Get(t, "/").Status(http.StatusNotFound))
	// Send the browser's requests to the impostor instead.
	tr := b.client.Transport.(*http.Transport).Clone()
	var d net.Dialer
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return d.DialContext(ctx, network, impostor.Listener.Addr().String())
	}
	defer tr.CloseIdleConnections()
	_, err = (&http.Client{Transport: tr}).Do(b.Request(t, http.MethodGet, "/", Body{}))
	t.Must(be.ErrorContains(err, "webuitest: the server is not the test server"))
}
