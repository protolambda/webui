package webuitest

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
)

// InsecureBaseURL is the base URL of a [Browser] with
// [BrowserOptions.Insecure].
const InsecureBaseURL = "http://localhost"

// BrowserOptions configure [NewBrowser].
type BrowserOptions struct {
	// Insecure serves plain http at [InsecureBaseURL] instead of https at
	// [BaseURL]: a loopback name, as plain http sites are loopback sites
	// for mw.ParseSite. Secure and __Host- cookies, which webui's sessions
	// use by default, then do not reach the server, and mw.SameOrigin
	// accepts requests without origin headers on loopback sites.
	Insecure bool
	// HTTP2 enables HTTP/2 (with TLS only).
	HTTP2 bool
	// Configure, when set, adapts the server before it starts, for
	// example its timeouts.
	Configure func(*http.Server)
	// StreamTimeout is the timeout of [Browser.Stream] streams, see
	// [StreamTimeout]; zero means [DefaultStreamTimeout].
	StreamTimeout time.Duration
}

// Browser is a client of a real HTTP server (httptest) that serves a
// handler: the full-stack harness, with cookies end to end, middleware
// that needs connections (deadlines), and streams. It keeps a cookie jar,
// does not follow redirects (tests assert them, see [Location]), and
// sends browser-realistic requests as [Request] builds them.
//
// The server listens on a loopback port, but the browser names it
// [BaseURL] (or [InsecureBaseURL] with [BrowserOptions.Insecure]):
// every connection goes to the server, so the Host header, the Origin
// header, cookies and TLS all name the application's site, as in
// production. Configure the application with that site.
//
// A Browser is safe for concurrent use; its methods take the test, so one
// browser can serve subtests. The server is closed at the end of the test
// that created it.
type Browser struct {
	srv     *httptest.Server
	client  *http.Client
	base    *url.URL
	timeout time.Duration
	// Options apply to every request, before the options of each call.
	Options []ReqOption
}

var _ Sender = (*Browser)(nil)

// NewBrowser starts a server for h and returns a browser of it. The
// server is closed when the test ends.
func NewBrowser(t TB, h http.Handler, o BrowserOptions) *Browser {
	t.Helper()
	if h == nil {
		fatalf(t, "webuitest.NewBrowser: nil handler")
		return nil
	}
	srv := httptest.NewUnstartedServer(h)
	if o.Configure != nil {
		o.Configure(srv.Config)
	}
	base := *defaultBase
	if o.Insecure {
		base = *mustParseBase(InsecureBaseURL)
		srv.Start()
	} else {
		srv.EnableHTTP2 = o.HTTP2
		srv.StartTLS()
	}
	t.Cleanup(srv.Close)
	tr := srv.Client().Transport.(*http.Transport).Clone()
	addr := srv.Listener.Addr().String()
	var d net.Dialer
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return d.DialContext(ctx, network, addr)
	}
	if tr.TLSClientConfig != nil {
		// The test certificate of httptest names example.com.
		cfg := tr.TLSClientConfig.Clone()
		cfg.ServerName = "example.com"
		cfg.MinVersion = tls.VersionTLS12
		tr.TLSClientConfig = cfg
	}
	timeout := o.StreamTimeout
	if timeout <= 0 {
		timeout = DefaultStreamTimeout
	}
	b := &Browser{srv: srv, base: &base, timeout: timeout}
	b.client = newBrowserClient(tr)
	return b
}

func newBrowserClient(tr http.RoundTripper) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		// cookiejar.New fails only with options it does not get here.
		panic(err)
	}
	return &http.Client{
		Transport: tr,
		Jar:       jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// URL returns the base URL of the browser's requests: [BaseURL] or
// [InsecureBaseURL].
func (b *Browser) URL() string { return b.base.String() }

// Server returns the test server.
func (b *Browser) Server() *httptest.Server { return b.srv }

// Jar returns the cookie jar of the browser.
func (b *Browser) Jar() http.CookieJar { return b.client.Jar }

// Fork returns another browser of the same server with an empty cookie
// jar and the same options: a second user, or the same user in a private
// window.
func (b *Browser) Fork() *Browser {
	f := *b
	f.client = newBrowserClient(b.client.Transport)
	f.Options = append([]ReqOption(nil), b.Options...)
	return &f
}

// Request builds a client request on the browser's base URL, see
// [Request].
func (b *Browser) Request(t TB, method, target string, body Body, opts ...ReqOption) *http.Request {
	t.Helper()
	all := append(append([]ReqOption(nil), b.Options...), opts...)
	req, err := buildRequest(t.Context(), b.base, method, target, body, all, false)
	if err != nil {
		fatalf(t, "webuitest: %v", err)
		return nil
	}
	return req
}

// Do sends req and captures the response. A transport error fails the
// test.
func (b *Browser) Do(t TB, req *http.Request) *cthttp.Result {
	t.Helper()
	if req == nil {
		fatalf(t, "webuitest.Browser: nil request")
		return nil
	}
	resp, err := b.client.Do(req)
	if err != nil {
		fatalf(t, "webuitest: %s %s: %v", req.Method, req.URL.Path, err)
		return nil
	}
	return cthttp.FromResponse(resp)
}

// Send builds a request ([Browser.Request]) and sends it ([Browser.Do]).
func (b *Browser) Send(t TB, method, target string, body Body, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	req := b.Request(t, method, target, body, opts...)
	if req == nil {
		return nil
	}
	return b.Do(t, req)
}

// Get sends a GET request for target.
func (b *Browser) Get(t TB, target string, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	return b.Send(t, http.MethodGet, target, Body{}, opts...)
}

// Post sends a POST request for target with body.
func (b *Browser) Post(t TB, target string, body Body, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	return b.Send(t, http.MethodPost, target, body, opts...)
}

// Submit submits a form of page as [Client.Submit] does, and sends the
// request.
func (b *Browser) Submit(t TB, page *ct.Subject, form, submitter *ct.Selection, edits url.Values, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	method, target, body, ok := submission(t, page, form, submitter, edits)
	if !ok {
		return nil
	}
	return b.Send(t, method, target, body, opts...)
}

// Stream opens an event stream as a browser's EventSource does ([EventSource]
// is applied first) and returns it once the response header arrives. It
// fails the test if the request fails or no header arrives within the
// timeout ([BrowserOptions.StreamTimeout]). The stream is closed at the
// end of the test.
func (b *Browser) Stream(t TB, target string, opts ...ReqOption) *Stream {
	t.Helper()
	req := b.Request(t, http.MethodGet, target, Body{}, append([]ReqOption{EventSource()}, opts...)...)
	if req == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(req.Context())
	stop := time.AfterFunc(b.timeout, cancel)
	resp, err := b.client.Do(req.WithContext(ctx))
	if !stop.Stop() || err != nil {
		cancel()
		if err == nil {
			_ = resp.Body.Close()
		}
		fatalf(t, "webuitest: GET %s: no stream response within %v: %v", req.URL.Path, b.timeout, err)
		return nil
	}
	s := newStream("GET "+req.URL.RequestURI(), resp, resp.Body, b.timeout, func() {
		cancel()
		_ = resp.Body.Close()
	})
	t.Cleanup(s.Close)
	return s
}
