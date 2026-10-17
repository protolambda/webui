package webuitest

import (
	"fmt"
	"strings"

	"github.com/protolambda/chord/ct"
)

// htmxTargetAttrs are the htmx attributes whose values select elements.
var htmxTargetAttrs = []string{"hx-target", "hx-include", "hx-indicator", "hx-disabled-elt"}

// HXTargets requires that the elements that hx-target, hx-include,
// hx-indicator and hx-disabled-elt select (also with a data- prefix)
// exist, so that a swap, an included value or an indicator does not
// silently go nowhere:
//
//   - "#id" names an element of the document; in a fragment (a subject
//     without an html element), also one of documentTargets, the ids the
//     page around the fragment provides (see [DocumentTargets]);
//   - "closest S", "find S", "next S" and "previous S" find an element
//     matching S among the ancestors, descendants, following or preceding
//     elements; in a fragment, closest, next and previous may also find
//     their element outside the fragment, so only find is checked there;
//   - another selector matches some element of a document.
//
// Selectors are checked when they are simple: a tag, an id, classes and
// [attr] or [attr=value] tests, with htmx's <S/> wrapping. Keywords such as
// "this", "next", "previous", "body", "document" and "window" always
// resolve, and other selectors (descendant combinators, pseudo-classes)
// are not checked. Each comma-separated part must resolve. hx-swap-oob ids
// are not checked: they name elements of the page the fragment is swapped
// into.
func HXTargets(documentTargets ...string) ct.Rule {
	allowed := make(map[string]bool, len(documentTargets))
	quoted := make([]string, len(documentTargets))
	for i, id := range documentTargets {
		allowed[id] = true
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return ct.CustomRule("hxTargets("+strings.Join(quoted, ", ")+")", func(root ct.Node) []string {
		frag := isFragment(root)
		ids := idIndex(root)
		var elems []ct.Node
		for e := range Elements(root, "") {
			elems = append(elems, e)
		}
		var out []string
		for i, e := range elems {
			for _, name := range htmxTargetAttrs {
				v, attr, ok := hxAttrOf(e, name)
				if !ok {
					continue
				}
				for _, part := range splitSelectors(v) {
					if msg := resolveTarget(elems, i, part, ids, frag, allowed); msg != "" {
						out = append(out, fmt.Sprintf("%s at %s %s", attr, loc(e), msg))
					}
				}
			}
		}
		return out
	})
}

// resolveTarget checks one extended selector of the element elems[i], and
// describes the problem, or returns "".
func resolveTarget(elems []ct.Node, i int, part string, ids map[string]ct.Node, frag bool, allowed map[string]bool) string {
	e := elems[i]
	p := unwrapSelector(part)
	switch p {
	case "", "this", "body", "document", "window", "next", "previous", "inherit", "root":
		return ""
	}
	if rest, ok := cutKeyword(p, "closest"); ok {
		s, ok := parseSimple(unwrapSelector(rest))
		if !ok || frag || Closest(e, s.match) != nil {
			return ""
		}
		return fmt.Sprintf("finds no %q ancestor", rest)
	}
	if rest, ok := cutKeyword(p, "find"); ok {
		s, ok := parseSimple(unwrapSelector(rest))
		if !ok {
			return ""
		}
		for d := range Elements(e, "") {
			if s.match(d) {
				return ""
			}
		}
		return fmt.Sprintf("finds no %q descendant", rest)
	}
	if rest, ok := cutKeyword(p, "next"); ok {
		s, ok := parseSimple(unwrapSelector(rest))
		if !ok || frag {
			return ""
		}
		for _, d := range elems[i+1:] {
			if s.match(d) && !isAncestor(e, d) {
				return ""
			}
		}
		return fmt.Sprintf("finds no following %q", rest)
	}
	if rest, ok := cutKeyword(p, "previous"); ok {
		s, ok := parseSimple(unwrapSelector(rest))
		if !ok || frag {
			return ""
		}
		for j := i - 1; j >= 0; j-- {
			if s.match(elems[j]) && !isAncestor(elems[j], e) {
				return ""
			}
		}
		return fmt.Sprintf("finds no preceding %q", rest)
	}
	if simpleIDSelector(p) {
		id := p[1:]
		if ids[id] != nil || (frag && allowed[id]) {
			return ""
		}
		return fmt.Sprintf("targets missing id %q", id)
	}
	s, ok := parseSimple(p)
	if !ok || frag {
		return ""
	}
	for _, d := range elems {
		if s.match(d) {
			return ""
		}
	}
	return fmt.Sprintf("selects no element with %q", p)
}

// cutKeyword cuts "keyword " from the start of s.
func cutKeyword(s, keyword string) (string, bool) {
	rest, ok := strings.CutPrefix(s, keyword)
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// unwrapSelector removes htmx's "<S/>" wrapping of a selector.
func unwrapSelector(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, "/>") {
		return strings.TrimSpace(s[1 : len(s)-2])
	}
	return s
}

// splitSelectors splits a selector list at the commas outside brackets,
// parentheses and quotes, and trims the parts. Empty parts are dropped.
func splitSelectors(v string) []string {
	var out []string
	depth := 0
	var quote byte
	start := 0
	flush := func(end int) {
		if p := strings.TrimSpace(v[start:end]); p != "" {
			out = append(out, p)
		}
		start = end + 1
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[' || c == '(':
			depth++
		case c == ']' || c == ')':
			if depth > 0 {
				depth--
			}
		case c == ',' && depth == 0:
			flush(i)
		}
	}
	flush(len(v))
	return out
}

// identByte reports whether c may appear in a CSS identifier (ASCII
// letters, digits, '-', '_', and the bytes of non-ASCII characters).
func identByte(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' || c >= 0x80
}

// simpleIDSelector reports whether s is "#ident".
func simpleIDSelector(s string) bool {
	if len(s) < 2 || s[0] != '#' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !identByte(s[i]) {
			return false
		}
	}
	return true
}

// simpleSelector is a compound CSS selector without combinators or
// pseudo-classes.
type simpleSelector struct {
	tag     string // "" or "*" for any
	id      string
	classes []string
	attrs   []attrTest
}

type attrTest struct {
	name, value string
	hasValue    bool
}

// parseSimple parses a compound selector of a tag, ids, classes and
// [attr] or [attr=value] tests. It reports false for anything else.
func parseSimple(s string) (simpleSelector, bool) {
	var sel simpleSelector
	s = strings.TrimSpace(s)
	if s == "" {
		return sel, false
	}
	ident := func(i int) int {
		j := i
		for j < len(s) && identByte(s[j]) {
			j++
		}
		return j
	}
	i := 0
	if s[0] == '*' {
		sel.tag = "*"
		i = 1
	} else if j := ident(0); j > 0 {
		sel.tag = strings.ToLower(s[:j])
		i = j
	}
	for i < len(s) {
		switch s[i] {
		case '#', '.':
			j := ident(i + 1)
			if j == i+1 {
				return sel, false
			}
			if s[i] == '#' {
				if sel.id != "" {
					return sel, false
				}
				sel.id = s[i+1 : j]
			} else {
				sel.classes = append(sel.classes, s[i+1:j])
			}
			i = j
		case '[':
			end := strings.IndexByte(s[i:], ']')
			if end < 0 {
				return sel, false
			}
			body := s[i+1 : i+end]
			i += end + 1
			name, value, hasValue := strings.Cut(body, "=")
			name = strings.TrimSpace(name)
			if !attrName(name) {
				return sel, false
			}
			value = strings.TrimSpace(value)
			if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
				value = value[1 : len(value)-1]
			}
			sel.attrs = append(sel.attrs, attrTest{name: strings.ToLower(name), value: value, hasValue: hasValue})
		default:
			return sel, false
		}
	}
	return sel, true
}

// attrName reports whether s is an attribute name of identifier bytes,
// possibly with a namespace colon.
func attrName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !identByte(s[i]) && s[i] != ':' {
			return false
		}
	}
	return true
}

func (s simpleSelector) match(n ct.Node) bool {
	if n.Kind() != ct.KindElement {
		return false
	}
	if s.tag != "" && s.tag != "*" && n.Tag() != s.tag {
		return false
	}
	if s.id != "" && !AttrIs(n, "id", s.id) {
		return false
	}
	for _, c := range s.classes {
		if !HasClass(n, c) {
			return false
		}
	}
	for _, a := range s.attrs {
		v, ok := attrOf(n, a.name)
		if !ok || (a.hasValue && v != a.value) {
			return false
		}
	}
	return true
}

// SSEWiring checks the markup of htmx server-sent event regions (the SSE
// extension, see the sse package): every element with sse-swap, or with an
// hx-trigger on an "sse:" event, sits in an element with sse-connect (or is
// that element), whose events it receives; every sse-connect has a URL, has
// the extension enabled by hx-ext="sse" on itself or an ancestor, and names
// an sse-close event, without which the browser reconnects forever when
// the stream ends; sse-swap names no empty event. In a fragment (a subject
// without an html element), the connect element and hx-ext may be in the
// page around it, so missing ancestors are not reported there.
func SSEWiring() ct.Rule {
	return ct.CustomRule("sseWiring()", func(root ct.Node) []string {
		frag := isFragment(root)
		connect := func(n ct.Node) bool {
			_, _, ok := hxAttrOf(n, "sse-connect")
			return ok
		}
		var out []string
		for e := range Elements(root, "") {
			if v, attr, ok := hxAttrOf(e, "sse-swap"); ok {
				for name := range strings.SplitSeq(v, ",") {
					if strings.TrimSpace(name) == "" {
						out = append(out, fmt.Sprintf("%s at %s names an empty event", attr, loc(e)))
						break
					}
				}
				if !frag && Closest(e, connect) == nil {
					out = append(out, fmt.Sprintf("%s at %s is outside every sse-connect element", attr, loc(e)))
				}
			}
			if v, attr, ok := hxAttrOf(e, "hx-trigger"); ok && triggersSSE(v) && !frag && Closest(e, connect) == nil {
				out = append(out, fmt.Sprintf("%s at %s waits for an SSE event outside every sse-connect element", attr, loc(e)))
			}
			if v, attr, ok := hxAttrOf(e, "sse-connect"); ok {
				if strings.TrimSpace(v) == "" {
					out = append(out, fmt.Sprintf("%s at %s has no URL", attr, loc(e)))
				}
				if !frag && !sseEnabled(e) {
					out = append(out, fmt.Sprintf("%s at %s is not inside hx-ext=\"sse\"", attr, loc(e)))
				}
				if c, _, ok := hxAttrOf(e, "sse-close"); !ok || strings.TrimSpace(c) == "" {
					out = append(out, fmt.Sprintf("%s at %s has no sse-close event, so the browser reconnects forever", attr, loc(e)))
				}
			}
		}
		return out
	})
}

// sseEnabled reports whether the SSE extension applies to e: the nearest
// hx-ext that mentions it, on e or an ancestor, enables it.
func sseEnabled(e ct.Node) bool {
	for p := e; p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		v, _, ok := hxAttrOf(p, "hx-ext")
		if !ok {
			continue
		}
		for ext := range strings.SplitSeq(v, ",") {
			switch strings.TrimSpace(ext) {
			case "sse":
				return true
			case "ignore:sse":
				return false
			}
		}
	}
	return false
}

// triggersSSE reports whether an hx-trigger value has a trigger on an
// "sse:" event.
func triggersSSE(v string) bool {
	for trig := range strings.SplitSeq(v, ",") {
		if f := strings.Fields(trig); len(f) > 0 && strings.HasPrefix(f[0], "sse:") {
			return true
		}
	}
	return false
}
