package mw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// ErrInvalidSite reports a site URL that [ParseSite] refuses.
var ErrInvalidSite = errors.New("mw: invalid site URL")

// maxSiteLen bounds the input of ParseSite.
const maxSiteLen = 2048

// Site is an origin that an application is served at, such as
// https://app.example.com, parsed from configuration by [ParseSite].
//
// A Site is a small comparable value, safe to copy and to use concurrently.
// The zero Site is invalid: it matches no host and is refused by [HostCheck].
type Site struct {
	origin   string // scheme://host[:port], lower-case, without a default port
	host     string // host[:port] as a Host header names it
	hostname string // host without the port; IPv6 literals keep their brackets
	port     string // "" for the scheme's default port
	https    bool
	loopback bool
}

// ParseSite parses the URL of a site. It accepts only an origin: an http or
// https URL with a host and nothing else (no user information, path, query or
// fragment; a single "/" path is allowed and dropped). Plain http is accepted
// only for loopback hosts ([IsLoopbackHost]), where browsers treat it as a
// secure context and no network carries it.
//
// The host must be a DNS name of ASCII letters, digits and dashes (punycode
// for international names), an IPv4 address, or a bracketed IPv6 address
// without zone. The result is normalized as browsers serialize origins: the
// scheme and host lower-cased, IPv6 in its canonical form, a default port
// dropped, other ports in plain decimal.
//
// Errors wrap [ErrInvalidSite] and never quote the input.
func ParseSite(raw string) (Site, error) {
	if raw == "" || len(raw) > maxSiteLen {
		return Site{}, fmt.Errorf("%w: the URL must have 1 to %d bytes", ErrInvalidSite, maxSiteLen)
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return Site{}, fmt.Errorf("%w: not a valid URL", ErrInvalidSite)
	case u.Scheme != "https" && u.Scheme != "http", u.Opaque != "", u.Host == "":
		return Site{}, fmt.Errorf("%w: the URL must be an absolute http(s) URL with a host", ErrInvalidSite)
	case u.User != nil:
		return Site{}, fmt.Errorf("%w: the URL must not have user information", ErrInvalidSite)
	case u.Path != "" && u.Path != "/", u.RawQuery != "", u.ForceQuery, u.Fragment != "", strings.Contains(raw, "#"):
		return Site{}, fmt.Errorf("%w: the URL must be an origin, without path, query or fragment", ErrInvalidSite)
	}
	hostname, err := siteHostname(u)
	if err != nil {
		return Site{}, err
	}
	https := u.Scheme == "https"
	port := ""
	if p := u.Port(); p != "" {
		n, ok := parsePort(p)
		if !ok {
			return Site{}, fmt.Errorf("%w: invalid port", ErrInvalidSite)
		}
		if n != defaultPort(u.Scheme) {
			port = strconv.Itoa(n)
		}
	}
	loopback := IsLoopbackHost(hostname)
	if !https && !loopback {
		return Site{}, fmt.Errorf("%w: the URL must be https unless its host is a loopback host", ErrInvalidSite)
	}
	host := hostname
	if port != "" {
		host += ":" + port
	}
	return Site{
		origin:   u.Scheme + "://" + host,
		host:     host,
		hostname: hostname,
		port:     port,
		https:    https,
		loopback: loopback,
	}, nil
}

// siteHostname returns the normalized host of a site URL, with brackets for
// IPv6.
func siteHostname(u *url.URL) (string, error) {
	// ASCII lower-casing keeps other characters for validDNSName to refuse:
	// Unicode lower-casing would turn U+0130 into "i" and U+212A into "k".
	name := asciiLower(u.Hostname())
	if strings.HasPrefix(u.Host, "[") {
		a, err := netip.ParseAddr(name)
		switch {
		case err != nil || !a.Is6():
			return "", fmt.Errorf("%w: invalid IPv6 address", ErrInvalidSite)
		case a.Zone() != "":
			return "", fmt.Errorf("%w: an IPv6 address with a zone is not an origin", ErrInvalidSite)
		case a.Is4In6():
			return "", fmt.Errorf("%w: use the IPv4 address instead of an IPv4-mapped IPv6 address", ErrInvalidSite)
		}
		return "[" + a.String() + "]", nil
	}
	if a, err := netip.ParseAddr(name); err == nil && a.Is4() {
		return a.String(), nil
	}
	if !validDNSName(name) {
		return "", fmt.Errorf("%w: the host must be a DNS name of letters, digits and dashes, or an IP address", ErrInvalidSite)
	}
	return name, nil
}

// MustParseSite is like [ParseSite] but panics on error. It is meant for
// constant URLs in programs and tests.
func MustParseSite(raw string) Site {
	s, err := ParseSite(raw)
	if err != nil {
		panic(err)
	}
	return s
}

// Origin returns the origin of the site, such as "https://app.example.com" or
// "http://127.0.0.1:8080", as browsers send it in the Origin header.
func (s Site) Origin() string { return s.origin }

// Host returns the host of the site as a Host header names it, with the port
// unless it is the scheme's default, such as "app.example.com" or
// "[::1]:8080".
func (s Site) Host() string { return s.host }

// HTTPS reports whether the site is served over https.
func (s Site) HTTPS() bool { return s.https }

// Loopback reports whether the host of the site is a loopback host, which
// only the local machine reaches. The policies of this package relax a few
// rules for loopback sites (plain http, requests without origin headers);
// see also server.CheckLoopback, which refuses to serve a loopback site on a
// listener that the network reaches.
func (s Site) Loopback() bool { return s.loopback }

// IsZero reports whether s is the zero Site.
func (s Site) IsZero() bool { return s.origin == "" }

// String returns the origin of the site.
func (s Site) String() string { return s.origin }

// MarshalText returns the origin of the site, for configuration files.
func (s Site) MarshalText() ([]byte, error) { return []byte(s.origin), nil }

// UnmarshalText parses a site URL with [ParseSite], for configuration files
// and flag.TextVar.
func (s *Site) UnmarshalText(text []byte) error {
	parsed, err := ParseSite(string(text))
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// MatchHost reports whether a Host header names the site. It tolerates what
// clients legitimately vary: the case of ASCII letters, a trailing dot on the
// name, and an explicit default port. It applies no Unicode case folding, so
// a non-ASCII character never matches. It never matches for the zero Site.
func (s Site) MatchHost(host string) bool {
	if s.IsZero() || host == "" || len(host) > 300 {
		return false
	}
	name, port, ok := splitHost(host)
	if !ok {
		return false
	}
	name = strings.TrimSuffix(name, ".")
	if (s.https && port == "443") || (!s.https && port == "80") {
		port = ""
	}
	return port == s.port && asciiEqualFold(name, s.hostname)
}

// splitHost splits a Host header into its name (IPv6 with brackets) and port,
// which is "" if absent. It fails for an unbracketed IPv6 address and
// malformed brackets.
func splitHost(hostport string) (name, port string, ok bool) {
	if strings.HasPrefix(hostport, "[") {
		end := strings.IndexByte(hostport, ']')
		if end < 0 {
			return "", "", false
		}
		name, rest := hostport[:end+1], hostport[end+1:]
		if rest == "" {
			return name, "", true
		}
		if rest[0] != ':' {
			return "", "", false
		}
		return name, rest[1:], true
	}
	i := strings.LastIndexByte(hostport, ':')
	if i < 0 {
		return hostport, "", true
	}
	if strings.IndexByte(hostport[:i], ':') >= 0 {
		return "", "", false
	}
	return hostport[:i], hostport[i+1:], true
}

// IsLoopbackHost reports whether a host name (without port) names the local
// machine: "localhost" and names under ".localhost" (which browsers resolve
// to loopback themselves), addresses in 127.0.0.0/8, and ::1 (also in
// brackets, or as an IPv4-mapped loopback address). The case of ASCII letters
// and a trailing dot are ignored; names with non-ASCII characters are not
// loopback hosts. Addresses with a zone are not loopback hosts.
func IsLoopbackHost(host string) bool {
	h := strings.TrimSuffix(asciiLower(host), ".")
	bracketed := false
	if inner, ok := strings.CutPrefix(h, "["); ok {
		h, bracketed = strings.CutSuffix(inner, "]")
		if !bracketed {
			return false
		}
	} else if h == "localhost" || (strings.HasSuffix(h, ".localhost") && validDNSName(h)) {
		return true
	}
	a, err := netip.ParseAddr(h)
	if err != nil || a.Zone() != "" || (bracketed && !a.Is6()) {
		return false
	}
	return a.Unmap().IsLoopback()
}

type siteKey struct{}

// WithSite returns a context that carries site, as [HostCheck] attaches it.
// It is meant for tests and for handlers mounted without HostCheck.
func WithSite(ctx context.Context, site Site) context.Context {
	return context.WithValue(ctx, siteKey{}, site)
}

// SiteFrom returns the site of a request that passed [HostCheck] (or that
// [WithSite] attached), and whether there is one.
func SiteFrom(ctx context.Context) (Site, bool) {
	s, ok := ctx.Value(siteKey{}).(Site)
	return s, ok && !s.IsZero()
}

// HostCheck refuses requests whose Host header names none of the sites
// ([Site.MatchHost]) with 400 Bad Request, and attaches the first site that
// matches to the request context ([SiteFrom]).
//
// It defends against DNS rebinding: a page of another site whose name the
// attacker points at this server reaches it with that other name in the Host
// header. Without the check, the browser would treat the attacker's page as
// same-origin with the responses, and every origin comparison would pass.
//
// HostCheck panics if there are no sites or one is the zero Site.
func HostCheck(sites ...Site) Middleware {
	if len(sites) == 0 {
		panic("mw: HostCheck needs at least one site")
	}
	for _, s := range sites {
		if s.IsZero() {
			panic("mw: HostCheck got the zero Site")
		}
	}
	sites = append([]Site(nil), sites...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, s := range sites {
				if s.MatchHost(r.Host) {
					next.ServeHTTP(w, r.WithContext(WithSite(r.Context(), s)))
					return
				}
			}
			http.Error(w, "invalid host", http.StatusBadRequest)
		})
	}
}
