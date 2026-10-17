package mw

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// DefaultPermissionsPolicy denies the powerful browser features that a web
// user interface rarely needs: sensors, camera, microphone, screen capture,
// location, device access (HID, MIDI, serial, USB), payments, idle detection,
// XR, and the interest-based advertising topics. Features that ordinary pages
// do use, such as the clipboard, fullscreen and passkeys, are left to the
// browser's defaults.
const DefaultPermissionsPolicy = "accelerometer=(), browsing-topics=(), camera=(), display-capture=(), " +
	"geolocation=(), gyroscope=(), hid=(), idle-detection=(), magnetometer=(), microphone=(), midi=(), " +
	"payment=(), serial=(), usb=(), xr-spatial-tracking=()"

// ErrInvalidHeaders reports a [Headers] configuration that cannot be sent.
var ErrInvalidHeaders = errors.New("mw: invalid security headers")

// Headers configures [SecurityHeaders]. The zero value is webui's strict
// configuration; every field's documentation names its default. Values must
// be visible ASCII and spaces ([Headers.Validate]).
type Headers struct {
	// CSP is the Content-Security-Policy. The zero CSP means [StrictCSP],
	// also as the receiver of [CSP.With], [CSP.Add] and [CSP.Without], so
	// h.CSP = h.CSP.Add(...) widens the strict default.
	CSP CSP

	// ReferrerPolicy is the Referrer-Policy. "" means "same-origin": other
	// origins learn nothing of the URL (whose path or query may carry
	// capabilities), while the own origin gets the full referrer. Avoid
	// "no-referrer": under it, browsers serialize the Origin header of every
	// request other than GET and HEAD as "null", which [SameOrigin] then
	// refuses.
	ReferrerPolicy string

	// PermissionsPolicy is the Permissions-Policy. "" means
	// [DefaultPermissionsPolicy].
	PermissionsPolicy string

	// CacheControl is the default Cache-Control. "" means "no-store": pages
	// may show private data and must not be kept by the browser or a proxy.
	// Handlers that serve cacheable content, such as versioned assets, set
	// their own Cache-Control, which replaces this one.
	CacheControl string

	// CrossOriginOpenerPolicy is the Cross-Origin-Opener-Policy. "" means
	// "same-origin": a page opened from another origin, or opening one, gets
	// no handle on the other window.
	CrossOriginOpenerPolicy string

	// CrossOriginResourcePolicy is the Cross-Origin-Resource-Policy. "" means
	// "same-origin": other origins cannot embed the responses (images,
	// scripts, styles).
	CrossOriginResourcePolicy string

	// HSTS configures Strict-Transport-Security. The zero value sends none,
	// so an https deployment sets it: [HSTSFor] derives the usual policy from
	// the configured sites, unless a TLS-terminating proxy in front of the
	// application sends the header already.
	HSTS HSTS
}

// HSTS configures the Strict-Transport-Security header, which makes browsers
// use https for the host (and optionally its subdomains) for MaxAge.
//
// It is set from configuration only ([HSTSFor] derives it from the configured
// sites), never inferred from a request: behind a TLS-terminating proxy a
// request says http, and a test or a plain-http loopback site must not pin a
// host to https by accident. Send it only for sites served over https.
// Browsers ignore the header on plain-http responses, apply it to the host
// name on every port, and keep it for MaxAge also after the header is gone.
type HSTS struct {
	// MaxAge is how long browsers remember the policy. Zero sends no header;
	// otherwise it must be at least one second, and is sent in whole seconds.
	// A year (365 * 24 * time.Hour) is usual once https works.
	MaxAge time.Duration
	// IncludeSubDomains extends the policy to every subdomain of the host.
	IncludeSubDomains bool
	// Preload asks for inclusion in the browsers' preload lists, which
	// requires IncludeSubDomains and a MaxAge of at least a year, and is hard
	// to undo.
	Preload bool
}

// DefaultHSTSMaxAge is the MaxAge that [HSTSFor] sets: a year, the usual
// value for a site that works over https, and the minimum of the browsers'
// preload lists.
const DefaultHSTSMaxAge = 365 * 24 * time.Hour

// hstsPreloadMinAge is the minimum max-age of the browsers' preload lists.
const hstsPreloadMinAge = DefaultHSTSMaxAge

// HSTSFor returns the Strict-Transport-Security policy of an application
// served at sites, the sites that [HostCheck] gets: a MaxAge of
// [DefaultHSTSMaxAge] when every site is https and none is a loopback host,
// else the zero HSTS, which sends no header.
//
//	site := mw.MustParseSite("https://app.example.com")
//	mw.SecurityHeaders(mw.Headers{HSTS: mw.HSTSFor(site)})
//
// The policy follows the configuration, never a request. Loopback sites get
// none, also over https: a browser that remembered https for localhost would
// refuse the plain-http servers that other programs run there. A mix of
// sites, such as an https site and a plain-http loopback one, gets none
// either, since one header serves them all; set HSTS explicitly for it
// (browsers ignore the header on plain-http responses). No sites, or a zero
// Site, get none.
//
// IncludeSubDomains and Preload stay off: set them on the result when every
// subdomain is served over https too. Leave HSTS zero when a TLS-terminating
// proxy in front of the application sends the header already.
func HSTSFor(sites ...Site) HSTS {
	if len(sites) == 0 {
		return HSTS{}
	}
	for _, s := range sites {
		if !s.HTTPS() || s.Loopback() {
			return HSTS{}
		}
	}
	return HSTS{MaxAge: DefaultHSTSMaxAge}
}

// String returns the header value, or "" for the zero HSTS.
func (h HSTS) String() string {
	if h.MaxAge <= 0 {
		return ""
	}
	v := "max-age=" + strconv.FormatInt(int64(h.MaxAge/time.Second), 10)
	if h.IncludeSubDomains {
		v += "; includeSubDomains"
	}
	if h.Preload {
		v += "; preload"
	}
	return v
}

// Validate reports whether the configuration can be sent, with an error
// wrapping [ErrInvalidHeaders] if not: header values must be visible ASCII and
// spaces, and HSTS needs a MaxAge of at least a second (and Preload its
// requirements) when any of its fields is set.
func (h Headers) Validate() error {
	fields := []struct{ name, value string }{
		{"ReferrerPolicy", h.ReferrerPolicy},
		{"PermissionsPolicy", h.PermissionsPolicy},
		{"CacheControl", h.CacheControl},
		{"CrossOriginOpenerPolicy", h.CrossOriginOpenerPolicy},
		{"CrossOriginResourcePolicy", h.CrossOriginResourcePolicy},
	}
	for _, f := range fields {
		if !validHeaderValue(f.value) {
			return fmt.Errorf("%w: %s must be visible ASCII and spaces", ErrInvalidHeaders, f.name)
		}
	}
	switch hsts := h.HSTS; {
	case hsts == HSTS{}:
	case hsts.MaxAge < time.Second:
		return fmt.Errorf("%w: HSTS.MaxAge must be at least a second", ErrInvalidHeaders)
	case hsts.Preload && (!hsts.IncludeSubDomains || hsts.MaxAge < hstsPreloadMinAge):
		return fmt.Errorf("%w: HSTS.Preload needs IncludeSubDomains and a MaxAge of at least a year", ErrInvalidHeaders)
	}
	return nil
}

const hstsHeader = "Strict-Transport-Security"

// headerField is a header with its canonical key.
type headerField struct{ key, value string }

// fields returns the headers of a valid configuration, in a fixed order.
func (h Headers) fields() []headerField {
	csp := h.CSP
	if csp.IsZero() {
		csp = StrictCSP()
	}
	out := []headerField{
		{cspHeader, csp.String()},
		{"Referrer-Policy", or(h.ReferrerPolicy, "same-origin")},
		{"X-Content-Type-Options", "nosniff"},
	}
	if fo := frameOptions(csp); fo != "" {
		out = append(out, headerField{"X-Frame-Options", fo})
	}
	out = append(out,
		headerField{"Cross-Origin-Opener-Policy", or(h.CrossOriginOpenerPolicy, "same-origin")},
		headerField{"Cross-Origin-Resource-Policy", or(h.CrossOriginResourcePolicy, "same-origin")},
		headerField{"Permissions-Policy", or(h.PermissionsPolicy, DefaultPermissionsPolicy)},
		headerField{"Cache-Control", or(h.CacheControl, "no-store")},
	)
	if v := h.HSTS.String(); v != "" {
		out = append(out, headerField{hstsHeader, v})
	}
	return out
}

// SecurityHeaders sets the security headers of cfg on every response before
// the next handler runs, so rejections and error pages carry them too:
//
//   - Content-Security-Policy: cfg.CSP, by default [StrictCSP];
//   - Referrer-Policy: same-origin;
//   - X-Content-Type-Options: nosniff;
//   - X-Frame-Options: DENY, or SAMEORIGIN when the policy's frame-ancestors
//     is exactly 'self', and none when frame-ancestors allows more, which
//     the older header cannot express (browsers that know frame-ancestors
//     ignore X-Frame-Options);
//   - Cross-Origin-Opener-Policy and Cross-Origin-Resource-Policy:
//     same-origin;
//   - Permissions-Policy: [DefaultPermissionsPolicy];
//   - Cache-Control: no-store;
//   - Strict-Transport-Security: only when cfg.HSTS is set (see [HSTSFor]);
//     [HostCheck] removes it from its refusals.
//
// Handlers may replace or delete any of them before they write the response,
// for example a Cache-Control for immutable assets, or a per-page policy with
// [SetCSP].
//
// SecurityHeaders panics if cfg is invalid ([Headers.Validate]); the
// configuration is the program's own.
func SecurityHeaders(cfg Headers) Middleware {
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	fields := cfg.fields()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			for _, f := range fields {
				h[f.key] = []string{f.value}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// validHeaderValue reports whether s is visible ASCII and spaces.
func validHeaderValue(s string) bool {
	return allBytes(s, func(c byte) bool { return c >= 0x20 && c < 0x7f })
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
