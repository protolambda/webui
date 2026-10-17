package webuitest

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/protolambda/chord/ct"
)

// NoInlineScripts forbids script that a strict script-src ('self', without
// 'unsafe-inline' or nonces) blocks: script elements without src, and
// iframe srcdoc documents. JSON data blocks (script type="application/json"
// or "application/ld+json"), which browsers do not run, are allowed.
// javascript: URLs are [SafeURLs]' concern, and on* handlers
// [ct.NoInlineHandlers]'.
func NoInlineScripts() ct.Rule {
	return ct.CustomRule("noInlineScripts()", func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			switch e.Tag() {
			case "script":
				if hasAttr(e, "src") || jsonDataBlock(e) {
					continue
				}
				out = append(out, "inline script at "+loc(e))
			case "iframe":
				if hasAttr(e, "srcdoc") {
					out = append(out, "iframe at "+loc(e)+" has an inline srcdoc document")
				}
			}
		}
		return out
	})
}

// jsonDataBlock reports whether a script element is a JSON data block.
func jsonDataBlock(e ct.Node) bool {
	t, _ := attrOf(e, "type")
	switch mediaEssence(t) {
	case "application/json", "application/ld+json":
		return true
	}
	return false
}

// NoInlineStyles forbids styles that a strict style-src ('self', without
// 'unsafe-inline') blocks: style elements and style attributes. Use classes
// of same-origin stylesheets instead (Bootstrap's utilities cover most
// needs).
func NoInlineStyles() ct.Rule {
	return ct.CustomRule("noInlineStyles()", func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			if e.Tag() == "style" {
				out = append(out, "style element at "+loc(e))
			}
			if hasAttr(e, "style") {
				out = append(out, "style attribute at "+loc(e))
			}
		}
		return out
	})
}

// NoHTMXScripting forbids htmx attributes that make htmx compile and run
// JavaScript from the page: hx-on attributes (hx-on:click, hx-on::after-swap,
// hx-on-click, the htmx 1 hx-on), hx-vars, "js:" and "javascript:" values of
// hx-vals and hx-headers, and event filters in square brackets in
// hx-trigger, each also with a data- prefix. [ct.NoInlineHandlers] does not
// catch them, as their names do not start with "on". webui's htmx
// configuration (allowEval false) makes htmx ignore them, and a strict
// script-src would block them, so they would only fail silently. Square
// brackets in an hx-trigger value are reported even inside a from: or
// target: selector, where they are not filters.
func NoHTMXScripting() ct.Rule {
	return ct.CustomRule("noHTMXScripting()", func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			for key, v := range e.Attrs() {
				name := hxName(key)
				switch {
				case strings.HasPrefix(name, "hx-on"):
					out = append(out, fmt.Sprintf("%s at %s is an htmx script handler", key, loc(e)))
				case name == "hx-vars":
					out = append(out, fmt.Sprintf("%s at %s evaluates script", key, loc(e)))
				case name == "hx-vals" || name == "hx-headers":
					if scriptValue(v) {
						out = append(out, fmt.Sprintf("%s at %s has a script value", key, loc(e)))
					}
				case name == "hx-trigger":
					if strings.Contains(v, "[") {
						out = append(out, fmt.Sprintf("%s at %s has an event filter, which evaluates script", key, loc(e)))
					}
				}
			}
		}
		return out
	})
}

// scriptValue reports whether an hx-vals or hx-headers value is evaluated
// as script.
func scriptValue(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.HasPrefix(v, "js:") || strings.HasPrefix(v, "javascript:")
}

// SafeURLs forbids URLs that run script or carry documents: javascript: and
// vbscript: URLs in any URL attribute (href, src, action, formaction, the
// hx- request attributes, sse-connect, ws-connect, and others), and data:
// URLs except images where images load (img, picture sources, image inputs,
// video posters, icon links) and the empty "data:," icon. URLs are read as
// browsers read them: case-insensitively, after dropping surrounding spaces
// and control characters and the tabs and line breaks inside.
func SafeURLs() ct.Rule {
	return ct.CustomRule("safeURLs()", func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			for _, u := range urlAttrs(e) {
				ref := parseRef(u.url)
				switch ref.scheme {
				case "javascript", "vbscript":
					out = append(out, fmt.Sprintf("%s at %s is a %s: URL", u.attr, loc(e), ref.scheme))
				case "data":
					if !dataAllowed(u, ref) {
						out = append(out, fmt.Sprintf("%s at %s is a data: URL that is not an image", u.attr, loc(e)))
					}
				}
			}
		}
		return out
	})
}

// LocalResources requires that the page loads resources and sends requests
// only to its own origin, as a strict content security policy ('self')
// allows: the URLs of scripts, fetching links (stylesheets, icons,
// preloads), images, media, frames, objects, form actions and formaction,
// pings, htmx requests, sse-connect and ws-connect must be paths (or
// relative references) without a scheme or a host. "//host" and the
// backslash spellings that browsers read as it are refused. data: images
// that [SafeURLs] allows are fine. Links (a href) may go anywhere.
func LocalResources() ct.Rule {
	return ct.CustomRule("localResources()", func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			for _, u := range urlAttrs(e) {
				if !u.resource {
					continue
				}
				ref := parseRef(u.url)
				switch {
				case ref.scheme == "data" && dataAllowed(u, ref):
				case ref.scheme == "javascript" || ref.scheme == "vbscript" || ref.scheme == "data":
					// SafeURLs reports these.
				case ref.authority:
					out = append(out, fmt.Sprintf("%s at %s names a host, not a same-origin path", u.attr, loc(e)))
				case ref.scheme != "":
					out = append(out, fmt.Sprintf("%s at %s is an absolute %s: URL, not a same-origin path", u.attr, loc(e), ref.scheme))
				}
			}
		}
		return out
	})
}

// SubresourceIntegrity requires an integrity value on every script with a
// src and every stylesheet, module preload and preload link: at least one
// well-formed sha256, sha384 or sha512 digest. With it, a browser refuses a
// file that differs from the one the page was rendered for, also from a
// cache. The rule checks the form of the value; [Crawl] can check it against
// the served bytes.
func SubresourceIntegrity() ct.Rule {
	return ct.CustomRule("subresourceIntegrity()", func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			switch e.Tag() {
			case "script":
				if src, ok := attrOf(e, "src"); !ok || src == "" {
					continue
				}
			case "link":
				rel, _ := attrOf(e, "rel")
				if !hasFieldFold(rel, "stylesheet") && !hasFieldFold(rel, "modulepreload") && !hasFieldFold(rel, "preload") {
					continue
				}
			default:
				continue
			}
			v, ok := attrOf(e, "integrity")
			switch {
			case !ok:
				out = append(out, e.Tag()+" at "+loc(e)+" has no integrity attribute")
			case !validIntegrity(v):
				out = append(out, e.Tag()+" at "+loc(e)+" has no well-formed sha256, sha384 or sha512 integrity value")
			}
		}
		return out
	})
}

// digestSizes are the digest lengths of the SRI hash algorithms.
var digestSizes = map[string]int{"sha256": 32, "sha384": 48, "sha512": 64}

// validIntegrity reports whether an integrity value has at least one
// well-formed digest.
func validIntegrity(v string) bool {
	for tok := range strings.FieldsSeq(v) {
		if _, _, ok := parseIntegrity(tok); ok {
			return true
		}
	}
	return false
}

// parseIntegrity parses one integrity token, "alg-base64[?options]".
func parseIntegrity(tok string) (alg string, digest []byte, ok bool) {
	tok, _, _ = strings.Cut(tok, "?")
	alg, b64, found := strings.Cut(tok, "-")
	if !found {
		return "", nil, false
	}
	alg = strings.ToLower(alg)
	size, known := digestSizes[alg]
	if !known {
		return "", nil, false
	}
	d, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(d) != size {
		return "", nil, false
	}
	return alg, d, true
}
