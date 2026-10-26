package webuitest

import (
	"iter"
	"slices"
	"strings"

	"github.com/protolambda/chord/ct"
)

// elementsByTag iterates over the element descendants of root in document
// order whose tag is tag, compared case-insensitively.
func elementsByTag(root ct.Node, tag string) iter.Seq[ct.Node] {
	tag = strings.ToLower(tag)
	return func(yield func(ct.Node) bool) {
		for e := range ct.Elements(root) {
			if e.Tag() == tag && !yield(e) {
				return
			}
		}
	}
}

// AttrIs reports whether n has the attribute name with exactly value. The
// name is matched case-insensitively, as HTML does: views keep the case in
// which attributes were written, parsed pages lower-case them.
func AttrIs(n ct.Node, name, value string) bool {
	v, ok := attrOf(n, name)
	return ok && v == value
}

// HasClass reports whether the class attribute of n contains the token
// class.
func HasClass(n ct.Node, class string) bool {
	v, ok := attrOf(n, "class")
	return ok && hasField(v, class)
}

// attrOf returns the value of the attribute name of n, matching the name
// case-insensitively, as [ct.Node.AttrFold] does.
func attrOf(n ct.Node, name string) (string, bool) {
	if n == nil {
		return "", false
	}
	return n.AttrFold(name)
}

func hasAttr(n ct.Node, name string) bool {
	_, ok := attrOf(n, name)
	return ok
}

// hxAttrOf returns the value of an htmx or extension attribute, which htmx
// also reads with a data- prefix: hxAttrOf(n, "hx-get") reads hx-get or
// data-hx-get. The name of the attribute found is returned for messages.
func hxAttrOf(n ct.Node, name string) (value, attrName string, ok bool) {
	if v, ok := attrOf(n, name); ok {
		return v, name, true
	}
	data := dataName(name)
	if v, ok := attrOf(n, data); ok {
		return v, data, true
	}
	return "", "", false
}

// hxValue returns the value of an htmx attribute as htmx's getAttributeValue
// reads it: the hx- attribute when it is not empty, else the data- one. An
// empty value means none. The name of the attribute read is returned for
// messages.
func hxValue(n ct.Node, name string) (value, attrName string) {
	if v, _ := attrOf(n, name); v != "" {
		return v, name
	}
	data := dataName(name)
	if v, _ := attrOf(n, data); v != "" {
		return v, data
	}
	return "", ""
}

// dataName returns name with the data- prefix. The rules look up htmx
// attributes on every element, and mostly miss: the names of the htmx
// attributes that this package reads come from dataNames, so that these
// lookups do not build a string each.
func dataName(name string) string {
	if data, ok := dataNames[name]; ok {
		return data
	}
	return "data-" + name
}

// dataNames maps the htmx and extension attributes that this package reads
// to their names with the data- prefix.
var dataNames = func() map[string]string {
	names := slices.Concat(htmxRequestAttrs, htmxTargetAttrs, []string{
		"hx-disable", "hx-disinherit", "hx-encoding", "hx-ext", "hx-headers",
		"hx-params", "hx-preserve", "hx-push-url", "hx-replace-url",
		"hx-trigger", "hx-vals", "sse-close", "sse-connect", "sse-swap",
		"ws-connect",
	})
	out := make(map[string]string, len(names))
	for _, name := range names {
		out[name] = "data-" + name
	}
	return out
}()

// hxName returns an attribute name without a data- prefix, lower-cased.
func hxName(key string) string {
	key = strings.ToLower(key)
	return strings.TrimPrefix(key, "data-")
}

// hasField reports whether the whitespace-separated list contains token.
func hasField(list, token string) bool {
	for f := range strings.FieldsSeq(list) {
		if f == token {
			return true
		}
	}
	return false
}

// hasFieldFold is hasField with ASCII case folding, for keyword lists such
// as rel.
func hasFieldFold(list, token string) bool {
	for f := range strings.FieldsSeq(list) {
		if strings.EqualFold(f, token) {
			return true
		}
	}
	return false
}

// loc returns the location of n for messages.
func loc(n ct.Node) string {
	if l := n.Location(); l != "" {
		return l
	}
	return "(root)"
}

// isFragment reports whether root is the root of a fragment rather than a
// complete document: it has no html element.
func isFragment(root ct.Node) bool {
	for c := range root.Children() {
		if c.Kind() == ct.KindElement && c.Tag() == "html" {
			return false
		}
	}
	return true
}

// rootOf returns the root of the tree that contains n.
func rootOf(n ct.Node) ct.Node {
	for {
		p := n.Parent()
		if p == nil {
			return n
		}
		n = p
	}
}

// idIndex maps the non-empty ids of the elements below root to the first
// element with each.
func idIndex(root ct.Node) map[string]ct.Node {
	out := make(map[string]ct.Node)
	for e := range ct.Elements(root) {
		if id, ok := attrOf(e, "id"); ok && id != "" {
			if _, dup := out[id]; !dup {
				out[id] = e
			}
		}
	}
	return out
}

// inside returns the nearest ancestor of n (not n itself) with the tag, or
// nil.
func inside(n ct.Node, tag string) ct.Node {
	if n == nil {
		return nil
	}
	return ct.Closest(n.Parent(), func(p ct.Node) bool { return p.Tag() == tag })
}

// rawText returns the text of the descendants of n, without collapsing
// whitespace.
func rawText(n ct.Node) string {
	var b strings.Builder
	for d := range ct.Descendants(n) {
		if d.Kind() == ct.KindText {
			b.WriteString(d.Data())
		}
	}
	return b.String()
}

// collapse trims s and collapses runs of whitespace into one space.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// mediaEssence returns the lower-cased type and subtype of a MIME type,
// without parameters.
func mediaEssence(v string) string {
	t, _, _ := strings.Cut(v, ";")
	return strings.ToLower(strings.TrimSpace(t))
}
