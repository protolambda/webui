// Package visible shows untrusted text, such as the names, titles, labels
// and descriptions that other users or other systems chose, so that a
// reader sees what the text is.
//
// HTML escaping, which chord applies to all text, keeps such text from
// becoming markup. It does not keep the text from misleading a reader. A
// right-to-left override (U+202E) shows "invoice\u202Efdp.exe" as
// "invoiceexe.pdf", and an unclosed one also reorders the page text after
// it. A zero-width space makes "ad\u200Bmin" a second "admin". Line breaks
// and runs of spaces collapse, so two different values look the same, and
// invalid UTF-8 shows as U+FFFD.
//
// [Escape] replaces every character that does not show as itself with an
// escape, and reports whether it replaced anything. [EscapeText] does the
// same, but is a little more lenient, for display text: it also keeps emoji
// sequences, no-break spaces between words, and the joiners that Arabic and
// Indic scripts write with. [EscapeProse] is EscapeText for text of several
// lines: it also keeps line breaks, spaces and tabs. [Text], [Code] and
// [Prose] render the result as chord nodes (Prose with a br element at each
// line break). When anything is escaped, they show the escaped form in a
// code element, preceded by [Marker], a badge that says so; [Marking] renders
// that badge with other words, such as a translation. [TextString] and
// [CodeString] return the same as text, for places that hold only text:
// the value as it is when nothing is escaped, and otherwise the word of the
// marker, a space, and the escaped form. [Rune] reports whether a single
// character shows as itself. All of them are pure functions, safe for
// concurrent use.
//
// # Escapes
//
// The escapes look like Go's, with uppercase hex digits:
//
//	\n \r \t      a line feed, carriage return or tab
//	\x20          an ASCII space that HTML would collapse or hide
//	\xHH          a byte (80 to FF) that is not part of valid UTF-8
//	\uHHHH        a character up to U+FFFF
//	\UHHHHHHHH    a character above U+FFFF
//	\\            a backslash
//
// Once anything in a value is escaped, every backslash (U+005C) in it is
// escaped too, so that a backslash in the value cannot start an escape: a
// literal "\u202E" in escaped text reads "\\u202E". Characters that only
// look like a backslash show as themselves and are not escaped (see below).
// Text that needs no escape is returned as it is, backslashes and all. The
// reported bool, or the marker that [Text], [Code] and [Prose] add, tells
// the two cases apart, so show it wherever the escaped form is shown: as
// [Marker] in markup, and as the word that [TextString] and [CodeString]
// write in text.
//
// The escaped form is valid UTF-8, at most six times as long as the input,
// and shows as itself: escaping it again changes nothing. It decodes
// unambiguously back to the input.
//
// # Where to use it
//
// Use [Text] for names and titles that people read, and [Code] for values
// that are read character by character, such as identifiers, host names,
// file names and URLs. Use [Escape] where a value decides what a reader
// trusts, for example the name of an account that asks for access: it
// escapes the invisible characters that [EscapeText] keeps for display (a
// no-break space in "Ada\u00A0Lovelace" looks like the ASCII space of
// another account's "Ada Lovelace"). A validator can refuse such values when
// they are entered: a value that [Escape] escapes does not show as itself.
//
// Use [Prose] for text of several lines, such as a description or a note.
// [Text] and [Code] escape line breaks and tabs, because a name must not
// hide text on lines out of view, so they would show every line break of a
// description as an escape, with the marker. Prose keeps line breaks, and
// leaves spaces and tabs for HTML to collapse: none of them hides text
// within a line. It escapes everything else as [Text] does, and shows the
// marker only then. Prose that differs only in white space looks the same,
// so names and titles still go through [Text].
//
// Line breaks can still move text out of view. Each one renders a br
// element, and blank lines do not collapse, so a run of them pushes the
// text after it down, past the end of a box that clips its content at a
// fixed height, and no marker shows. Bound the number of lines of
// untrusted prose when it is entered, or show prose in a box that grows
// with it or scrolls, not in one that clips it by height.
//
// Escaping removes the direction controls, but right-to-left letters still
// order the characters around them, by the Unicode bidirectional algorithm.
// [Text] renders text in a bdi element, [Prose] each of its lines, and
// [Code] in a code element with dir="ltr"; all of them isolate the text, so
// that it cannot reorder the page text next to it. Put the results of
// [Escape], [EscapeText] and [EscapeProse] in such an element too.
//
// Some places hold only text: attribute values such as aria-label, title,
// alt and placeholder, the label of an option, the document title, the
// message of hx-confirm. There, use [TextString] and [CodeString], which
// write the word of the marker before an escaped value. The result of
// [EscapeText] alone would drop the marker, and an escaped value would read
// the same as a value with those backslashes in it. Never use the string
// forms where a value is submitted or parsed, such as the value of an
// input, the content of a textarea or an href: the marker and the escapes
// would become part of the value.
//
// # What it does not do
//
// Look-alike characters show as themselves, so they are not escaped: the
// Cyrillic "\u0430" (U+0430) in "\u0430dmin" is a visible letter. Telling
// such names apart needs the Unicode confusables data and a policy on mixed
// scripts, which depend on the application. Common answers are to show who
// chose a name, to warn when a name equals or resembles another one, or to
// refuse characters outside the scripts that the application expects.
//
// The same holds for characters that look like a backslash, which can
// imitate an escape: the reverse solidus operator U+29F5, the set minus
// U+2216, the big, small and fullwidth reverse solidus U+29F9, U+FE68 and
// U+FF3C, and the combining reverse solidus overlay U+20E5. In escaped
// text, "\u29F5u202E" looks like the escape of U+202E. These characters
// belong to no script, so a policy on scripts lets them through. Where a
// value decides what a reader trusts, refuse them when the value is
// entered, or compare confusable skeletons.
//
// Values that [Escape] keeps can still look alike. Whether a presentation
// selector changes how an emoji shows depends on the platform: "\u2764" and
// "\u2764\uFE0F" both show as a red heart on some, and so do "1\u20E3" and
// "1\uFE0F\u20E3". An accented letter can be one character or a letter and
// a combining accent: "Jos\u00E9" and "Jose\u0301" look the same. Where
// names must be unique, compare them on a canonical key: normalized to NFC
// (for example with the golang.org/x/text/unicode/norm package), without the
// presentation selectors U+FE0E and U+FE0F.
//
// The marker shows as a badge, and the escaped value as code, so that a
// reader tells them apart from plain text by their style. In extracted text
// and to a screen reader, and on screen in the string forms, the marker is
// just the word before the value, and a value that starts with that word
// and has literal backslashes reads the same. Where that matters, such as
// for account names, refuse values that [Escape] escapes, and backslashes,
// when they are entered.
//
// Characters that show can still crowd out other text: a long run of
// combining marks draws over the lines above and below it. Bound the length
// of untrusted values when they are entered, and clip the elements that show
// them where the layout matters. Clipping must keep the marker in view:
// [Text], [Code] and [Prose] put it before the value, so that clipping the
// end, for example with Bootstrap's text-truncate class
// ([github.com/protolambda/webui/ba.TextTruncate]), cuts the value and keeps
// the marker. Put [Marker] before the value in markup of your own too.
//
// # Unicode data
//
// The character classes come from the tables of the Go unicode package
// (Unicode 17.0 in Go 1.27). Characters that those tables do not assign are
// escaped, because a font may draw them as anything. The emoji rules need
// properties that the unicode package does not have, so this package embeds
// them as tables, derived from the emoji data of Unicode 17.0
// (emoji-data.txt and emoji-variation-sequences.txt): the
// Extended_Pictographic property that emoji sequences are made of (156
// ranges), and the emoji that presentation selectors apply to, those that
// show as text by default (116 ranges) and those that show as emoji by
// default but have a text form (85 ranges). Extended_Pictographic reserves
// code point ranges for future emoji, so a new emoji falls in it once the
// unicode tables assign the character; new emoji show as emoji by default,
// so [Escape] escapes a presentation selector after them.
package visible
