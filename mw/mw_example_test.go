package mw_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/protolambda/webui/mw"
)

// The recommended stack, outermost first, in front of an application mux.
func ExampleChain() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "home") })
	mux.HandleFunc("POST /notes", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "saved")
	})

	site := mw.MustParseSite("https://app.example.com")
	h := mw.Chain(mux,
		mw.LogRequests(mw.RequestLog{Routes: mux, Logger: slog.New(slog.DiscardHandler)}),
		mw.BodyDeadline(mw.DefaultBodyTimeout),
		mw.WriteDeadline(mw.DefaultWriteTimeout),
		mw.SecurityHeaders(mw.Headers{}),
		mw.HostCheck(site),
		mw.SameOrigin(mw.OriginPolicy{}),
		mw.MaxBody(1<<20),
	)

	sameOrigin := httptest.NewRequest(http.MethodPost, "https://app.example.com/notes", strings.NewReader("text=hi"))
	sameOrigin.Header.Set("Sec-Fetch-Site", "same-origin")
	crossSite := httptest.NewRequest(http.MethodPost, "https://app.example.com/notes", strings.NewReader("text=hi"))
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "https://app.example.com/", nil),
		httptest.NewRequest(http.MethodGet, "https://rebound.example.net/", nil),
		sameOrigin,
		crossSite,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		fmt.Println(req.Method, req.Host, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// GET app.example.com 200 home
	// GET rebound.example.net 400 invalid host
	// POST app.example.com 201 saved
	// POST app.example.com 403 cross-origin request refused
}

func ExampleStrictCSP() {
	for d, values := range mw.StrictCSP().All() {
		fmt.Println(d, values)
	}
	// Output:
	// default-src ['none']
	// script-src ['self']
	// style-src ['self']
	// img-src ['self' data:]
	// font-src ['self']
	// connect-src ['self']
	// form-action ['self']
	// frame-ancestors ['none']
	// base-uri ['none']
}

// The zero CSP stands for StrictCSP, also when a policy is derived from it:
// widening the default of a zero Headers keeps the strict baseline. ParseCSP
// builds a policy without it.
func ExampleCSP() {
	var cfg mw.Headers
	cfg.CSP = cfg.CSP.Add(mw.ConnectSrc, "wss://live.example.com")
	fmt.Println(cfg.CSP)

	own, err := mw.ParseCSP("default-src 'self'")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(own.With(mw.FrameAncestors, mw.SourceNone))
	// Output:
	// default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self' wss://live.example.com; form-action 'self'; frame-ancestors 'none'; base-uri 'none'
	// default-src 'self'; frame-ancestors 'none'
}

// A page whose form submission redirects to another origin names that origin
// in form-action; the origin comes from a runtime value, so it is validated
// with HostSource first.
func ExampleCSP_Add() {
	src, err := mw.HostSource("https://partner.example.com/return?state=abc")
	if err != nil {
		fmt.Println(err)
		return
	}
	policy := mw.StrictCSP().
		Add(mw.FormAction, src).
		With(mw.ImgSrc, mw.SourceSelf).
		Without(mw.FontSrc)
	fmt.Println(policy)
	// Output:
	// default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self' https://partner.example.com; frame-ancestors 'none'; base-uri 'none'
}

func ExampleHostSource() {
	for _, target := range []string{
		"https://Partner.example.com:443/return?state=abc",
		"wss://live.example.com:8443/stream",
		"http://[::1]:8080/return",
		"javascript:alert(1)",
	} {
		src, err := mw.HostSource(target)
		fmt.Printf("%q %v\n", src, err)
	}
	// Output:
	// "https://partner.example.com" <nil>
	// "wss://live.example.com:8443" <nil>
	// "" mw: invalid content security policy: a CSP source cannot name an IP address
	// "" mw: invalid content security policy: the URL is not an absolute http(s) or ws(s) URL
}

func ExampleParseCSP() {
	policy, err := mw.ParseCSP("default-src 'self'; IMG-SRC 'self' data:;")
	if err != nil {
		fmt.Println(err)
		return
	}
	img, ok := policy.Values(mw.ImgSrc)
	fmt.Println(img, ok)
	fmt.Println(policy)

	_, err = mw.ParseCSP("img-src 'self'; script-src 'self' 'unsafe-inline' evil.example.com, default-src *")
	fmt.Println(err)
	// Output:
	// ['self' data:] true
	// default-src 'self'; img-src 'self' data:
	// mw: invalid content security policy: directive 2: value 3 is not a CSP token
}

func ExampleSecurityHeaders() {
	h := mw.SecurityHeaders(mw.Headers{
		HSTS: mw.HSTS{MaxAge: 365 * 24 * time.Hour},
	})(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://app.example.com/", nil))
	for _, name := range []string{
		"Content-Security-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options",
		"Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy", "Cache-Control", "Strict-Transport-Security",
	} {
		fmt.Printf("%s: %s\n", name, rec.Header().Get(name))
	}
	// Output:
	// Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'
	// Referrer-Policy: same-origin
	// X-Content-Type-Options: nosniff
	// X-Frame-Options: DENY
	// Cross-Origin-Opener-Policy: same-origin
	// Cross-Origin-Resource-Policy: same-origin
	// Cache-Control: no-store
	// Strict-Transport-Security: max-age=31536000
}

func ExampleSetCSP() {
	page := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The form on this page may post to the own origin and to a partner.
		mw.SetCSP(w, mw.StrictCSP().Add(mw.FormAction, "https://partner.example.com"))
		_, _ = io.WriteString(w, "<form>...</form>")
	})
	h := mw.SecurityHeaders(mw.Headers{})(page)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	policy, _ := mw.ParseCSP(rec.Header().Get("Content-Security-Policy"))
	formAction, _ := policy.Values(mw.FormAction)
	fmt.Println(formAction)
	// Output:
	// ['self' https://partner.example.com]
}

func ExampleParseSite() {
	for _, raw := range []string{
		"https://App.Example.com:443/",
		"http://localhost:8080",
		"http://[0:0:0:0:0:0:0:1]:8080",
		"http://app.example.com",
		"https://app.example.com/app",
	} {
		site, err := mw.ParseSite(raw)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(site.Origin(), site.Host(), site.HTTPS(), site.Loopback())
	}
	// Output:
	// https://app.example.com app.example.com true false
	// http://localhost:8080 localhost:8080 false true
	// http://[::1]:8080 [::1]:8080 false true
	// mw: invalid site URL: the URL must be https unless its host is a loopback host
	// mw: invalid site URL: the URL must be an origin, without path, query or fragment
}

func ExampleSite_MatchHost() {
	site := mw.MustParseSite("https://app.example.com")
	for _, host := range []string{"app.example.com", "APP.example.com.:443", "app.example.com:8443", "app.example.com.evil.example"} {
		fmt.Println(host, site.MatchHost(host))
	}
	// Output:
	// app.example.com true
	// APP.example.com.:443 true
	// app.example.com:8443 false
	// app.example.com.evil.example false
}

func ExampleHostCheck() {
	public := mw.MustParseSite("https://app.example.com")
	local := mw.MustParseSite("http://localhost:8080")
	h := mw.HostCheck(public, local)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site, _ := mw.SiteFrom(r.Context())
		_, _ = fmt.Fprintf(w, "served for %s", site)
	}))
	for _, host := range []string{"app.example.com", "localhost:8080", "attacker.example"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		fmt.Println(rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// 200 served for https://app.example.com
	// 200 served for http://localhost:8080
	// 400 invalid host
}

func ExampleOriginPolicy_Check() {
	public := mw.MustParseSite("https://app.example.com")
	local := mw.MustParseSite("http://127.0.0.1:8080")
	check := func(site mw.Site, header, value string) {
		req := httptest.NewRequest(http.MethodPost, "/notes", nil)
		if header != "" {
			req.Header.Set(header, value)
		}
		err := mw.OriginPolicy{}.Check(req, site)
		fmt.Printf("%s %s=%q: %v\n", site, header, value, err)
	}
	check(public, "Sec-Fetch-Site", "same-origin")
	check(public, "Sec-Fetch-Site", "same-site")
	check(public, "Origin", "https://app.example.com")
	check(public, "Origin", "http://app.example.com")
	check(public, "Referer", "https://app.example.com/notes/new")
	check(public, "", "")
	check(local, "", "")
	// Output:
	// https://app.example.com Sec-Fetch-Site="same-origin": <nil>
	// https://app.example.com Sec-Fetch-Site="same-site": mw: cross-origin request: Sec-Fetch-Site names another origin
	// https://app.example.com Origin="https://app.example.com": <nil>
	// https://app.example.com Origin="http://app.example.com": mw: cross-origin request: Origin differs from the site
	// https://app.example.com Referer="https://app.example.com/notes/new": <nil>
	// https://app.example.com ="": mw: cross-origin request: no Sec-Fetch-Site, Origin or Referer
	// http://127.0.0.1:8080 ="": <nil>
}

// A custom deny handler, and the stdlib-like choice for header-less requests
// on a route that non-browser clients call with their own credentials.
func ExampleSameOrigin() {
	site := mw.MustParseSite("https://app.example.com")
	deny := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "This form expired. Reload the page and try again.", http.StatusForbidden)
	})
	h := mw.Chain(http.NotFoundHandler(),
		mw.HostCheck(site),
		mw.SameOrigin(mw.OriginPolicy{Headerless: mw.HeaderlessAllow, Deny: deny}),
	)
	crossSite := httptest.NewRequest(http.MethodPost, "https://app.example.com/notes", nil)
	crossSite.Header.Set("Origin", "https://attacker.example")
	headerless := httptest.NewRequest(http.MethodPost, "https://app.example.com/notes", nil)
	for _, req := range []*http.Request{crossSite, headerless} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		fmt.Println(rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// 403 This form expired. Reload the page and try again.
	// 404 404 page not found
}

// A WebSocket handshake is a GET request, but pages of other origins could
// read and write the socket it opens, so SameOrigin checks it like a POST.
func ExampleSameOrigin_webSocket() {
	site := mw.MustParseSite("https://app.example.com")
	live := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "handshake accepted") // a WebSocket library takes over here
	})
	h := mw.Chain(live, mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{}))
	for _, origin := range []string{"https://app.example.com", "https://other.example.com"} {
		req := httptest.NewRequest(http.MethodGet, "https://app.example.com/live", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		fmt.Println(origin, rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// https://app.example.com 200 handshake accepted
	// https://other.example.com 403 cross-origin request refused
}

func ExampleLogRequests() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey || a.Key == "duration" {
				return slog.Attr{} // for a stable output
			}
			return a
		},
	}))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		mw.Logger(r.Context()).Info("note loaded")
		_, _ = io.WriteString(w, "note")
	})
	h := mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux})(mux)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/notes/42?share=private-token", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private/path", nil))
	// Output:
	// level=INFO msg="note loaded" method=GET pattern="GET /notes/{id}"
	// level=INFO msg="http request" method=GET pattern="GET /notes/{id}" status=200 bytes=4
	// level=INFO msg="http request" method=GET pattern="" status=404 bytes=19
}

func ExampleLogger() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	save := func(r *http.Request) {
		// Handlers log through the context logger; without one it is slog.Default().
		mw.Logger(r.Context()).Warn("saving failed", "err", errors.New("disk full"))
	}
	req := httptest.NewRequest(http.MethodPost, "/notes", nil)
	save(req.WithContext(mw.WithLogger(req.Context(), logger.With("app", "notes"))))
	// Output:
	// level=WARN msg="saving failed" app=notes err="disk full"
}

// A route that expects slow uploads extends its read deadline itself.
func ExampleBodyDeadline() {
	upload := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Minute))
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			http.Error(w, "upload failed", http.StatusBadRequest)
			return
		}
		_, _ = fmt.Fprintf(w, "received %d bytes", n)
	})
	h := mw.BodyDeadline(mw.DefaultBodyTimeout)(upload)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("file contents")))
	fmt.Println(rec.Body.String())
	// Output:
	// received 13 bytes
}

// A stream takes the write deadline over with one deadline per event.
func ExampleWriteDeadline() {
	events := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		for i := range 3 {
			_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, _ = fmt.Fprintf(w, "data: %d\n\n", i)
			if err := rc.Flush(); err != nil {
				return
			}
		}
	})
	h := mw.WriteDeadline(mw.DefaultWriteTimeout)(events)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
	fmt.Print(rec.Body.String())
	fmt.Println("flushed:", rec.Flushed)
	// Output:
	// data: 0
	//
	// data: 1
	//
	// data: 2
	//
	// flushed: true
}

func ExampleMaxBody() {
	save := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, fmt.Sprintf("at most %d bytes", tooLarge.Limit), http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = fmt.Fprintf(w, "saved %d bytes", len(body))
	})
	h := mw.MaxBody(8)(save)
	for _, body := range []string{"short", "far too long"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(body)))
		fmt.Println(rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	// Output:
	// 200 saved 5 bytes
	// 413 at most 8 bytes
}
