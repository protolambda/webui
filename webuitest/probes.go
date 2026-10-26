package webuitest

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/mustbe/assertion"

	"github.com/protolambda/webui/mw"
)

// Probe is a variant of a request that a cross-site attacker, or a
// browser on an attacker's page, could send.
type Probe struct {
	// Name describes the probe in messages.
	Name string
	// Options turn a same-origin request into the probe.
	Options []ReqOption
}

// Origins that probes use: another site, and another origin of the same
// site (a sibling subdomain of [BaseURL]'s registrable domain).
const (
	ForeignOrigin  = "https://evil.example"
	SameSiteOrigin = "https://other.example.test"
)

// CrossSiteProbes returns the probes of [CrossSiteProbesFor] for
// [BaseURL]: requests that a same-origin policy (such as mw.SameOrigin
// behind mw.HostCheck) must refuse when they change state. Each call
// returns new probes.
func CrossSiteProbes() []Probe {
	return crossSiteProbes(defaultBase)
}

// CrossSiteProbesFor returns requests that a same-origin policy (such as
// mw.SameOrigin behind mw.HostCheck) of site, a site URL as mw.ParseSite
// takes it, such as the [Client.Base] of the application under test, must
// refuse when they change state:
//
//   - a cross-site request with fetch metadata (Sec-Fetch-Site:
//     cross-site, a foreign Origin), as current browsers send it;
//   - a same-site request from another origin of the site (Sec-Fetch-Site:
//     same-site; a sibling subdomain, or for a host that has none, such as
//     an IP address or localhost, another port), which cookies with
//     SameSite do not stop;
//   - without fetch metadata: a foreign Origin; Origins that only look
//     like the site's, which a prefix, suffix or host-only comparison
//     accepts (the site's origin as the prefix of another host, its host
//     as the suffix of another, the site on another port); the opaque
//     "null" Origin (sandboxed frames, some redirects); the site's own host
//     over the other scheme (a downgraded origin for an https site,
//     named "http origin", and "https origin" for an http one); a foreign
//     Referer only, also one that starts with the site's origin; and, on
//     sites other than loopback ones, no origin headers at all;
//   - DNS rebinding: a foreign Host header (with the site's port) with
//     that host as the origin, which looks same-origin to the browser.
//
// The look-alike probes are built from site, so that they catch a check
// that compares origins by prefix, suffix or host on the site under test.
// A loopback site (mw.IsLoopbackHost) gets no probe without origin
// headers: mw.SameOrigin accepts such requests there by default
// (mw.HeaderlessLoopback, for local tools). An application that refuses
// them there too (mw.HeaderlessDeny) checks it with a request of its own
// with [NoFetchMetadata].
//
// The other origin of the same-site probe is found without a public
// suffix list: a host of three labels or more gets a sibling (its first
// label replaced), and one of two a subdomain. Under a public suffix of
// more labels, such as co.uk, the sibling of a host of three labels
// (example.co.uk) is of another site (other.co.uk), so that probe is a
// cross-site one: probe such a site with a request of your own, with
// [FetchSite]("same-site") and the Origin of a subdomain of the site.
//
// The site's host with a trailing dot ("https://app.example.test.") is
// not probed: it names the same server, so a policy that accepts it lets
// no other site in, and requiring its refusal would fail harmless
// normalizations. A site that mw.ParseSite refuses is a programming
// error: CrossSiteProbesFor panics. Each call returns new probes.
func CrossSiteProbesFor(site string) []Probe {
	u, err := parseBase(site)
	if err != nil {
		panic(fmt.Sprintf("webuitest.CrossSiteProbesFor: %v", err))
	}
	return crossSiteProbes(u)
}

// crossSiteProbes returns the probes of [CrossSiteProbesFor] for the
// origin u.
func crossSiteProbes(u *url.URL) []Probe {
	origin, host, port := urlOrigin(u), hostKey(u), u.Port()
	scheme, other := "https", "http"
	if u.Scheme == "http" {
		scheme, other = "http", "https"
	}
	rebound := hostPort("evil.example", port)
	probes := []Probe{
		{Name: "cross-site", Options: []ReqOption{FetchSite("cross-site"), Origin(ForeignOrigin)}},
		{Name: "same-site", Options: []ReqOption{FetchSite("same-site"), Origin(sameSiteOrigin(u))}},
		{Name: "foreign origin without fetch metadata", Options: []ReqOption{NoFetchMetadata(), Header("Origin", ForeignOrigin)}},
		{Name: "lookalike origin", Options: []ReqOption{NoFetchMetadata(), Header("Origin", origin+".evil.example")}},
		{Name: "suffix origin", Options: []ReqOption{NoFetchMetadata(), Header("Origin", scheme+"://evil"+host)}},
		{Name: "other port origin", Options: []ReqOption{NoFetchMetadata(), Header("Origin", otherPortOrigin(u))}},
		{Name: "null origin", Options: []ReqOption{NoFetchMetadata(), Header("Origin", "null")}},
		{Name: other + " origin", Options: []ReqOption{NoFetchMetadata(), Header("Origin", other+"://"+host)}},
		{Name: "foreign referer only", Options: []ReqOption{NoFetchMetadata(), Header("Referer", ForeignOrigin+"/page")}},
		{Name: "lookalike referer only", Options: []ReqOption{NoFetchMetadata(), Header("Referer", origin+".evil.example/page")}},
		{Name: "no origin headers", Options: []ReqOption{NoFetchMetadata()}},
		{Name: "dns rebinding", Options: []ReqOption{Host(rebound), Origin(scheme + "://" + rebound)}},
	}
	if mw.IsLoopbackHost(u.Hostname()) {
		probes = slices.DeleteFunc(probes, func(p Probe) bool { return p.Name == "no origin headers" })
	}
	return probes
}

// sameSiteOrigin returns another origin of the site of u: a sibling
// subdomain of a DNS name with three or more labels, a subdomain of one
// with two, or, for an IP address or a single label (localhost), the host
// on another port, as browsers do not tell ports apart in sites. Without a
// public suffix list, it takes the last two labels for the registrable
// domain: under a public suffix of more labels (co.uk), the sibling of a
// name of three labels is of another site.
func sameSiteOrigin(u *url.URL) string {
	name := u.Hostname()
	labels := strings.Split(name, ".")
	if _, err := netip.ParseAddr(name); err == nil || len(labels) < 2 {
		return otherPortOrigin(u)
	}
	sibling := "other"
	if labels[0] == sibling {
		sibling = "another"
	}
	if len(labels) >= 3 {
		labels[0] = sibling
	} else {
		labels = append([]string{sibling}, labels...)
	}
	return u.Scheme + "://" + hostPort(strings.Join(labels, "."), u.Port())
}

// otherPortOrigin returns the origin of the host of u on another port:
// 8443, or 8444 if u has that one.
func otherPortOrigin(u *url.URL) string {
	port := "8443"
	if u.Port() == port {
		port = "8444"
	}
	return u.Scheme + "://" + hostPort(u.Hostname(), port)
}

// hostPort joins a host name (an IPv6 address without brackets) and a
// port, which may be empty, as a Host header names them.
func hostPort(name, port string) string {
	if strings.Contains(name, ":") {
		name = "[" + name + "]"
	}
	if port == "" {
		return name
	}
	return name + ":" + port
}

// RejectsCrossSite sends every probe of [CrossSiteProbesFor] the site of s
// (its URL as a [SiteSender], such as a Client with a Base, else
// [BaseURL]) for a state-changing request with s (a [Client] carries its
// CSRF token, so the origin layer is tested alone) and returns an
// assertion that each was refused: status 400 (a rejected Host) or 403,
// no cookies set, and, when unchanged is not nil, unchanged reporting no
// error right after the probe (for example, comparing the application's
// state with a copy taken before). The requests are sent when
// RejectsCrossSite is called; the assertion reports every probe that got
// through.
func RejectsCrossSite(t TB, s Sender, method, target string, body Body, unchanged func() error) assertion.Assertion {
	t.Helper()
	desc := fmt.Sprintf("rejectsCrossSite(%s %s)", method, target)
	site, ok := senderSite(t, s)
	if !ok {
		return constant{desc: desc, err: fmt.Errorf("%w: the sender has no valid site", ct.ErrLoad)}
	}
	var parts []assertion.Assertion
	for _, p := range crossSiteProbes(site) {
		res := s.Send(t, method, target, body, p.Options...)
		if res == nil {
			return constant{desc: "rejectsCrossSite", err: fmt.Errorf("%w: probe %q was not sent", ct.ErrLoad, p.Name)}
		}
		parts = append(parts, probe{name: p.Name, inner: All("", refused{res}, NoCookies(res))})
		if unchanged != nil {
			if err := unchanged(); err != nil {
				parts = append(parts, constant{desc: "unchanged", err: fmt.Errorf("probe %q changed the state: %w", p.Name, err)})
			}
		}
	}
	return All(desc, parts...)
}

type probe struct {
	name  string
	inner assertion.Assertion
}

func (p probe) String() string { return "probe(" + p.name + ")" }

func (p probe) Check(ctx context.Context) error {
	if err := p.inner.Check(ctx); err != nil {
		return fmt.Errorf("probe %q: %w", p.name, err)
	}
	return nil
}

type refused struct{ res *cthttp.Result }

func (r refused) String() string { return "refused(" + r.res.Describe() + ")" }

func (r refused) Check(ctx context.Context) error {
	if err := r.res.Captured().Check(ctx); err != nil {
		return err
	}
	switch code := r.res.StatusCode(); code {
	case http.StatusBadRequest, http.StatusForbidden:
		return nil
	default:
		return fmt.Errorf("%w: %s got status %d, want 400 or 403", ct.ErrMismatch, r.res.Describe(), code)
	}
}

// constant is an assertion with a fixed outcome.
type constant struct {
	desc string
	err  error
}

func (c constant) String() string              { return c.desc }
func (c constant) Check(context.Context) error { return c.err }

// OpenRedirectVectors returns values of a "next" or "return to"
// parameter that a redirect must never follow, because browsers would
// leave the site or run script: scheme-relative and backslash spellings of
// another host, absolute URLs, script and data URLs, values with control
// characters that URL parsers drop or that split headers, and paths
// without a leading slash. A safe redirect helper (web.SafeNext) should
// replace each with its fallback. Each call returns a new slice.
func OpenRedirectVectors() []string {
	return []string{
		"//evil.example",
		"///evil.example",
		"////evil.example/path",
		`/\evil.example`,
		`\\evil.example`,
		`\/evil.example`,
		`/\/evil.example`,
		`/./\evil.example`,
		"https://evil.example",
		"https:evil.example",
		"https:/evil.example",
		"http://evil.example",
		"HTTPS://EVIL.EXAMPLE",
		"https://app.example.test@evil.example",
		"//app.example.test@evil.example",
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		" javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"\t//evil.example",
		"/\t/evil.example",
		"/\n/evil.example",
		"/\r/evil.example",
		" //evil.example",
		"\x00//evil.example",
		"/\x7f/evil.example",
		"/next\r\nSet-Cookie: x=y",
		"evil.example",
		"tasks",
		"",
	}
}

// LocalRedirectTargets returns values of a "next" parameter that name
// pages of the application and should be kept by a safe redirect helper.
// Each call returns a new slice.
func LocalRedirectTargets() []string {
	return []string{"/", "/tasks", "/tasks/7", "/tasks?page=2", "/tasks#notes", "/a/b/c", "/search?q=a%2Fb"}
}

// Hostile returns text inputs that try to become markup, script, URLs,
// template expressions or misleading text when an application echoes
// them: tag and attribute break-outs, script URLs, htmx attributes,
// template syntax, bidirectional overrides and invisible characters,
// control characters, path traversal and a long value. A page that shows
// one must show it as text: check that it appears in the text, not in a
// tag or attribute name, and that the page still satisfies [Rules]. Each
// call returns a new slice.
//
// Showing these inputs as text keeps them from becoming markup. It does
// not keep the direction overrides and invisible characters among them
// from misleading a reader: as text they display as the attacker intends,
// and an unclosed override reorders the page text that follows it in the
// same block. Render untrusted names and titles with
// [github.com/protolambda/webui/visible.Text] or
// [github.com/protolambda/webui/visible.Code], which escape such
// characters, mark the value as escaped and isolate its direction, and
// refuse or flag them where a name decides what a user trusts.
func Hostile() []string {
	return []string{
		"<script>alert(1)</script>",
		`"><img src=x onerror=alert(1)>`,
		`'><svg onload=alert(1)>`,
		`" autofocus onfocus="alert(1)`,
		"</textarea><script>alert(1)</script>",
		"</title><script>alert(1)</script>",
		"javascript:alert(1)",
		`<a href="javascript:alert(1)">link</a>`,
		`<div hx-get="/admin" hx-trigger="load">`,
		`hx-on:click="alert(1)"`,
		"{{7*7}}",
		"${7*7}",
		"&lt;b&gt;escaped&lt;/b&gt;",
		"\u202Egnp.exe",
		"ad\u200Bmin",
		"Ad\u043Cin",
		"line\nbreak\r\nand\rcarriage",
		"tab\tand\x00nul",
		"../../etc/passwd",
		"%3Cscript%3E",
		"\U0001F600 \u00FCnic\u00F6de",
		strings.Repeat("a", 4096),
	}
}
