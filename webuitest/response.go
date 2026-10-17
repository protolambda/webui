package webuitest

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/mustbe/assertion"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
)

// Location asserts a redirect: the response has the status (usually
// http.StatusSeeOther) and a Location header that matches want.
func Location(res *cthttp.Result, status int, want ct.ValueMatch) assertion.Assertion {
	return All(fmt.Sprintf("location(%s, %d, %s)", res.Describe(), status, want),
		res.Status(status), res.Header("Location", want))
}

// HXRedirect asserts the redirect of an htmx request: an HX-Redirect header
// that matches want, on a response that is not a 3xx redirect (the browser
// follows those itself, so htmx never sees their headers).
func HXRedirect(res *cthttp.Result, want ct.ValueMatch) assertion.Assertion {
	return All(fmt.Sprintf("hxRedirect(%s, %s)", res.Describe(), want),
		notRedirect{res}, res.Header(hx2.HeaderRedirect, want))
}

type notRedirect struct{ res *cthttp.Result }

func (a notRedirect) String() string { return "notRedirect(" + a.res.Describe() + ")" }

func (a notRedirect) Check(ctx context.Context) error {
	if err := a.res.Captured().Check(ctx); err != nil {
		return err
	}
	if code := a.res.StatusCode(); code >= 300 && code < 400 {
		return fmt.Errorf("%w: %s has status %d: a browser follows a redirect before htmx sees the response", ct.ErrMismatch, a.res.Describe(), code)
	}
	return nil
}

// HXRetarget asserts an HX-Retarget header that matches want, as an error
// fragment that goes to another element than the request's target sends
// it.
func HXRetarget(res *cthttp.Result, want ct.ValueMatch) assertion.Assertion {
	return res.Header(hx2.HeaderRetarget, want)
}

// HXRefresh asserts HX-Refresh: true, which makes htmx reload the page.
func HXRefresh(res *cthttp.Result) assertion.Assertion {
	return res.Header(hx2.HeaderRefresh, ct.Exact("true"))
}

// CookieWant is what [Cookie] expects of a cookie. Boolean fields that are
// false and zero fields are not checked, except that a cookie that clears
// itself fails unless Cleared is set.
type CookieWant struct {
	// HostPrefix requires the __Host- prefix and what it implies: Secure,
	// Path=/ and no Domain, so the cookie stays with exactly this origin.
	HostPrefix bool
	// Secure requires the Secure attribute.
	Secure bool
	// HttpOnly requires the HttpOnly attribute.
	HttpOnly bool
	// SameSite, when not zero, requires that SameSite mode. (A cookie
	// without the attribute has the zero mode; http.SameSiteDefaultMode is
	// an attribute without a valid value.)
	SameSite http.SameSite
	// Path, when not "", requires that Path.
	Path string
	// Session requires a session cookie: no Max-Age and no Expires.
	Session bool
	// MaxAge, when not zero, requires that Max-Age, in whole seconds.
	MaxAge time.Duration
	// Cleared requires a cookie that deletes itself: Max-Age zero or less,
	// or, without a Max-Age, an Expires date before Now. Max-Age takes
	// precedence over Expires (RFC 6265, section 5.3), so a cookie with a
	// positive Max-Age is kept whatever its Expires says.
	Cleared bool
	// Now is the time that an Expires date is compared with, for an
	// application with an injected clock; zero means the current time.
	Now time.Time
}

// Cookie asserts that the response sets exactly one cookie called name,
// with the attributes of want. The cookie's value is never part of a
// message.
func Cookie(res *cthttp.Result, name string, want CookieWant) assertion.Assertion {
	return cookieAssertion{res: res, name: name, want: want}
}

type cookieAssertion struct {
	res  *cthttp.Result
	name string
	want CookieWant
}

func (a cookieAssertion) String() string {
	return fmt.Sprintf("cookie(%s, %q)", a.res.Describe(), a.name)
}

// setCookies parses the Set-Cookie headers of a response.
func setCookies(res *cthttp.Result) (cookies []*http.Cookie, bad int) {
	for _, line := range res.Headers().Values("Set-Cookie") {
		c, err := http.ParseSetCookie(line)
		if err != nil {
			bad++
			continue
		}
		cookies = append(cookies, c)
	}
	return cookies, bad
}

func (a cookieAssertion) Check(ctx context.Context) error {
	if err := a.res.Captured().Check(ctx); err != nil {
		return err
	}
	cookies, bad := setCookies(a.res)
	var found []*http.Cookie
	var names []string
	for _, c := range cookies {
		names = append(names, c.Name)
		if c.Name == a.name {
			found = append(found, c)
		}
	}
	switch {
	case len(found) == 0:
		msg := fmt.Sprintf("%s sets no cookie %q", a.res.Describe(), a.name)
		if len(names) > 0 {
			msg += " (it sets " + strings.Join(names, ", ") + ")"
		}
		if bad > 0 {
			msg += fmt.Sprintf("; %d Set-Cookie headers do not parse", bad)
		}
		return fmt.Errorf("%w: %s", ct.ErrMismatch, msg)
	case len(found) > 1:
		return fmt.Errorf("%w: %s sets cookie %q %d times", ct.ErrMismatch, a.res.Describe(), a.name, len(found))
	}
	c, w := found[0], a.want
	var problems []string
	if w.HostPrefix {
		if !strings.HasPrefix(c.Name, "__Host-") {
			problems = append(problems, "the name lacks the __Host- prefix")
		}
		if !c.Secure {
			problems = append(problems, "a __Host- cookie needs Secure")
		}
		if c.Path != "/" {
			problems = append(problems, fmt.Sprintf("a __Host- cookie needs Path=/, has %q", c.Path))
		}
		if c.Domain != "" {
			problems = append(problems, "a __Host- cookie must not have a Domain")
		}
	}
	if w.Secure && !c.Secure && !w.HostPrefix {
		problems = append(problems, "not Secure")
	}
	if w.HttpOnly && !c.HttpOnly {
		problems = append(problems, "not HttpOnly")
	}
	if w.SameSite != 0 && c.SameSite != w.SameSite {
		problems = append(problems, fmt.Sprintf("SameSite is %s, want %s", sameSiteName(c.SameSite), sameSiteName(w.SameSite)))
	}
	if w.Path != "" && c.Path != w.Path {
		problems = append(problems, fmt.Sprintf("Path is %q, want %q", c.Path, w.Path))
	}
	now := w.Now
	if now.IsZero() {
		now = time.Now()
	}
	// http.ParseSetCookie reports a Max-Age of zero or less as -1, and a
	// missing (or invalid, so ignored) one as 0.
	cleared := c.MaxAge < 0 || (c.MaxAge == 0 && !c.Expires.IsZero() && !c.Expires.After(now))
	switch {
	case w.Cleared && !cleared:
		problems = append(problems, "not cleared (no Max-Age<=0, or no Max-Age and a past Expires)")
	case !w.Cleared && cleared:
		problems = append(problems, "the cookie is cleared")
	}
	if w.Session && (c.MaxAge != 0 || c.RawExpires != "") {
		problems = append(problems, "has Max-Age or Expires, want a session cookie")
	}
	if w.MaxAge != 0 && c.MaxAge != int(w.MaxAge/time.Second) {
		problems = append(problems, fmt.Sprintf("Max-Age is %d, want %d", c.MaxAge, int(w.MaxAge/time.Second)))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: cookie %q of %s: %s", ct.ErrMismatch, a.name, a.res.Describe(), strings.Join(problems, "; "))
	}
	return nil
}

func sameSiteName(s http.SameSite) string {
	switch s {
	case 0:
		return "unset"
	case http.SameSiteDefaultMode:
		return "default"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	}
	return fmt.Sprintf("SameSite(%d)", int(s))
}

// NoCookies asserts that the response sets no cookie, as refusals and
// public pages should not. A failure names the cookies, never their values.
func NoCookies(res *cthttp.Result) assertion.Assertion {
	return noCookies{res}
}

type noCookies struct{ res *cthttp.Result }

func (a noCookies) String() string { return "noCookies(" + a.res.Describe() + ")" }

func (a noCookies) Check(ctx context.Context) error {
	if err := a.res.Captured().Check(ctx); err != nil {
		return err
	}
	lines := a.res.Headers().Values("Set-Cookie")
	if len(lines) == 0 {
		return nil
	}
	cookies, bad := setCookies(a.res)
	names := make([]string, 0, len(cookies))
	for _, c := range cookies {
		names = append(names, fmt.Sprintf("%q", c.Name))
	}
	msg := fmt.Sprintf("%s sets %d cookies", a.res.Describe(), len(lines))
	if len(names) > 0 {
		msg += ": " + strings.Join(names, ", ")
	}
	if bad > 0 {
		msg += fmt.Sprintf(" (%d do not parse)", bad)
	}
	return fmt.Errorf("%w: %s", ct.ErrMismatch, msg)
}

// Policy is what [SecurityHeaders] expects of a response: the security
// headers that mw.SecurityHeaders sends for Headers. As in mw.Headers, a
// zero field means mw's default, so the zero Policy (and [StrictPolicy])
// expects webui's strict defaults, and [PolicyFor] the headers of an
// application's own configuration.
type Policy struct {
	// Headers is the configuration of mw.SecurityHeaders the response
	// should reflect.
	Headers mw.Headers
	// AllowUnsafe permits 'unsafe-inline', 'unsafe-eval' and
	// 'unsafe-hashes' in the expected policy. Without it, any of them in
	// the response fails, also when the expected policy has it.
	AllowUnsafe bool
}

// StrictPolicy returns the policy of webui's default security headers,
// mw.SecurityHeaders(mw.Headers{}), with every value spelled out.
func StrictPolicy() Policy {
	return Policy{Headers: mw.Headers{
		CSP:                       mw.StrictCSP(),
		ReferrerPolicy:            "same-origin",
		PermissionsPolicy:         mw.DefaultPermissionsPolicy,
		CacheControl:              "no-store",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	}}
}

// PolicyFor returns the policy of the headers that
// mw.SecurityHeaders(h) sends.
func PolicyFor(h mw.Headers) Policy {
	return Policy{Headers: h}
}

// unsafeSources are the CSP keywords that re-enable inline code or eval.
var unsafeSources = []string{"'unsafe-inline'", "'unsafe-eval'", "'unsafe-hashes'"}

// SecurityHeaders asserts that a response carries the security headers of
// p, each exactly once: the Content-Security-Policy, parsed with
// mw.ParseCSP and compared directive by directive with the exact source
// lists (in any order); X-Frame-Options as mw derives it from
// frame-ancestors (DENY without frame-ancestors or for 'none', SAMEORIGIN
// for 'self', compared case-insensitively, none when it allows more);
// X-Content-Type-Options: nosniff; Referrer-Policy, Permissions-Policy,
// Cache-Control, Cross-Origin-Opener-Policy and
// Cross-Origin-Resource-Policy; and Strict-Transport-Security when p sets
// HSTS, absent otherwise. Unless p.AllowUnsafe, no directive may allow
// 'unsafe-inline', 'unsafe-eval' or 'unsafe-hashes'. The failure lists
// every difference.
func SecurityHeaders(res *cthttp.Result, p Policy) assertion.Assertion {
	return securityHeaders{res: res, p: p}
}

type securityHeaders struct {
	res *cthttp.Result
	p   Policy
}

func (a securityHeaders) String() string { return "securityHeaders(" + a.res.Describe() + ")" }

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func (a securityHeaders) Check(ctx context.Context) error {
	if err := a.res.Captured().Check(ctx); err != nil {
		return err
	}
	h := a.res.Headers()
	cfg := a.p.Headers
	var problems []string
	single := func(name, want string) {
		values := h.Values(name)
		switch {
		case want == "" && len(values) == 0:
		case want == "":
			problems = append(problems, fmt.Sprintf("has %s %q, want none", name, strings.Join(values, ", ")))
		case len(values) == 0:
			problems = append(problems, fmt.Sprintf("has no %s, want %q", name, want))
		case len(values) > 1:
			problems = append(problems, fmt.Sprintf("has %d %s headers", len(values), name))
		case !strings.EqualFold(values[0], want):
			problems = append(problems, fmt.Sprintf("has %s %q, want %q", name, values[0], want))
		}
	}
	want := cfg.CSP
	if want.IsZero() {
		want = mw.StrictCSP()
	}
	problems = append(problems, a.cspProblems(h.Values("Content-Security-Policy"), want)...)
	single("X-Frame-Options", frameOptions(want))
	single("X-Content-Type-Options", "nosniff")
	single("Referrer-Policy", or(cfg.ReferrerPolicy, "same-origin"))
	single("Permissions-Policy", or(cfg.PermissionsPolicy, mw.DefaultPermissionsPolicy))
	single("Cache-Control", or(cfg.CacheControl, "no-store"))
	single("Cross-Origin-Opener-Policy", or(cfg.CrossOriginOpenerPolicy, "same-origin"))
	single("Cross-Origin-Resource-Policy", or(cfg.CrossOriginResourcePolicy, "same-origin"))
	single("Strict-Transport-Security", cfg.HSTS.String())
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%w: security headers of %s:\n  %s", ct.ErrMismatch, a.res.Describe(), strings.Join(problems, "\n  "))
}

// frameOptions returns the X-Frame-Options that mw derives from the
// frame-ancestors of p, for browsers that do not support frame-ancestors:
// DENY without frame-ancestors (which does not fall back to default-src,
// so framing is otherwise unrestricted) or with just 'none', SAMEORIGIN
// with just 'self', and none otherwise. Keywords are case-insensitive.
func frameOptions(p mw.CSP) string {
	fa, ok := p.Values(mw.FrameAncestors)
	switch {
	case !ok:
		return "DENY"
	case len(fa) == 1 && strings.EqualFold(fa[0], mw.SourceNone):
		return "DENY"
	case len(fa) == 1 && strings.EqualFold(fa[0], mw.SourceSelf):
		return "SAMEORIGIN"
	}
	return ""
}

func (a securityHeaders) cspProblems(values []string, want mw.CSP) []string {
	switch {
	case len(values) == 0:
		return []string{"has no Content-Security-Policy"}
	case len(values) > 1:
		return []string{fmt.Sprintf("has %d Content-Security-Policy headers, which browsers all enforce", len(values))}
	}
	got, err := mw.ParseCSP(values[0])
	if err != nil {
		return []string{fmt.Sprintf("Content-Security-Policy does not parse: %v", err)}
	}
	var out []string
	if !a.p.AllowUnsafe {
		for d, sources := range got.All() {
			for _, s := range sources {
				if contains(unsafeSources, strings.ToLower(s)) {
					out = append(out, fmt.Sprintf("CSP %s allows %s", d, s))
				}
			}
		}
	}
	for d, ws := range want.All() {
		gs, ok := got.Values(d)
		if !ok {
			out = append(out, fmt.Sprintf("CSP lacks %s, want %s", d, sourceList(ws)))
			continue
		}
		if !sameSources(gs, ws) {
			out = append(out, fmt.Sprintf("CSP %s is %s, want %s", d, sourceList(gs), sourceList(ws)))
		}
	}
	for d := range got.All() {
		if _, ok := want.Values(d); !ok {
			out = append(out, fmt.Sprintf("CSP has unexpected %s", d))
		}
	}
	return out
}

func sourceList(s []string) string {
	if len(s) == 0 {
		return "(empty)"
	}
	return strings.Join(s, " ")
}

// sameSources compares source lists as sets: their order does not matter
// to browsers. Keywords, schemes and hosts compare case-insensitively, the
// base64 values of nonces and hashes exactly.
func sameSources(a, b []string) bool {
	norm := func(s []string) []string {
		out := make([]string, len(s))
		for i, v := range s {
			out[i] = normSource(v)
		}
		sort.Strings(out)
		return out
	}
	x, y := norm(a), norm(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// normSource lower-cases a CSP source expression except the base64 value
// of a nonce or hash source.
func normSource(v string) string {
	lower := strings.ToLower(v)
	for _, prefix := range []string{"'nonce-", "'sha256-", "'sha384-", "'sha512-"} {
		if strings.HasPrefix(lower, prefix) {
			return prefix + v[len(prefix):]
		}
	}
	return lower
}
