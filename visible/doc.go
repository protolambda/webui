// Package visible shows untrusted text, such as the names, titles and
// labels that other users or other systems chose, so that a reader sees
// what the text is.
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
// Indic scripts write with. [Text] and [Code] render the result as chord
// nodes. When anything is escaped, they show the escaped form in a code
// element, followed by [Marker], a badge that says so. [Rune] reports
// whether a single character shows as itself. All of them are pure
// functions, safe for concurrent use.
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
// Once anything in a value is escaped, every backslash in it is escaped
// too, so a value cannot forge an escape: a literal "\u202E" in escaped text
// reads "\\u202E". Text that needs no escape is returned as it is,
// backslashes and all. The reported bool, or the marker that [Text] and
// [Code] add, tells the two cases apart, so show it wherever the escaped
// form is shown.
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
// Escaping removes the direction controls, but right-to-left letters still
// order the characters around them, by the Unicode bidirectional algorithm.
// [Text] renders text in a bdi element, and [Code] in a code element with
// dir="ltr"; both isolate the text, so that it cannot reorder the page text
// next to it. Put the results of [Escape] and [EscapeText] in such an
// element too.
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
// Characters that show can still crowd out other text: a long run of
// combining marks draws over the lines above and below it. Bound the length
// of untrusted values when they are entered, and clip the elements that show
// them where the layout matters.
//
// # Unicode data
//
// The character classes come from the tables of the Go unicode package
// (Unicode 17.0 in Go 1.27). Characters that those tables do not assign are
// escaped, because a font may draw them as anything. Recognizing emoji
// sequences needs the Extended_Pictographic property, which the unicode
// package does not have; this package embeds it as a table of 78 ranges,
// derived from the Unicode Character Database and the same in Unicode 15.0
// and 16.0. The property reserves code point ranges for future emoji, so a
// new emoji falls in it once the unicode tables assign the character.
package visible
