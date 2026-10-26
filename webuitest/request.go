package webuitest

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/protolambda/chord/ct/cthttp"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
)

// BaseURL is the origin of the requests that [Get], [Post] and [Request]
// build: a reserved test domain over https, so that Secure and __Host-
// cookies and https-only logic behave as in production. Configure the
// application under test with this site, e.g. mw.MustParseSite(BaseURL).
const BaseURL = "https://app.example.test"

// DefaultRemoteAddr is the http.Request.RemoteAddr of requests built for
// handlers: a host of the documentation range of RFC 5737, and a port.
const DefaultRemoteAddr = "192.0.2.1:1234"

// Part is one field of a form body.
type Part struct {
	// Name is the field name.
	Name string
	// Value is the field value, or the content of a file.
	Value string
	// File marks a file part: it has a filename parameter (possibly empty,
	// as browsers send for a file input without a file) and a content type.
	File bool
	// Filename is the file name of a file part.
	Filename string
	// ContentType is the content type of a file part; "" means
	// application/octet-stream.
	ContentType string
}

type bodyKind uint8

const (
	bodyNone bodyKind = iota
	bodyForm
	bodyMultipart
	bodyText
	bodyRaw
)

// Body is a request body. The zero Body sends no body. A Body is an
// immutable value: [Body.With] returns a new one.
type Body struct {
	kind        bodyKind
	parts       []Part
	contentType string
	data        []byte
}

// Form returns an application/x-www-form-urlencoded body with values, in
// the order of [url.Values.Encode]: sorted by name, values in order.
func Form(values url.Values) Body {
	return Body{kind: bodyForm, parts: sortedParts(values)}
}

// Multipart returns a multipart/form-data body with the fields, sorted by
// name, values in order, as text parts.
func Multipart(fields map[string][]string) Body {
	return Body{kind: bodyMultipart, parts: sortedParts(fields)}
}

// Parts returns a multipart/form-data body with the parts in the given
// order, for file uploads and exact orders.
func Parts(parts ...Part) Body {
	return Body{kind: bodyMultipart, parts: append([]Part(nil), parts...)}
}

// Raw returns a body of the content type with a copy of data.
func Raw(contentType string, data []byte) Body {
	return Body{kind: bodyRaw, contentType: contentType, data: bytes.Clone(data)}
}

func sortedParts(values map[string][]string) []Part {
	names := make([]string, 0, len(values))
	for n := range values {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []Part
	for _, n := range names {
		for _, v := range values[n] {
			out = append(out, Part{Name: n, Value: v})
		}
	}
	return out
}

// Has reports whether a form body has a field called name. A raw body
// has no fields.
func (b Body) Has(name string) bool {
	for _, p := range b.parts {
		if p.Name == name {
			return true
		}
	}
	return false
}

// With returns the body with a text field appended. On a body without
// fields (the zero Body) it returns an urlencoded body with the field; on
// a raw body it returns b unchanged.
func (b Body) With(name, value string) Body {
	switch b.kind {
	case bodyRaw:
		return b
	case bodyNone:
		b.kind = bodyForm
	}
	b.parts = append(append([]Part(nil), b.parts...), Part{Name: name, Value: value})
	return b
}

// Fields returns a copy of the fields of a form body, in order.
func (b Body) Fields() []Part {
	return append([]Part(nil), b.parts...)
}

// ContentType returns the content type the body is sent with, "" for the
// zero Body.
func (b Body) ContentType() string {
	switch b.kind {
	case bodyForm:
		return "application/x-www-form-urlencoded"
	case bodyMultipart:
		return "multipart/form-data; boundary=" + b.boundary()
	case bodyText:
		return "text/plain"
	case bodyRaw:
		return b.contentType
	}
	return ""
}

// defaultBoundary is the multipart boundary of bodies whose parts do not
// contain it, fixed so that bodies are deterministic.
const defaultBoundary = "webuitest-form-boundary-5f3c9a1e"

func (b Body) boundary() string {
	bnd := defaultBoundary
	for i := 0; ; i++ {
		clash := false
		for _, p := range b.parts {
			if strings.Contains(p.Value, bnd) || strings.Contains(p.Name, bnd) || strings.Contains(p.Filename, bnd) {
				clash = true
				break
			}
		}
		if !clash {
			return bnd
		}
		bnd = fmt.Sprintf("%s-%d", defaultBoundary, i)
	}
}

// Bytes returns the encoded body.
func (b Body) Bytes() []byte {
	switch b.kind {
	case bodyForm:
		var s strings.Builder
		for i, p := range b.parts {
			if i > 0 {
				s.WriteByte('&')
			}
			v := p.Value
			if p.File {
				v = p.Filename
			}
			s.WriteString(url.QueryEscape(p.Name) + "=" + url.QueryEscape(v))
		}
		return []byte(s.String())
	case bodyText:
		var s strings.Builder
		for _, p := range b.parts {
			v := p.Value
			if p.File {
				v = p.Filename
			}
			s.WriteString(p.Name + "=" + v + "\r\n")
		}
		return []byte(s.String())
	case bodyMultipart:
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		if err := w.SetBoundary(b.boundary()); err != nil {
			// The boundary is a valid fixed token.
			panic(err)
		}
		for _, p := range b.parts {
			h := make(textproto.MIMEHeader)
			disp := `form-data; name="` + escapeFormName(p.Name) + `"`
			if p.File {
				disp += `; filename="` + escapeFormName(p.Filename) + `"`
				ct := p.ContentType
				if ct == "" {
					ct = "application/octet-stream"
				}
				h.Set("Content-Type", ct)
			}
			h.Set("Content-Disposition", disp)
			part, err := w.CreatePart(h)
			if err == nil {
				_, err = io.WriteString(part, p.Value)
			}
			if err != nil {
				// Writes to a bytes.Buffer do not fail.
				panic(err)
			}
		}
		if err := w.Close(); err != nil {
			panic(err)
		}
		return buf.Bytes()
	case bodyRaw:
		return bytes.Clone(b.data)
	}
	return nil
}

// escapeFormName escapes a name or file name in a multipart header as
// browsers do: LF, CR and the double quote as %0A, %0D and %22.
func escapeFormName(s string) string {
	return strings.NewReplacer("\n", "%0A", "\r", "%0D", `"`, "%22").Replace(s)
}

// ReqOption adapts a request built by [Get], [Post], [Request], a [Client]
// or a [Browser].
type ReqOption func(*reqSettings)

type headerOp struct {
	name, value string
	del, add    bool
}

type reqSettings struct {
	htmx, historyRestore, eventSource bool
	page                              *string
	target, currentURL                *string
	origin, referer, fetchSite        *string
	noFetchMetadata                   bool
	host, remoteAddr                  *string
	cookies                           []*http.Cookie
	headers                           []headerOp
	logger                            *slog.Logger
	urlMethods                        map[string]bool // of PageHTMXConfig
	// err is the first error of an option, which buildRequest reports.
	err error
}

// settings applies opts, skipping nil ones.
func settings(opts []ReqOption) reqSettings {
	var s reqSettings
	for _, o := range opts {
		if o != nil {
			o(&s)
		}
	}
	return s
}

func ptr(s string) *string { return &s }

// PageURL names the URL of the page that makes the request: a path (or an
// absolute URL), resolved against the base URL. The request target is
// resolved against it, as a browser resolves the URLs of a page, so a
// target relative to the page ("done", "?page=2", or "" for the page
// itself) works. [Client.Submit], [Browser.Submit] and
// [Submission.Request] need it for a form whose action is relative to the
// page or empty; without it, such a submission fails the test rather than
// go to a URL that no browser would request. The same holds for the
// requests of [Client.SubmitHTMX], [Browser.SubmitHTMX] and
// [HTMXRequest.Request]. An htmx request ([HTMX]) sends the resolved page
// URL as HX-Current-URL, as htmx does, unless [HXCurrentURL] sets another.
//
// The page is also where the request comes from: its origin is the
// default Origin (see [Origin]), and Sec-Fetch-Site compares it with the
// request's URL, as browsers do: "same-origin" for the same origin,
// "same-site" for another origin of the same site, and "cross-site"
// otherwise, unless [FetchSite] sets it. Two origins are of the same site
// when they have the same scheme and the same host, or DNS names that end
// in the same two labels: the registrable domain under a public suffix of
// one label, such as com or test. Under a public suffix of more labels,
// such as co.uk, that takes other sites for the same one: set FetchSite
// there.
func PageURL(u string) ReqOption { return func(s *reqSettings) { s.page = ptr(u) } }

// PageHTMXConfig names the htmx configuration of the page that makes the
// request (page.Document.HTMX), for [Client.SubmitHTMX] and
// [Browser.SubmitHTMX] of a fragment: a fragment has no htmx-config meta
// element of its own and runs under the configuration of its page, by
// default the zero hx2.Config. Only hx2.Config.DeleteURLParams changes the
// request (see [HTMXData]); a complete page always uses its own
// htmx-config meta element. It is the request counterpart of
// [WithHTMXConfig], and other requests ignore it. A config that does not
// encode (hx2.Config.JSON fails) is a programming error: PageHTMXConfig
// panics.
func PageHTMXConfig(c hx2.Config) ReqOption {
	methods := configURLMethods("webuitest.PageHTMXConfig", c)
	return func(s *reqSettings) { s.urlMethods = methods }
}

// HTMX marks the request as an htmx request: HX-Request: true, and the
// fetch metadata of a script request (Sec-Fetch-Mode: cors,
// Sec-Fetch-Dest: empty) instead of a navigation; with [PageURL], also
// HX-Current-URL.
//
// It changes only the headers: [Client.Submit] and [Browser.Submit] with
// HTMX still send the form's own submission, its method, action and
// fields, which models an htmx form whose hx-post (or other request
// attribute) matches its method and action, and they fail when the htmx
// request of the form or its submitter has another method or URL. hx-vals,
// hx-include and hx-params are not applied; [Client.SubmitHTMX] and
// [Browser.SubmitHTMX] send what htmx sends.
func HTMX() ReqOption { return func(s *reqSettings) { s.htmx = true } }

// HXTarget makes it an htmx request ([HTMX]) with HX-Target: id, the id of
// the target element.
func HXTarget(id string) ReqOption {
	return func(s *reqSettings) { s.htmx, s.target = true, ptr(id) }
}

// HXCurrentURL makes it an htmx request ([HTMX]) with HX-Current-URL: u,
// the URL of the page that makes the request. A path is resolved against
// the base URL.
func HXCurrentURL(u string) ReqOption {
	return func(s *reqSettings) { s.htmx, s.currentURL = true, ptr(u) }
}

// HXHistoryRestore makes it a history restore request of htmx:
// HX-History-Restore-Request: true, without HX-Request, as webui's htmx
// configuration sends it (historyRestoreAsHxRequest false).
func HXHistoryRestore() ReqOption {
	return func(s *reqSettings) { s.historyRestore = true }
}

// EventSource makes it the request of a browser EventSource: Accept:
// text/event-stream, Cache-Control: no-cache, and the fetch metadata of a
// script request.
func EventSource() ReqOption {
	return func(s *reqSettings) { s.eventSource = true }
}

// Origin sets the Origin header; "" removes it. By default, requests other
// than GET and HEAD carry the origin of the page that sends them, as
// browsers send it: the origin of [PageURL], or else that of the request's
// URL, the base URL for a path.
//
// It changes only the Origin: Sec-Fetch-Site stays as the page makes it
// (see [PageURL]), "same-origin" for a page of the request's origin, which
// mw.SameOrigin (like http.CrossOriginProtection) trusts before Origin,
// so Origin([ForeignOrigin]) alone builds a request that no browser sends,
// and it passes such a policy. For a cross-origin request of a current
// browser, add [FetchSite]("cross-site") (or "same-site"); for one of a
// client without fetch metadata, use [NoFetchMetadata] and set the Origin
// with [Header]; [CrossSiteProbes] has both kinds.
func Origin(origin string) ReqOption {
	return func(s *reqSettings) { s.origin = ptr(origin) }
}

// Referer sets the Referer header; "" removes it. Built requests have none
// by default.
func Referer(u string) ReqOption {
	return func(s *reqSettings) { s.referer = ptr(u) }
}

// FetchSite sets Sec-Fetch-Site, e.g. "cross-site", "same-site" or "none";
// "" removes it. The default compares the page with the request's URL
// (see [PageURL]): "same-origin" without a page of another origin.
func FetchSite(site string) ReqOption {
	return func(s *reqSettings) { s.fetchSite = ptr(site) }
}

// NoFetchMetadata removes Origin, Referer and every Sec-Fetch-* header, as
// a client that is not a browser sends a request. It applies after the
// other options except [Header], so add an Origin back with Header.
func NoFetchMetadata() ReqOption {
	return func(s *reqSettings) { s.noFetchMetadata = true }
}

// Host sets the Host header (and http.Request.Host), e.g. to a name of
// another site for DNS rebinding tests. The URL and the default Origin
// stay those of the request's site.
func Host(host string) ReqOption {
	return func(s *reqSettings) { s.host = ptr(host) }
}

// RemoteAddr sets http.Request.RemoteAddr of a request served by a
// handler, the client's host and port; the default is
// [DefaultRemoteAddr]. A [Browser] ignores it.
func RemoteAddr(addr string) ReqOption {
	return func(s *reqSettings) { s.remoteAddr = ptr(addr) }
}

// SendCookie adds a cookie to the Cookie header. A [Browser] sends the
// cookies of its jar as well. (The response assertion is [Cookie].)
func SendCookie(name, value string) ReqOption {
	return func(s *reqSettings) { s.cookies = append(s.cookies, &http.Cookie{Name: name, Value: value}) }
}

// SendCookies adds the cookies that res sets to the Cookie header, in
// order, for a [Client], which keeps no cookies, to send them back as a
// browser would: a cookie set twice goes with its last value, and one
// that clears itself (Max-Age zero or less, or without Max-Age an Expires
// date before the current time, as a cookie jar compares it) is left out.
// Domain, Path and Secure are not matched against the request; a
// [Browser] does that. A response that was not captured, or a Set-Cookie
// header that does not parse, fails the test when the request is built.
// [CookieValue] reads a single cookie.
func SendCookies(res *cthttp.Result) ReqOption {
	cookies, err := responseCookies(res)
	if err == nil {
		cookies = keptCookies(cookies, time.Now())
	}
	return func(s *reqSettings) {
		if err != nil {
			if s.err == nil {
				s.err = err
			}
			return
		}
		s.cookies = append(s.cookies, cookies...)
	}
}

// responseCookies returns the cookies that a captured response sets, in
// order, or an error if it was not captured or a Set-Cookie header does
// not parse. Errors never quote a header.
func responseCookies(res *cthttp.Result) ([]*http.Cookie, error) {
	if res == nil {
		return nil, errors.New("no response to take cookies from")
	}
	if err := res.Captured().Check(context.Background()); err != nil {
		return nil, fmt.Errorf("cookies of %s: %w", res.Describe(), err)
	}
	cookies, bad := setCookies(res)
	if bad > 0 {
		return nil, fmt.Errorf("cookies of %s: %d Set-Cookie headers do not parse", res.Describe(), bad)
	}
	return cookies, nil
}

// keptCookies returns name and value of the cookies that a cookie jar
// keeps at now, the last of each name, in the order of their first
// appearance.
func keptCookies(cookies []*http.Cookie, now time.Time) []*http.Cookie {
	var out []*http.Cookie
	at := make(map[string]int)
	for _, c := range cookies {
		kept := &http.Cookie{Name: c.Name, Value: c.Value}
		if cookieCleared(c, now) {
			kept = nil
		}
		if i, ok := at[c.Name]; ok {
			out[i] = kept
			continue
		}
		at[c.Name] = len(out)
		out = append(out, kept)
	}
	return slices.DeleteFunc(out, func(c *http.Cookie) bool { return c == nil })
}

// cookieCleared reports whether a parsed Set-Cookie deletes its cookie at
// now: Max-Age zero or less, or, without a Max-Age, an Expires date that
// is not after now. Max-Age takes precedence over Expires (RFC 6265,
// section 5.3).
func cookieCleared(c *http.Cookie, now time.Time) bool {
	// http.ParseSetCookie reports a Max-Age of zero or less as -1, and a
	// missing (or invalid, so ignored) one as 0.
	return c.MaxAge < 0 || (c.MaxAge == 0 && !c.Expires.IsZero() && !c.Expires.After(now))
}

// Header sets a request header after every other option; an empty value
// removes it.
func Header(name, value string) ReqOption {
	return func(s *reqSettings) {
		s.headers = append(s.headers, headerOp{name: name, value: value, del: value == ""})
	}
}

// addHeader adds a value to a request header after every other option, as
// [Header] sets one.
func addHeader(name, value string) ReqOption {
	return func(s *reqSettings) {
		s.headers = append(s.headers, headerOp{name: name, value: value, add: true})
	}
}

// LogTo attaches l to the request context with mw.WithLogger, so handlers
// that log through mw.Logger write to l (for example a [Logs] handler). A
// [Browser] ignores it: the server's requests have their own contexts.
func LogTo(l *slog.Logger) ReqOption {
	return func(s *reqSettings) { s.logger = l }
}

// safeMethod reports whether browsers send the method without an Origin
// header for same-origin requests.
func safeMethod(m string) bool { return m == http.MethodGet || m == http.MethodHead }

// buildRequest builds a request for target, resolved against base, with
// browser-realistic headers. A server request has the shape a handler
// receives (origin-form URL, RequestURI, RemoteAddr, TLS state); otherwise
// it is a client request for an http.Client.
func buildRequest(ctx context.Context, base *url.URL, method, target string, body Body, opts []ReqOption, server bool) (*http.Request, error) {
	s := settings(opts)
	if s.err != nil {
		return nil, s.err
	}
	from := base
	if s.page != nil {
		p, err := url.Parse(*s.page)
		if err != nil {
			return nil, fmt.Errorf("invalid page URL %q: %w", *s.page, err)
		}
		from = base.ResolveReference(p)
	}
	ref, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid request target %q: %w", target, err)
	}
	u := from.ResolveReference(ref)
	u.Fragment, u.RawFragment = "", ""
	if u.Path == "" {
		// A browser requests "/" for a URL without a path.
		u.Path, u.RawPath = "/", ""
	}
	if s.logger != nil {
		ctx = mw.WithLogger(ctx, s.logger)
	}
	var reader io.Reader
	data := body.Bytes()
	if body.kind != bodyNone {
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if ct := body.ContentType(); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	h := req.Header
	script := s.htmx || s.historyRestore || s.eventSource
	// The page that sends the request: a page of the site that the
	// request goes to, unless PageURL names the page.
	page := u
	if s.page != nil {
		page = from
	}
	h.Set("Sec-Fetch-Site", fetchSite(page, u))
	if script {
		h.Set("Sec-Fetch-Mode", "cors")
		h.Set("Sec-Fetch-Dest", "empty")
	} else {
		h.Set("Sec-Fetch-Mode", "navigate")
		h.Set("Sec-Fetch-Dest", "document")
		h.Set("Sec-Fetch-User", "?1")
	}
	if !safeMethod(method) {
		h.Set("Origin", urlOrigin(page))
	}
	if s.htmx {
		h.Set(hx2.HeaderRequest, "true")
	}
	if s.target != nil {
		h.Set(hx2.HeaderTarget, *s.target)
	}
	switch {
	case s.currentURL != nil:
		cu := *s.currentURL
		if r, err := url.Parse(cu); err == nil {
			cu = base.ResolveReference(r).String()
		}
		h.Set(hx2.HeaderCurrentURL, cu)
	case s.htmx && s.page != nil:
		// htmx sends the page's URL, location.href.
		h.Set(hx2.HeaderCurrentURL, from.String())
	}
	if s.historyRestore {
		h.Set(hx2.HeaderHistoryRestoreRequest, "true")
	}
	if s.eventSource {
		h.Set("Accept", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
	}
	setOrDel(h, "Origin", s.origin)
	setOrDel(h, "Referer", s.referer)
	setOrDel(h, "Sec-Fetch-Site", s.fetchSite)
	if s.noFetchMetadata {
		for _, name := range []string{"Origin", "Referer", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
			h.Del(name)
		}
	}
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	for _, op := range s.headers {
		switch {
		case op.del:
			h.Del(op.name)
		case op.add:
			h.Add(op.name, op.value)
		default:
			h.Set(op.name, op.value)
		}
	}
	if s.host != nil {
		req.Host = *s.host
	}
	if !server {
		return req, nil
	}
	if req.Body == nil {
		// Handlers may read and close the body of every request.
		req.Body = http.NoBody
	}
	req.RequestURI = u.RequestURI()
	req.URL = &url.URL{Path: u.Path, RawPath: u.RawPath, RawQuery: u.RawQuery}
	if req.Host == "" {
		req.Host = u.Host
	}
	req.RemoteAddr = DefaultRemoteAddr
	if s.remoteAddr != nil {
		req.RemoteAddr = *s.remoteAddr
	}
	if u.Scheme == "https" {
		req.TLS = &tls.ConnectionState{
			Version:           tls.VersionTLS13,
			HandshakeComplete: true,
			ServerName:        u.Hostname(),
		}
	}
	return req, nil
}

// fetchSite returns the Sec-Fetch-Site of a request from a page to u, as
// [PageURL] describes it.
func fetchSite(page, u *url.URL) string {
	switch {
	case urlOrigin(page) == urlOrigin(u):
		return "same-origin"
	case page.Scheme == u.Scheme && sameSiteHosts(page.Hostname(), u.Hostname()):
		return "same-site"
	}
	return "cross-site"
}

// sameSiteHosts reports whether two hosts are of the same site, as
// [PageURL] approximates it: the same host, or DNS names of two labels or
// more that end in the same two.
func sameSiteHosts(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return true
	}
	_, errA := netip.ParseAddr(a)
	_, errB := netip.ParseAddr(b)
	if errA == nil || errB == nil {
		return false
	}
	la, lb := strings.Split(a, "."), strings.Split(b, ".")
	return len(la) >= 2 && len(lb) >= 2 && slices.Equal(la[len(la)-2:], lb[len(lb)-2:])
}

func setOrDel(h http.Header, name string, v *string) {
	switch {
	case v == nil:
	case *v == "":
		h.Del(name)
	default:
		h.Set(name, *v)
	}
}

var defaultBase = mustParseBase(BaseURL)

func mustParseBase(raw string) *url.URL {
	u, err := parseBase(raw)
	if err != nil {
		panic(err)
	}
	return u
}

// parseBase parses the base URL of a [Client] or [Browser], a site URL as
// mw.ParseSite takes it: an https origin, or an http one of a loopback
// host, with an optional port and nothing else (a single "/" path is
// allowed). The result is a new URL of the site's origin, normalized as
// mw.ParseSite normalizes it and browsers serialize it: the scheme and
// host lower-cased, IPv6 in its canonical form, a default port dropped,
// other ports in plain decimal. So the requests name the host that
// mw.HostCheck(mw.MustParseSite(raw)) accepts.
func parseBase(raw string) (*url.URL, error) {
	site, err := mw.ParseSite(raw)
	if err != nil {
		return nil, fmt.Errorf("base URL %q: %w", raw, err)
	}
	return url.Parse(site.Origin())
}

// baseOr returns the parsed base URL raw, or a copy of [BaseURL]'s when
// raw is empty.
func baseOr(raw string) (*url.URL, error) {
	if raw == "" {
		u := *defaultBase
		return &u, nil
	}
	return parseBase(raw)
}

// Request builds a request as a browser on [BaseURL] sends it, for a
// handler: target is a path (or an absolute URL), resolved against
// BaseURL (or against [PageURL]), and the request is bound to
// t.Context(). It carries:
//
//   - Sec-Fetch-Site: same-origin (for a [PageURL] of another origin,
//     same-site or cross-site), and the Sec-Fetch-Mode, Sec-Fetch-Dest
//     and Sec-Fetch-User of a navigation (or of a script request for
//     [HTMX], [HXHistoryRestore] and [EventSource]);
//   - for methods other than GET and HEAD, the Origin of the page that
//     sends it: of [PageURL], or else of the request's URL (BaseURL for a
//     path);
//   - the content type of body;
//   - the server-side fields a handler sees: an origin-form URL,
//     RequestURI, Host, RemoteAddr ([DefaultRemoteAddr]) and, for https,
//     a TLS connection state.
//
// opts adjust it, in order. An invalid method or target fails the test.
func Request(t TB, method, target string, body Body, opts ...ReqOption) *http.Request {
	t.Helper()
	req, err := buildRequest(t.Context(), defaultBase, method, target, body, opts, true)
	if err != nil {
		fatalf(t, "webuitest: %v", err)
		return nil
	}
	return req
}

// Get builds a GET request, see [Request].
func Get(t TB, target string, opts ...ReqOption) *http.Request {
	t.Helper()
	return Request(t, http.MethodGet, target, Body{}, opts...)
}

// Post builds a POST request with body, see [Request].
func Post(t TB, target string, body Body, opts ...ReqOption) *http.Request {
	t.Helper()
	return Request(t, http.MethodPost, target, body, opts...)
}

// Serve serves req with h through [cthttp.Serve], synchronously, and
// returns the captured response. A nil request or handler fails the test.
// A handler that streams must end by itself; use [ServeStream] for live
// streams.
func Serve(t TB, h http.Handler, req *http.Request, opts ...cthttp.Option) *cthttp.Result {
	t.Helper()
	if h == nil || req == nil {
		fatalf(t, "webuitest.Serve: nil handler or request")
		return nil
	}
	return cthttp.Serve(h, req, opts...)
}
