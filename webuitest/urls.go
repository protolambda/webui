package webuitest

import (
	"strings"

	"github.com/protolambda/chord/ct"
)

// urlAttr is one URL that an attribute of an element names.
type urlAttr struct {
	attr string // attribute name, as written
	url  string
	// resource is set for URLs that the page loads or sends requests to by
	// itself or when submitted (scripts, styles, images, form actions, htmx
	// requests, event streams), as opposed to navigations (links).
	resource bool
	// image is set for URLs that load an image, where data: images are fine.
	image bool
	// icon is set for link icons, where also the empty "data:," is fine.
	icon bool
}

// htmxRequestAttrs are the htmx attributes whose values are request URLs.
var htmxRequestAttrs = []string{"hx-get", "hx-post", "hx-put", "hx-patch", "hx-delete"}

// fetchingRels are the link types whose href the browser fetches or
// connects to.
var fetchingRels = []string{
	"stylesheet", "icon", "apple-touch-icon", "mask-icon", "preload", "modulepreload",
	"prefetch", "prerender", "manifest", "dns-prefetch", "preconnect",
}

// urlAttrs returns the URLs that the attributes of element e name.
func urlAttrs(e ct.Node) []urlAttr {
	var out []urlAttr
	add := func(name string, u urlAttr) {
		if v, ok := attrOf(e, name); ok {
			u.attr, u.url = name, v
			out = append(out, u)
		}
	}
	addSet := func(name string, u urlAttr) {
		v, ok := attrOf(e, name)
		if !ok {
			return
		}
		for _, c := range srcsetURLs(v) {
			u.attr, u.url = name, c
			out = append(out, u)
		}
	}
	switch e.Tag() {
	case "a", "area":
		add("href", urlAttr{})
		if v, ok := attrOf(e, "ping"); ok {
			for p := range strings.FieldsSeq(v) {
				out = append(out, urlAttr{attr: "ping", url: p, resource: true})
			}
		}
	case "link":
		rel, _ := attrOf(e, "rel")
		fetch := false
		for _, r := range fetchingRels {
			if hasFieldFold(rel, r) {
				fetch = true
			}
		}
		icon := hasFieldFold(rel, "icon") || hasFieldFold(rel, "apple-touch-icon") || hasFieldFold(rel, "mask-icon")
		add("href", urlAttr{resource: fetch, image: icon, icon: icon})
	case "script", "iframe", "embed", "track", "audio":
		add("src", urlAttr{resource: true})
	case "img":
		add("src", urlAttr{resource: true, image: true})
		addSet("srcset", urlAttr{resource: true, image: true})
	case "source":
		img := inside(e, "picture") != nil
		add("src", urlAttr{resource: true, image: img})
		addSet("srcset", urlAttr{resource: true, image: img})
	case "video":
		add("src", urlAttr{resource: true})
		add("poster", urlAttr{resource: true, image: true})
	case "object":
		add("data", urlAttr{resource: true})
	case "form":
		add("action", urlAttr{resource: true})
	case "button":
		add("formaction", urlAttr{resource: true})
	case "input":
		add("formaction", urlAttr{resource: true})
		if t, _ := attrOf(e, "type"); strings.EqualFold(strings.TrimSpace(t), "image") {
			add("src", urlAttr{resource: true, image: true})
		}
	case "blockquote", "q", "del", "ins":
		add("cite", urlAttr{})
	case "base":
		add("href", urlAttr{resource: true})
	}
	for _, name := range htmxRequestAttrs {
		if v, attr, ok := hxAttrOf(e, name); ok {
			out = append(out, urlAttr{attr: attr, url: v, resource: true})
		}
	}
	for _, name := range []string{"hx-push-url", "hx-replace-url"} {
		if v, attr, ok := hxAttrOf(e, name); ok && !strings.EqualFold(v, "true") && !strings.EqualFold(v, "false") {
			out = append(out, urlAttr{attr: attr, url: v})
		}
	}
	for _, name := range []string{"sse-connect", "ws-connect"} {
		if v, attr, ok := hxAttrOf(e, name); ok {
			out = append(out, urlAttr{attr: attr, url: v, resource: true})
		}
	}
	return out
}

// srcsetURLs returns the URLs of the image candidates of a srcset value.
// Candidates are separated by commas that follow whitespace or a
// descriptor; URLs themselves may contain commas, so this follows the
// shape of the HTML parsing algorithm: a URL runs up to whitespace, and a
// trailing comma ends it.
func srcsetURLs(v string) []string {
	var out []string
	for {
		v = strings.TrimLeft(v, " \t\n\r\f,")
		if v == "" {
			return out
		}
		end := strings.IndexAny(v, " \t\n\r\f")
		if end < 0 {
			end = len(v)
		}
		u := v[:end]
		v = v[end:]
		trailing := strings.HasSuffix(u, ",")
		u = strings.TrimRight(u, ",")
		if u != "" {
			out = append(out, u)
		}
		if !trailing {
			// Skip the descriptors up to the next comma outside parentheses.
			depth, i := 0, 0
		scan:
			for ; i < len(v); i++ {
				switch v[i] {
				case '(':
					depth++
				case ')':
					if depth > 0 {
						depth--
					}
				case ',':
					if depth == 0 {
						break scan
					}
				}
			}
			v = v[i:]
		}
	}
}

// urlRef is a URL reference as a browser reads it.
type urlRef struct {
	scheme    string // lower-cased; "" for a relative reference
	authority bool   // names a host: "//host", or "\\host" and mixtures
	rest      string // after "scheme:"
}

// parseRef reads a URL reference the way browsers do before resolving it:
// leading and trailing C0 control characters and spaces are dropped, tabs
// and line breaks anywhere are removed, and a backslash counts as a slash
// (browsers read it so for http and https URLs, and so for references
// relative to them).
func parseRef(v string) urlRef {
	v = strings.TrimFunc(v, func(r rune) bool { return r <= ' ' })
	if strings.ContainsAny(v, "\t\n\r") {
		v = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(v)
	}
	var r urlRef
	if i := schemeEnd(v); i > 0 {
		r.scheme = strings.ToLower(v[:i])
		v = v[i+1:]
	}
	r.rest = v
	slashed := strings.ReplaceAll(v, `\`, "/")
	switch r.scheme {
	case "", "http", "https", "ws", "wss", "ftp", "file":
		r.authority = strings.HasPrefix(slashed, "//")
		if r.scheme != "" && !r.authority && r.scheme != "file" {
			// "https:host/x" relative to a page of another scheme still names
			// a host: special schemes skip any number of slashes.
			r.authority = true
		}
	}
	return r
}

// schemeEnd returns the index of the colon that ends a scheme at the start
// of v, or -1 when v starts with no scheme.
func schemeEnd(v string) int {
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.'):
		case i > 0 && c == ':':
			return i
		default:
			return -1
		}
	}
	return -1
}

// dataMediaType returns the lower-cased media type of a data: URL's rest,
// "" when it has none (as in "data:,").
func dataMediaType(rest string) string {
	head, _, _ := strings.Cut(rest, ",")
	return mediaEssence(head)
}

// dataAllowed reports whether a data: URL is fine where u appears: images
// in image contexts, and the empty "data:," icon that keeps a browser from
// requesting /favicon.ico.
func dataAllowed(u urlAttr, ref urlRef) bool {
	mt := dataMediaType(ref.rest)
	if u.image && strings.HasPrefix(mt, "image/") {
		return true
	}
	return u.icon && mt == ""
}
