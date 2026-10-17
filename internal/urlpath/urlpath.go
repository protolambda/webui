// Package urlpath holds the URL checks that web, page and sse share, so
// that the rule for "a path on the page's own origin" exists once.
//
// The checks are strict on purpose. Browsers drop tabs and line breaks
// inside URLs, strip leading and trailing spaces, and read a backslash as
// a slash, so "/\host" or "/<TAB>/host" can turn into "//host", which names
// another host. Non-ASCII characters must arrive percent-encoded, as
// browsers send them and as url.PathEscape writes them. Each caller adds
// its own rules, such as a length limit, on top.
package urlpath

import (
	"net/url"
	"strings"
)

// Printable reports whether s is non-empty and consists of printable ASCII
// other than space and backslash.
func Printable(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c <= ' ' || c >= 0x7f || c == '\\' {
			return false
		}
	}
	return true
}

// Local reports whether s is a path on the page's own origin, with an
// optional query and fragment: it is [Printable], starts with a single
// "/", parses as a URL reference with complete percent escapes in the
// path, query and fragment, and has no scheme, host or user information.
// A ';' in the query is allowed.
func Local(s string) bool {
	if !Printable(s) || s[0] != '/' || strings.HasPrefix(s, "//") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.User != nil {
		return false
	}
	// url.Parse checks the escapes of the path and fragment but keeps the
	// raw query unchecked. url.QueryUnescape checks them without
	// url.ParseQuery's refusal of ';'.
	_, err = url.QueryUnescape(u.RawQuery)
	return err == nil
}
