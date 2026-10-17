package mw

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrCrossOrigin reports a request that [OriginPolicy.Check] refuses. The
// errors that Check returns wrap it with a fixed reason that never quotes the
// request.
var ErrCrossOrigin = errors.New("mw: cross-origin request")

var (
	errNoSite    = fmt.Errorf("%w: no site in the request context", ErrCrossOrigin)
	errRepeated  = fmt.Errorf("%w: an origin header is repeated", ErrCrossOrigin)
	errFetchSite = fmt.Errorf("%w: Sec-Fetch-Site names another origin", ErrCrossOrigin)
	errOrigin    = fmt.Errorf("%w: Origin differs from the site", ErrCrossOrigin)
	errReferer   = fmt.Errorf("%w: Referer differs from the site", ErrCrossOrigin)
	errNoHeaders = fmt.Errorf("%w: no Sec-Fetch-Site, Origin or Referer", ErrCrossOrigin)
)

// maxRefererSize bounds the Referer that Check parses; longer ones do not
// match.
const maxRefererSize = 8 << 10

// Headerless decides about requests that carry none of Sec-Fetch-Site, Origin
// and Referer. Browsers send at least one of them with every cross-origin
// request other than GET and HEAD, so such a request is either same-origin
// from a client that strips the headers (some privacy tools, local scripts)
// or not from a browser at all.
type Headerless uint8

const (
	// HeaderlessLoopback, the zero value, accepts header-less requests on
	// loopback sites only, where local tools and privacy settings commonly
	// strip the headers, and refuses them on every other site.
	HeaderlessLoopback Headerless = iota
	// HeaderlessDeny refuses header-less requests on every site.
	HeaderlessDeny
	// HeaderlessAllow accepts header-less requests on every site, as
	// [http.CrossOriginProtection] does. Choose it for handlers that serve
	// non-browser clients too, which must then authenticate by other means
	// than cookies.
	HeaderlessAllow
)

// String returns the name of the constant, or "Headerless(n)" for an unknown
// value.
func (h Headerless) String() string {
	switch h {
	case HeaderlessLoopback:
		return "HeaderlessLoopback"
	case HeaderlessDeny:
		return "HeaderlessDeny"
	case HeaderlessAllow:
		return "HeaderlessAllow"
	}
	return "Headerless(" + strconv.Itoa(int(h)) + ")"
}

// OriginPolicy configures [SameOrigin]. The zero value is the strict policy.
type OriginPolicy struct {
	// Headerless decides about requests without any origin header. The zero
	// value accepts them on loopback sites only; unknown values refuse them.
	Headerless Headerless
	// Deny answers refused requests. Nil means 403 Forbidden with a short
	// plain-text message.
	Deny http.Handler
}

// Check reports whether a request comes from a page of site, with nil if it
// does or is a GET or HEAD request, else with an error wrapping
// [ErrCrossOrigin]. In order:
//
//  1. If Sec-Fetch-Site is present, it decides: "same-origin" and "none" (a
//     navigation the user started, such as a bookmark) pass; "same-site" is
//     another origin under the same registrable domain and is refused like
//     "cross-site" and any other value.
//  2. Else, if Origin is present, it must equal the site's origin, scheme
//     included (case-insensitively). "null" never does.
//  3. Else, if Referer is present, its origin must equal the site's origin.
//  4. Else the request has no origin headers, and p.Headerless decides.
//
// A repeated Sec-Fetch-Site, Origin or Referer header is refused. The zero
// Site is refused. Check never consults the Host header: the site comes from
// configuration (see [HostCheck]).
func (p OriginPolicy) Check(r *http.Request, site Site) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return nil
	}
	if site.IsZero() {
		return errNoSite
	}
	fetchSite, origin, referer := r.Header.Values("Sec-Fetch-Site"), r.Header.Values("Origin"), r.Header.Values("Referer")
	if len(fetchSite) > 1 || len(origin) > 1 || len(referer) > 1 {
		return errRepeated
	}
	switch {
	case len(fetchSite) == 1:
		if fetchSite[0] == "same-origin" || fetchSite[0] == "none" {
			return nil
		}
		return errFetchSite
	case len(origin) == 1:
		if strings.EqualFold(origin[0], site.origin) {
			return nil
		}
		return errOrigin
	case len(referer) == 1:
		if o, ok := refererOrigin(referer[0]); ok && strings.EqualFold(o, site.origin) {
			return nil
		}
		return errReferer
	}
	switch p.Headerless {
	case HeaderlessLoopback:
		if site.loopback {
			return nil
		}
	case HeaderlessAllow:
		return nil
	}
	return errNoHeaders
}

// refererOrigin returns the origin of an absolute http(s) Referer: the
// lower-case scheme and host, without a default port.
func refererOrigin(ref string) (string, bool) {
	if len(ref) > maxRefererSize {
		return "", false
	}
	u, err := url.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	name, port, ok := splitHost(strings.ToLower(u.Host))
	if !ok {
		return "", false
	}
	if port == "" || port == strconv.Itoa(defaultPort(u.Scheme)) {
		return u.Scheme + "://" + name, true
	}
	return u.Scheme + "://" + name + ":" + port, true
}

// SameOrigin refuses every request other than GET and HEAD that does not come
// from a page of the request's site ([OriginPolicy.Check]): this is the
// origin layer of CSRF protection. The site comes from the request context,
// so SameOrigin must run inside [HostCheck]; without a site, every such
// request is refused. Refusals are logged at debug level to [Logger] with
// their reason.
//
// GET and HEAD always pass, so they must never change state. OPTIONS is
// checked like any other method (no CORS preflight passes).
//
// Keep a second, independent CSRF layer, such as a synchronizer token in each
// form: the origin layer depends on browsers sending the headers, and passes
// header-less requests on loopback sites.
//
// # Compared with http.CrossOriginProtection
//
// [http.CrossOriginProtection] (Go 1.25) checks Sec-Fetch-Site the same way,
// but differs where it falls back:
//
//   - It exempts OPTIONS besides GET and HEAD.
//   - Without Sec-Fetch-Site, it compares the host of Origin with the Host
//     header and ignores the scheme, so an http page may post to the https
//     site; SameOrigin compares Origin with the configured site origin,
//     scheme included.
//   - Without Sec-Fetch-Site and Origin, it accepts the request as a
//     non-browser one; SameOrigin falls back to Referer, and then accepts
//     only on loopback sites (see [Headerless] for the other choices).
//   - It offers trusted origins and bypass patterns; SameOrigin has neither:
//     mount routes that other origins or non-browser clients may call outside
//     of it instead.
//
// Neither defends against DNS rebinding by itself, which [HostCheck] does.
// Use SameOrigin for browser-facing applications that know their sites (the
// webui default). Use http.CrossOriginProtection for handlers that also serve
// non-browser clients that send no headers, that must accept posts from
// listed other origins, or that do not know their public origin. Layering
// both requires a request to pass both.
func SameOrigin(p OriginPolicy) Middleware {
	deny := p.Deny
	if deny == nil {
		deny = http.HandlerFunc(denyCrossOrigin)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			site, _ := SiteFrom(r.Context())
			if err := p.Check(r, site); err != nil {
				Logger(r.Context()).LogAttrs(r.Context(), slog.LevelDebug, "refused a cross-origin request",
					slog.String("method", sanitizeMethod(r.Method)), slog.String("reason", err.Error()))
				deny.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func denyCrossOrigin(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "cross-origin request refused", http.StatusForbidden)
}
