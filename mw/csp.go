package mw

import (
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Directive is the name of a Content-Security-Policy directive, such as
// "script-src". Names are case-insensitive; the builder stores them in lower
// case.
type Directive string

// Directives of CSP Level 3. The builder accepts other syntactically valid
// names too.
const (
	DefaultSrc     Directive = "default-src"
	ScriptSrc      Directive = "script-src"
	ScriptSrcElem  Directive = "script-src-elem"
	ScriptSrcAttr  Directive = "script-src-attr"
	StyleSrc       Directive = "style-src"
	StyleSrcElem   Directive = "style-src-elem"
	StyleSrcAttr   Directive = "style-src-attr"
	ImgSrc         Directive = "img-src"
	FontSrc        Directive = "font-src"
	ConnectSrc     Directive = "connect-src"
	MediaSrc       Directive = "media-src"
	ObjectSrc      Directive = "object-src"
	FrameSrc       Directive = "frame-src"
	ChildSrc       Directive = "child-src"
	WorkerSrc      Directive = "worker-src"
	ManifestSrc    Directive = "manifest-src"
	FormAction     Directive = "form-action"
	FrameAncestors Directive = "frame-ancestors"
	BaseURI        Directive = "base-uri"
	Sandbox        Directive = "sandbox"
	ReportTo       Directive = "report-to"

	UpgradeInsecureRequests Directive = "upgrade-insecure-requests"
	RequireTrustedTypesFor  Directive = "require-trusted-types-for"
	TrustedTypes            Directive = "trusted-types"
)

// Source expressions that policies commonly use.
const (
	SourceSelf = "'self'" // the origin of the document
	SourceNone = "'none'" // nothing; must be the only source of its list
	SourceData = "data:"  // data: URLs, such as the SVG images that Bootstrap's CSS draws
	SourceBlob = "blob:"  // blob: URLs
)

// ErrInvalidCSP reports a policy, directive or source that is not valid CSP,
// or a URL that no CSP source expression can name.
var ErrInvalidCSP = errors.New("mw: invalid content security policy")

// Bounds of the inputs that the CSP parsers accept.
const (
	maxCSPLen = 64 << 10
	maxURLLen = 4096
)

const cspHeader = "Content-Security-Policy"

// CSP is a Content-Security-Policy: an ordered list of directives, each with
// its values (source expressions for the fetch directives, form-action,
// frame-ancestors and base-uri; tokens for the others).
//
// A CSP is an immutable value: [CSP.With], [CSP.Add] and [CSP.Without] return
// a new policy and leave the receiver unchanged, so a policy may be shared
// between goroutines and derived from freely.
//
// The zero CSP has no directives and serializes to "". Where a policy is
// applied ([Headers], [SetCSP]), the zero CSP stands for [StrictCSP], so a
// forgotten policy fails closed.
//
// The builder validates every value when it is added: it accepts only
// syntactically valid source expressions in source lists (keywords, nonces,
// hashes, schemes and host sources) and visible ASCII without ';' and ',' in
// other directives, so no value can inject a directive or a second policy. A
// keyword without its quotes, such as self, is refused too: it would name a
// host.
// Invalid values passed to With or Add are programmer errors and panic; values
// that come from outside the program, such as a redirect target, must first go
// through [HostSource], which reports an error instead.
type CSP struct {
	directives []cspDirective
}

// cspDirective is a directive of a policy. Its values are never modified once
// the directive is part of a policy; policies share them.
type cspDirective struct {
	name   Directive
	values []string
}

var strictCSP = CSP{directives: []cspDirective{
	{DefaultSrc, []string{SourceNone}},
	{ScriptSrc, []string{SourceSelf}},
	{StyleSrc, []string{SourceSelf}},
	{ImgSrc, []string{SourceSelf, SourceData}},
	{FontSrc, []string{SourceSelf}},
	{ConnectSrc, []string{SourceSelf}},
	{FormAction, []string{SourceSelf}},
	{FrameAncestors, []string{SourceNone}},
	{BaseURI, []string{SourceNone}},
}}

// StrictCSP returns webui's default policy:
//
//	default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:;
//	font-src 'self'; connect-src 'self'; form-action 'self';
//	frame-ancestors 'none'; base-uri 'none'
//
// Everything is denied unless named: scripts, styles and fonts load from the
// page's own origin only, without inline code, inline styles or eval; images
// also from data: URLs, which Bootstrap's CSS uses for its form controls and
// close buttons; fetches, htmx requests and event streams go to the own origin
// only; forms post only to the own origin; the page cannot be framed, and a
// <base> element cannot redirect relative URLs. Objects, frames, media,
// manifests and workers of other origins fall back to default-src 'none'.
func StrictCSP() CSP { return strictCSP }

// ParseCSP parses a serialized policy, such as the value of a
// Content-Security-Policy header, with the validation of the builder.
// Directive names are lower-cased; empty directives (as after a trailing ';')
// are skipped. A repeated directive, an invalid name or value, or more than
// 64 KiB of input is an error wrapping [ErrInvalidCSP]; errors never quote the
// input. The empty string parses to the zero CSP.
func ParseCSP(s string) (CSP, error) {
	if len(s) > maxCSPLen {
		return CSP{}, fmt.Errorf("%w: the policy is longer than %d bytes", ErrInvalidCSP, maxCSPLen)
	}
	var p CSP
	for i, part := range strings.Split(s, ";") {
		fields := strings.FieldsFunc(part, isASCIIWhitespace)
		if len(fields) == 0 {
			continue
		}
		name := Directive(strings.ToLower(fields[0]))
		if !validDirectiveName(string(name)) {
			return CSP{}, fmt.Errorf("%w: directive %d has an invalid name", ErrInvalidCSP, i+1)
		}
		if _, dup := p.Values(name); dup {
			return CSP{}, fmt.Errorf("%w: directive %d repeats an earlier directive", ErrInvalidCSP, i+1)
		}
		values, reason := checkValues(name, fields[1:])
		if reason != "" {
			return CSP{}, fmt.Errorf("%w: directive %d: %s", ErrInvalidCSP, i+1, reason)
		}
		p.directives = append(p.directives, cspDirective{name: name, values: values})
	}
	return p, nil
}

// With returns the policy with directive d set to exactly values: in the
// position of d if the policy has it, else appended. A source-list directive
// without values matches nothing and is written as 'none'. Duplicate values
// are dropped.
//
// With panics if d is not a valid directive name or a value is not valid for
// d (see [CSP]); pass runtime values through [HostSource] first.
func (p CSP) With(d Directive, values ...string) CSP {
	name := mustDirectiveName(d)
	checked, reason := checkValues(name, values)
	if reason != "" {
		panic(fmt.Errorf("%w: %s: %s", ErrInvalidCSP, name, reason))
	}
	return p.set(name, checked)
}

// Add returns the policy with values appended to directive d, which is created
// (at the end) if the policy lacks it. Adding sources to a list that is just
// 'none' replaces the 'none'. Duplicate values are dropped.
//
// Add panics like [CSP.With] on invalid input.
func (p CSP) Add(d Directive, values ...string) CSP {
	name := mustDirectiveName(d)
	old, _ := p.Values(name)
	merged := make([]string, 0, len(old)+len(values))
	if len(values) == 0 || len(old) != 1 || !strings.EqualFold(old[0], SourceNone) {
		merged = append(merged, old...)
	}
	merged = append(merged, values...)
	checked, reason := checkValues(name, merged)
	if reason != "" {
		panic(fmt.Errorf("%w: %s: %s", ErrInvalidCSP, name, reason))
	}
	return p.set(name, checked)
}

// Without returns the policy without directive d. A fetch directive that is
// removed falls back to default-src in browsers.
func (p CSP) Without(d Directive) CSP {
	name := Directive(strings.ToLower(string(d)))
	i := p.index(name)
	if i < 0 {
		return p
	}
	return CSP{directives: slices.Delete(slices.Clone(p.directives), i, i+1)}
}

// Values returns a copy of the values of directive d, and whether the policy
// has d. A directive without values, such as upgrade-insecure-requests,
// returns an empty slice and true.
func (p CSP) Values(d Directive) ([]string, bool) {
	i := p.index(Directive(strings.ToLower(string(d))))
	if i < 0 {
		return nil, false
	}
	return slices.Clone(p.directives[i].values), true
}

// All iterates over the directives of the policy in order, with a copy of
// their values. The iterator is restartable.
func (p CSP) All() iter.Seq2[Directive, []string] {
	return func(yield func(Directive, []string) bool) {
		for _, d := range p.directives {
			if !yield(d.name, slices.Clone(d.values)) {
				return
			}
		}
	}
}

// IsZero reports whether the policy has no directives.
func (p CSP) IsZero() bool { return len(p.directives) == 0 }

// String serializes the policy as a header value: directives separated by
// "; ", each name followed by its values separated by spaces.
func (p CSP) String() string {
	var b strings.Builder
	for i, d := range p.directives {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(string(d.name))
		for _, v := range d.values {
			b.WriteByte(' ')
			b.WriteString(v)
		}
	}
	return b.String()
}

func (p CSP) index(name Directive) int {
	return slices.IndexFunc(p.directives, func(d cspDirective) bool { return d.name == name })
}

// set returns the policy with directive name set to the checked values,
// without modifying p.
func (p CSP) set(name Directive, values []string) CSP {
	out := make([]cspDirective, len(p.directives), len(p.directives)+1)
	copy(out, p.directives)
	if i := p.index(name); i >= 0 {
		out[i].values = values
		return CSP{directives: out}
	}
	return CSP{directives: append(out, cspDirective{name: name, values: values})}
}

// SetCSP sets the Content-Security-Policy of one response, replacing the one
// that [SecurityHeaders] set, for a page that needs a different policy, such
// as a form whose submission redirects to another origin (browsers apply
// form-action to the redirects after a submission too). The zero CSP sets
// [StrictCSP].
//
// SetCSP also sets X-Frame-Options to agree with the policy's frame-ancestors
// (see [Headers]). It must be called before the response header is written.
func SetCSP(w http.ResponseWriter, p CSP) {
	if p.IsZero() {
		p = StrictCSP()
	}
	h := w.Header()
	h.Set(cspHeader, p.String())
	if fo := frameOptions(p); fo != "" {
		h.Set("X-Frame-Options", fo)
	} else {
		h.Del("X-Frame-Options")
	}
}

// frameOptions returns the X-Frame-Options value that agrees with the
// frame-ancestors directive of p, for browsers that know only the older
// header: DENY without frame-ancestors or with 'none', SAMEORIGIN for exactly
// 'self', and "" (no header) for anything else, which the older header cannot
// express.
func frameOptions(p CSP) string {
	fa, ok := p.Values(FrameAncestors)
	switch {
	case !ok:
		return "DENY"
	case len(fa) == 1 && strings.EqualFold(fa[0], SourceNone):
		return "DENY"
	case len(fa) == 1 && strings.EqualFold(fa[0], SourceSelf):
		return "SAMEORIGIN"
	default:
		return ""
	}
}

// HostSource returns the origin of an absolute http, https, ws or wss URL as a
// CSP host source, such as "https://app.example.com:8443", for adding a runtime
// origin to a policy: form-action for a form whose submission redirects there,
// or connect-src. The scheme and host are lower-cased, a default port is
// dropped, and the path, query and fragment are ignored.
//
// It fails with an error wrapping [ErrInvalidCSP] for what the host-source
// grammar of CSP cannot name or should not: other schemes, user information,
// hosts that are not DNS names of ASCII letters, digits and dashes (CSP has no
// syntax for IPv6 literals, and its host matching applies to domain names, so
// browsers do not reliably match IPv4 literals either), names whose last label
// is numeric, invalid ports, and inputs over 4 KiB. A caller that cannot name
// a target can send the browser there with a navigation (a link, or a page
// that continues by itself), which form-action does not govern. Errors never
// quote the input.
func HostSource(rawURL string) (string, error) {
	if len(rawURL) > maxURLLen {
		return "", fmt.Errorf("%w: the URL is longer than %d bytes", ErrInvalidCSP, maxURLLen)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: not a valid URL", ErrInvalidCSP)
	}
	switch u.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return "", fmt.Errorf("%w: the URL is not an absolute http(s) or ws(s) URL", ErrInvalidCSP)
	}
	if u.Opaque != "" || u.Host == "" {
		return "", fmt.Errorf("%w: the URL has no host", ErrInvalidCSP)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: the URL has user information", ErrInvalidCSP)
	}
	host := strings.ToLower(u.Hostname())
	if _, err := netip.ParseAddr(host); err == nil || strings.Contains(host, ":") {
		return "", fmt.Errorf("%w: a CSP source cannot name an IP address", ErrInvalidCSP)
	}
	if !validDNSName(host) {
		return "", fmt.Errorf("%w: the host is not a DNS name of letters, digits and dashes", ErrInvalidCSP)
	}
	out := u.Scheme + "://" + host
	if port := u.Port(); port != "" {
		n, ok := parsePort(port)
		if !ok {
			return "", fmt.Errorf("%w: invalid port", ErrInvalidCSP)
		}
		if n != defaultPort(u.Scheme) {
			out += ":" + strconv.Itoa(n)
		}
	}
	return out, nil
}

// defaultPort returns the default port of a URL scheme, or 0.
func defaultPort(scheme string) int {
	switch scheme {
	case "http", "ws":
		return 80
	case "https", "wss":
		return 443
	}
	return 0
}

// parsePort parses a decimal port in [1, 65535].
func parsePort(s string) (int, bool) {
	if s == "" || len(s) > 5 || !allBytes(s, isDigit) {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, false
	}
	return n, true
}

func mustDirectiveName(d Directive) Directive {
	name := Directive(strings.ToLower(string(d)))
	if !validDirectiveName(string(name)) {
		panic(fmt.Errorf("%w: invalid directive name", ErrInvalidCSP))
	}
	return name
}

// isSourceList reports whether a directive's value is a source list.
func isSourceList(name Directive) bool {
	return strings.HasSuffix(string(name), "-src") || name == FormAction || name == FrameAncestors || name == BaseURI
}

// checkValues validates the values of a directive and returns them as a new
// slice without duplicates, or the reason why they are invalid (which never
// quotes them). An empty source list becomes 'none'.
func checkValues(name Directive, values []string) ([]string, string) {
	out := make([]string, 0, len(values))
	for i, v := range values {
		if !validValueToken(v) {
			return nil, fmt.Sprintf("value %d is not a CSP token", i+1)
		}
		if isSourceList(name) && !validSource(name, v) {
			return nil, fmt.Sprintf("value %d is not a valid source expression", i+1)
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if !isSourceList(name) {
		return out, ""
	}
	if len(out) == 0 {
		return []string{SourceNone}, ""
	}
	if len(out) > 1 && slices.ContainsFunc(out, func(v string) bool { return strings.EqualFold(v, SourceNone) }) {
		return nil, "'none' must be the only source"
	}
	return out, ""
}

// validDirectiveName reports whether s is 1*( ALPHA / DIGIT / "-" ).
func validDirectiveName(s string) bool {
	return s != "" && allBytes(s, func(c byte) bool { return isAlpha(c) || isDigit(c) || c == '-' })
}

// validValueToken reports whether s is a non-empty directive value token:
// visible ASCII except ';' and ','.
func validValueToken(s string) bool {
	return s != "" && allBytes(s, func(c byte) bool { return c > 0x20 && c < 0x7f && c != ';' && c != ',' })
}

// cspKeywords are the keyword sources of CSP Level 3, in lower case.
var cspKeywords = map[string]bool{
	"'self'": true, "'none'": true, "'unsafe-inline'": true, "'unsafe-eval'": true,
	"'strict-dynamic'": true, "'unsafe-hashes'": true, "'report-sample'": true,
	"'unsafe-allow-redirects'": true, "'wasm-unsafe-eval'": true, "'trusted-types-eval'": true,
	"'inline-speculation-rules'": true, "'report-sha256'": true, "'report-sha384'": true,
	"'report-sha512'": true,
}

// validSource reports whether s is a source expression valid in the source
// list of directive name. frame-ancestors takes only 'self', 'none', schemes
// and hosts.
func validSource(name Directive, s string) bool {
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "'") {
		if name == FrameAncestors {
			return lower == SourceSelf || lower == SourceNone
		}
		if cspKeywords[lower] {
			return true
		}
		for _, prefix := range []string{"'nonce-", "'sha256-", "'sha384-", "'sha512-"} {
			if strings.HasPrefix(lower, prefix) && len(s) > len(prefix)+1 && strings.HasSuffix(s, "'") {
				return validBase64Value(s[len(prefix) : len(s)-1])
			}
		}
		return false
	}
	if strings.HasSuffix(s, ":") && !strings.Contains(s, "/") {
		return validScheme(s[:len(s)-1])
	}
	if cspKeywords["'"+lower+"'"] {
		// A keyword without its quotes would name a host; it is a mistake.
		return false
	}
	return validHostSource(s)
}

// validHostSource reports whether s matches
// [ scheme "://" ] host-part [ ":" port-part ] [ path-part ].
func validHostSource(s string) bool {
	rest := s
	if i := strings.Index(rest, "://"); i >= 0 {
		if !validScheme(rest[:i]) {
			return false
		}
		rest = rest[i+len("://"):]
	}
	host, tail := rest, ""
	if i := strings.IndexAny(rest, ":/"); i >= 0 {
		host, tail = rest[:i], rest[i:]
	}
	if !validHostPart(host) {
		return false
	}
	if strings.HasPrefix(tail, ":") {
		port := tail[1:]
		tail = ""
		if i := strings.IndexByte(port, '/'); i >= 0 {
			port, tail = port[:i], port[i:]
		}
		if _, ok := parsePort(port); !ok && port != "*" {
			return false
		}
	}
	if tail == "" {
		return true
	}
	return tail[0] == '/' && !strings.HasPrefix(tail, "//") && allBytes(tail, isPathByte)
}

// validHostPart reports whether s is "*" or [ "*." ] labels of letters, digits
// and dashes, separated by dots.
func validHostPart(s string) bool {
	if s == "*" {
		return true
	}
	s = strings.TrimPrefix(s, "*.")
	if s == "" {
		return false
	}
	for label := range strings.SplitSeq(s, ".") {
		if label == "" || !allBytes(label, isHostByte) {
			return false
		}
	}
	return true
}

// validScheme reports whether s is ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func validScheme(s string) bool {
	return s != "" && isAlpha(s[0]) &&
		allBytes(s, func(c byte) bool { return isAlpha(c) || isDigit(c) || c == '+' || c == '-' || c == '.' })
}

// validBase64Value reports whether s is 1*( ALPHA / DIGIT / "+" / "/" / "-" /
// "_" ) followed by at most two "=".
func validBase64Value(s string) bool {
	body := strings.TrimRight(s, "=")
	if body == "" || len(s)-len(body) > 2 {
		return false
	}
	return allBytes(body, func(c byte) bool {
		return isAlpha(c) || isDigit(c) || c == '+' || c == '/' || c == '-' || c == '_'
	})
}

// validDNSName reports whether s is a lower-case DNS name of at most 253
// bytes: dot-separated labels of 1 to 63 letters, digits and dashes, without a
// trailing dot, whose last label is not a number (decimal, or hexadecimal
// with 0x), which browsers would read as part of an IPv4 address.
func validDNSName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	last := ""
	for label := range strings.SplitSeq(s, ".") {
		if label == "" || len(label) > 63 || !allBytes(label, func(c byte) bool {
			return 'a' <= c && c <= 'z' || isDigit(c) || c == '-'
		}) {
			return false
		}
		last = label
	}
	if hex, ok := strings.CutPrefix(last, "0x"); ok && allBytes(hex, isHexDigit) {
		return false
	}
	return !allBytes(last, isDigit)
}

func isHostByte(c byte) bool { return isAlpha(c) || isDigit(c) || c == '-' }

// isPathByte reports whether c may appear in the path of a host source: the
// URL path characters except ';' and ',', which end directives and policies.
func isPathByte(c byte) bool {
	return isAlpha(c) || isDigit(c) || strings.IndexByte("-._~!$&'()*+=:@/%", c) >= 0
}

func isAlpha(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func isHexDigit(c byte) bool { return isDigit(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' }

func isASCIIWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\f' || r == '\r'
}

func allBytes(s string, ok func(byte) bool) bool {
	for i := 0; i < len(s); i++ {
		if !ok(s[i]) {
			return false
		}
	}
	return true
}
