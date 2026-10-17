package visible

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"
)

// MarkerText is the visible text of [Marker].
const MarkerText = "escaped"

// MarkerTitle is the title (tooltip) of [Marker]: why the value before it is
// shown with escapes.
const MarkerTitle = "Shown with escapes: the value has invisible, control or text direction characters, " +
	"or spaces that would not show"

// Text renders display text, such as a name or a title, escaped by
// [EscapeText]. Text that needs no escape renders in a bdi element, which
// isolates its direction, so that right-to-left text cannot reorder the page
// text around it:
//
//	<bdi>Ada Lovelace</bdi>
//
// Escaped text renders as code, left to right, followed by a space and
// [Marker]:
//
//	<code dir="ltr">invoice\u202Efdp.exe</code> <span class="badge ...">escaped</span>
//
// Empty text renders an empty bdi element; render a placeholder instead
// where an empty value needs one. The output has no style attributes, so it
// works under a strict content security policy.
func Text(s string) elem.Node {
	v, escaped := EscapeText(s)
	if !escaped {
		return text.Bdi()(text.Text(v))
	}
	return marked(v)
}

// Code renders a value that is read character by character, such as an
// identifier, a host name, a file name or a URL, strictly escaped by
// [Escape], in a code element that shows it left to right (dir="ltr", which
// also isolates it from the text around it):
//
//	<code dir="ltr">notes.example</code>
//
// Escaped values are followed by a space and [Marker]. Code never renders a
// link: put a value that is a URL in an href only after checking it.
func Code(s string) elem.Node {
	v, escaped := Escape(s)
	if !escaped {
		return code(v)
	}
	return marked(v)
}

// Marker renders the badge that follows an escaped value: a Bootstrap
// warning badge with [MarkerText], and [MarkerTitle] as its title:
//
//	<span class="badge text-bg-warning" title="Shown with escapes: ...">escaped</span>
//
// Use it next to the results of [Escape] and [EscapeText] that report true,
// so that the reader knows to read the backslashes as escapes.
func Marker() elem.Node {
	return text.Span(attr.Class("badge text-bg-warning"), attr.Title(MarkerTitle))(text.Text(MarkerText))
}

// code renders the value v in a code element, left to right.
func code(v string) elem.Node {
	return text.Code(attr.Dir("ltr"))(text.Text(v))
}

// marked renders the escaped value v as code, followed by a space and the
// marker.
func marked(v string) elem.Node {
	return elem.Bundle{code(v), text.Text(" "), Marker()}
}
