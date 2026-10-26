package visible

import (
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"
)

// MarkerText is the visible text of [Marker].
const MarkerText = "escaped"

// MarkerTitle is the title (tooltip) of [Marker]: why the value after it is
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
// Escaped text renders as [Marker], a space, and the escaped text as code,
// left to right:
//
//	<span class="badge ...">escaped</span> <code dir="ltr">invoice\u202Efdp.exe</code>
//
// The marker comes first, so that a screen reader announces it before the
// value, and clipping the end of the text cuts the value, not the marker.
// Empty text renders an empty bdi element; render a placeholder instead
// where an empty value needs one. The output has no style attributes, so it
// works under a strict content security policy. [Marking.Text] renders the
// marker with other words, such as a translation.
func Text(s string) elem.Node {
	return Marking{}.Text(s)
}

// Code renders a value that is read character by character, such as an
// identifier, a host name, a file name or a URL, strictly escaped by
// [Escape], in a code element that shows it left to right (dir="ltr", which
// also isolates it from the text around it):
//
//	<code dir="ltr">notes.example</code>
//
// Escaped values render after [Marker] and a space, as in [Text]. Code
// never renders a link: put a value that is a URL in an href only after
// checking it. [Marking.Code] renders the marker with other words, such as a
// translation.
func Code(s string) elem.Node {
	return Marking{}.Code(s)
}

// TextString returns display text, such as a name or a title, for places
// that hold only text: attribute values such as aria-label, title, alt and
// placeholder, the label of an option, the document title, the message of
// hx-confirm. Text that needs no escape is returned as it is; escaped text
// (by [EscapeText]) is returned after [MarkerText] and a space, the words
// that [Text] shows in its badge:
//
//	visible.TextString("Ada Lovelace")        // Ada Lovelace
//	visible.TextString("invoice\u202Efdp.exe") // escaped invoice\u202Efdp.exe
//
// The result reads as [Text] does in extracted text and to a screen
// reader, but without the badge the marker is only a word on screen too: a
// value that starts with that word and has literal backslashes looks the
// same (see the package documentation). Nor can a text-only place isolate
// the direction of the value: right-to-left letters in it still order the
// text next to it in the same attribute or option. Never use the result
// where a value is submitted or parsed, such as the value of an input, the
// content of a textarea or an href: the marker and the escapes would become
// part of the value. [Marking.TextString] uses other words.
func TextString(s string) string {
	return Marking{}.TextString(s)
}

// CodeString returns a value that is read character by character, such as
// an identifier, a host name, a file name or a URL, for places that hold
// only text, as [TextString] does for display text: the value as it is when
// [Escape] escapes nothing, and otherwise [MarkerText], a space, and the
// escaped value. [Marking.CodeString] uses other words.
func CodeString(s string) string {
	return Marking{}.CodeString(s)
}

// Marker renders the badge that precedes an escaped value: a Bootstrap
// warning badge with [MarkerText], and [MarkerTitle] as its title:
//
//	<span class="badge text-bg-warning" title="Shown with escapes: ...">escaped</span>
//
// Put it, and a space, before the results of [Escape] and [EscapeText] that
// report true, so that the reader knows to read the backslashes (U+005C) as
// escapes, and so that clipping the end of the text keeps it in view. The
// title is a tooltip, which only mouse users get; the visible text is what
// every reader gets. [Marking.Marker] renders it with other words.
func Marker() elem.Node {
	return Marking{}.Marker()
}

// Marking is the marker that [Marking.Text] and [Marking.Code] put before
// escaped values, and whose label [Marking.TextString] and
// [Marking.CodeString] write before them, with words of the application's
// choice, such as a translation. The zero value is the English marker of
// [Text], [Code], [TextString], [CodeString] and [Marker]. A Marking is a
// small value, safe to copy and to use concurrently.
type Marking struct {
	// Label is the visible text of the badge, and the word before escaped
	// values in the string forms; empty or blank means [MarkerText]. The
	// marker must never be empty, because it is what tells an escaped value
	// apart from text with literal backslashes.
	Label string
	// Title is the title (tooltip) of the badge: why the value is shown
	// with escapes. Empty or blank means [MarkerTitle].
	Title string
}

// Marker renders the badge of [Marker] with the words of m.
func (m Marking) Marker() elem.Node {
	return text.Span(attr.Class("badge text-bg-warning"), attr.Title(m.title()))(text.Text(m.label()))
}

// label returns the visible text of the marker: Label, or [MarkerText] when
// it is blank.
func (m Marking) label() string {
	if strings.TrimSpace(m.Label) == "" {
		return MarkerText
	}
	return m.Label
}

// title returns the title of the marker: Title, or [MarkerTitle] when it is
// blank.
func (m Marking) title() string {
	if strings.TrimSpace(m.Title) == "" {
		return MarkerTitle
	}
	return m.Title
}

// Text renders s as [Text] does, with the marker of m.
func (m Marking) Text(s string) elem.Node {
	v, escaped := EscapeText(s)
	if !escaped {
		return text.Bdi()(text.Text(v))
	}
	return m.marked(v)
}

// Code renders s as [Code] does, with the marker of m.
func (m Marking) Code(s string) elem.Node {
	v, escaped := Escape(s)
	if !escaped {
		return code(v)
	}
	return m.marked(v)
}

// TextString returns s as [TextString] does, with the label of m: the
// Label, or [MarkerText] when it is blank.
func (m Marking) TextString(s string) string {
	v, escaped := EscapeText(s)
	if !escaped {
		return v
	}
	return m.label() + " " + v
}

// CodeString returns s as [CodeString] does, with the label of m.
func (m Marking) CodeString(s string) string {
	v, escaped := Escape(s)
	if !escaped {
		return v
	}
	return m.label() + " " + v
}

// code renders the value v in a code element, left to right.
func code(v string) elem.Node {
	return text.Code(attr.Dir("ltr"))(text.Text(v))
}

// marked renders the marker of m, a space, and the escaped value v as code.
func (m Marking) marked(v string) elem.Node {
	return elem.Bundle{m.Marker(), text.Text(" "), code(v)}
}
