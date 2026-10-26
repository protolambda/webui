package visible

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Characters that show as themselves only next to certain others.
const (
	zwnj         = '\u200C' // zero width non-joiner
	zwj          = '\u200D' // zero width joiner
	textSelector = '\uFE0E' // variation selector 15: text presentation
	emojiSelect  = '\uFE0F' // variation selector 16: emoji presentation
	keycap       = '\u20E3' // combining enclosing keycap
	nbsp         = '\u00A0' // no-break space
	narrowNBSP   = '\u202F' // narrow no-break space
)

// Printable symbols that fonts draw as a blank (see [Rune]).
const (
	brailleBlank = '\u2800'     // braille pattern blank
	nullNotehead = '\U0001D159' // musical symbol null notehead
)

// Escape returns s with every character that does not show as itself
// replaced by an escape (see the package documentation for the escapes),
// and reports whether it replaced anything. If not, it returns s itself,
// without allocating.
//
// Escape replaces:
//   - invalid UTF-8, byte by byte;
//   - control characters, also line feeds and tabs: a value must not hide
//     text on lines out of view, or pass for another value once HTML
//     collapses its white space;
//   - format characters, which show as nothing: the direction controls
//     (U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069), the
//     zero-width space, joiners and word joiner, the byte order mark, the
//     soft hyphen, and the tag characters (U+E0001 and U+E0020 to
//     U+E007F), which can carry hidden text;
//   - other characters that fonts draw as nothing or as a blank (see
//     [Rune]): for example the combining grapheme joiner U+034F, the Hangul
//     fillers, the braille pattern blank U+2800 and the musical symbol null
//     notehead U+1D159;
//   - variation selectors, except a presentation selector where it selects
//     how an emoji shows, as in the emoji variation sequences of Unicode:
//     U+FE0F (emoji) or U+FE0E (text) right after an emoji that shows as
//     text by default, such as U+2764 or U+00A9; U+FE0E right after an
//     emoji that shows as emoji by default but has a text form, such as
//     U+231A; and U+FE0F in a keycap ('0' to '9', '#' or '*', the selector,
//     then U+20E3). Anywhere else a presentation selector shows as nothing:
//     U+FE0F after U+1F600 or U+231A, which show as emoji anyway, or a
//     selector after U+2605, which has no emoji form;
//   - combining marks at the start of s or after an escaped character: they
//     would draw on the text before the value, or on the escape;
//   - spaces other than the ASCII space, line and paragraph separators;
//   - ASCII spaces that HTML collapses or hides: leading and trailing ones,
//     and runs of two or more;
//   - private-use characters and code points that the unicode tables do not
//     assign, which fonts may draw as anything;
//   - backslashes, once anything else is replaced.
//
// The result is valid UTF-8, at most six times as long as s, and shows as
// itself: Escape of the result returns it unchanged. See [EscapeText] for
// display text, and [EscapeProse] for text of several lines.
func Escape(s string) (string, bool) {
	return escape(s, strict)
}

// EscapeText is [Escape] for display text, such as names and titles: it
// also keeps these invisible characters, which take part in how common text
// shows:
//   - a presentation selector (U+FE0E or U+FE0F) right after any emoji (an
//     Extended_Pictographic character), also where it changes nothing, as
//     in U+1F600 U+FE0F: some keyboards and programs add one to every emoji;
//   - a zero-width joiner (U+200D) between two emoji, which joins them into
//     one: the emoji before it may have its emoji presentation selector
//     (U+FE0F) or a skin tone (U+1F3FB to U+1F3FF), as in the emoji ZWJ
//     sequences of Unicode, such as U+1F469 U+1F3FD U+200D U+1F4BB (a
//     technologist with a medium skin tone);
//   - a zero-width non-joiner (U+200C) or joiner (U+200D) between two
//     letters or marks of one script that is written with them: Arabic,
//     Syriac, N'Ko, Mongolian, Devanagari, Bengali, Gurmukhi, Gujarati,
//     Oriya, Tamil, Telugu, Kannada, Malayalam or Sinhala (in Persian, for
//     example, the non-joiner is part of the spelling of many words);
//   - a no-break space (U+00A0) or narrow no-break space (U+202F) between
//     two characters that show, neither a space, as in "10\u00A0km", and in
//     the dates and times that some formatters write;
//   - the flags of England, Scotland and Wales: U+1F3F4 followed by the tag
//     characters of "gbeng", "gbsct" or "gbwls" and the cancel tag U+E007F.
//     These are the only emoji tag sequences that Unicode recommends; every
//     other tag character is still escaped.
//
// Text that differs only in these characters looks the same: "Ada Lovelace"
// with a no-break space looks like "Ada Lovelace" with a space. Use [Escape]
// where that matters. Everything else that [Escape] replaces, EscapeText
// replaces too, so the result of [Escape] needs no further escape by
// EscapeText.
func EscapeText(s string) (string, bool) {
	return escape(s, display)
}

// EscapeProse is [EscapeText] for prose, text of several lines such as a
// description or a note. It keeps the white space that [Prose] shows as
// line breaks, or that HTML collapses into one space, as it is:
//   - line breaks: a line feed (U+000A), or a carriage return and a line
//     feed;
//   - ASCII spaces and tabs, also at the start or end of a line and in
//     runs.
//
// None of it hides text within a line, but many line breaks can push the
// text after them out of a box that clips at a fixed height, unmarked (see
// the package documentation). Everything else is escaped as [EscapeText]
// escapes it: the other control characters (a carriage return on its own
// too), the line and paragraph separators U+2028 and U+2029, the other
// spaces, such as a no-break space next to white space, the direction
// controls and the invisible characters. A character that shows only after
// another one, such as a combining mark, is escaped at the start of s and
// of every line, and after white space other than a single space after
// another character. EscapeProse reports whether it escaped anything, which
// white space alone never is. If not, it returns s itself, without
// allocating.
//
// As with [EscapeText], the result decodes unambiguously back to s, and
// EscapeProse of the result returns it unchanged. Prose that differs only
// in white space looks the same: a tab or two spaces between two words look
// like one space. Use [EscapeText] or [Escape] for values that must look
// different, such as names.
func EscapeProse(s string) (string, bool) {
	return escape(s, prose)
}

// Rune reports whether r shows as itself on its own: it is a printable
// character (a letter, mark, number, punctuation, symbol, or the ASCII
// space, see [unicode.IsPrint]), but not a default-ignorable character that
// fonts draw as nothing (Other_Default_Ignorable_Code_Point, such as the
// combining grapheme joiner U+034F, the Hangul fillers U+115F, U+1160,
// U+3164 and U+FFA0, and the Khmer vowels U+17B4 and U+17B5), not a
// variation selector, and not a symbol that fonts draw as a blank: the
// braille pattern blank U+2800 or the musical symbol null notehead U+1D159.
//
// Within text, the neighbors matter too: [Escape] also replaces ASCII
// spaces that HTML collapses, and combining marks with nothing to combine
// with, and keeps the presentation selectors that select how an emoji
// shows.
func Rune(r rune) bool {
	return unicode.IsPrint(r) && r != brailleBlank && r != nullNotehead &&
		!unicode.In(r, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector)
}

// mode is the leniency of an escape.
type mode uint8

const (
	strict  mode = iota // [Escape]
	display             // [EscapeText]
	prose               // [EscapeProse]
)

// escape escapes s (see [Escape]), with the leniency of mode m.
func escape(s string, m mode) (string, bool) {
	if showsAsIs(s, m) {
		return s, false
	}
	var b strings.Builder
	b.Grow(len(s) + len(s)/4 + 16)
	prev := false
	for i := 0; i < len(s); {
		n, shows := unit(s, i, prev, m)
		switch {
		case !shows:
			writeEscape(&b, s[i:i+n])
		case s[i] == '\\':
			b.WriteString(`\\`)
		default:
			b.WriteString(s[i : i+n])
		}
		prev = shows && base(s, i, m)
		i += n
	}
	return b.String(), true
}

// showsAsIs reports whether every unit of s shows as itself (see unit).
func showsAsIs(s string, m mode) bool {
	prev := false
	for i := 0; i < len(s); {
		n, shows := unit(s, i, prev, m)
		if !shows {
			return false
		}
		prev = base(s, i, m)
		i += n
	}
	return true
}

// unit returns the size in bytes of the unit at byte i of s, and whether it
// shows as itself. A unit is a character, a byte of invalid UTF-8, in prose
// a line break (a line feed, or a carriage return and a line feed), or, if
// lenient (display or prose), a flag with its tag characters (see
// [EscapeText]); a unit that does not show is always a single character or
// byte. prev reports whether the unit before it shows as itself and is a
// base (see base; false at the start of s): characters that show only next
// to others need such a unit before them.
func unit(s string, i int, prev bool, m mode) (int, bool) {
	lenient := m != strict
	if c := s[i]; c < utf8.RuneSelf {
		switch {
		case c > ' ' && c < 0x7F:
			return 1, true
		case m == prose:
			return proseSpace(s, i)
		case c == ' ':
			return 1, plainSpace(s, i)
		}
		return 1, false // a control character
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	switch {
	case r == utf8.RuneError && size == 1:
		return 1, false
	case r == textSelector, r == emojiSelect:
		return size, prev && selects(s, i, r, size, lenient)
	case unicode.Is(unicode.M, r):
		return size, prev && Rune(r)
	case Rune(r):
		if lenient && r == blackFlag {
			return size + tagFlag(s[i+size:]), true
		}
		return size, true
	case !lenient || !prev:
		return size, false
	case r == zwj:
		return size, joinsEmoji(s, i, size) || joinsLetters(s, i, size)
	case r == zwnj:
		return size, joinsLetters(s, i, size)
	case r == nbsp, r == narrowNBSP:
		return size, betweenVisible(s, i, size)
	}
	return size, false
}

// plainSpace reports whether the ASCII space at byte i of s shows in HTML
// text: it is neither the first nor the last character of s, and not next
// to another space.
func plainSpace(s string, i int) bool {
	return i > 0 && i < len(s)-1 && s[i-1] != ' ' && s[i+1] != ' '
}

// proseSpace returns the size of the ASCII control character or space at
// byte i of s, and whether it shows as itself in prose (see [EscapeProse]):
// a space, a tab, a line feed, or a carriage return and a line feed, as one
// unit.
func proseSpace(s string, i int) (int, bool) {
	switch s[i] {
	case ' ', '\t', '\n':
		return 1, true
	case '\r':
		if i+1 < len(s) && s[i+1] == '\n' {
			return 2, true
		}
	}
	return 1, false
}

// base reports whether the unit at byte i of s, which shows as itself, is
// a base for the characters that show only next to others, such as
// combining marks (see unit). Every unit is, except in prose: there, line
// breaks and tabs are not, and a space is only right after a character
// that is not white space, where HTML keeps it.
func base(s string, i int, m mode) bool {
	if m != prose {
		return true
	}
	switch s[i] {
	case '\t', '\n', '\r':
		return false
	case ' ':
		return i > 0 && !asciiSpace(s[i-1])
	}
	return true
}

// asciiSpace reports whether c is white space that HTML collapses: a
// space, a tab, a line feed or a carriage return.
func asciiSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// selects reports whether the presentation selector r (of size bytes) at
// byte i of s selects how the character before it shows (see [Escape]):
// U+FE0F after an emoji that shows as text by default (hasEmojiStyle),
// U+FE0E after an emoji that has a text form (hasTextStyle), or U+FE0F in a
// keycap. If lenient, a selector after any emoji will do (see
// [EscapeText]). The caller checks that the character before it shows.
func selects(s string, i int, r rune, size int, lenient bool) bool {
	base, _ := runeBefore(s, i)
	switch {
	case base >= '0' && base <= '9', base == '#', base == '*':
		return r == emojiSelect && runeAt(s, i+size) == keycap
	case lenient:
		return isEmoji(base)
	case r == emojiSelect:
		return hasEmojiStyle(base)
	}
	return hasTextStyle(base)
}

// joinsEmoji reports whether the zero-width joiner (of size bytes) at byte i
// of s joins two emoji (see [EscapeText]).
func joinsEmoji(s string, i, size int) bool {
	before, n := runeBefore(s, i)
	if before == emojiSelect || isSkinTone(before) {
		before, _ = runeBefore(s, i-n)
	}
	return isEmoji(before) && isEmoji(runeAt(s, i+size))
}

// joinerScripts are the scripts that are written with the zero-width
// joiners (see [EscapeText]).
var joinerScripts = []*unicode.RangeTable{
	unicode.Arabic, unicode.Syriac, unicode.Nko, unicode.Mongolian,
	unicode.Devanagari, unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati, unicode.Oriya,
	unicode.Tamil, unicode.Telugu, unicode.Kannada, unicode.Malayalam, unicode.Sinhala,
}

// joinsLetters reports whether the zero-width joiner or non-joiner (of size
// bytes) at byte i of s is between two letters or marks of one of the
// joinerScripts. The caller checks that the character before it shows; the
// one after it must show too.
func joinsLetters(s string, i, size int) bool {
	before, _ := runeBefore(s, i)
	after := runeAt(s, i+size)
	if !unicode.In(before, unicode.L, unicode.M) || !unicode.In(after, unicode.L, unicode.M) || !Rune(after) {
		return false
	}
	for _, script := range joinerScripts {
		if unicode.Is(script, before) {
			return unicode.Is(script, after)
		}
	}
	return false
}

// betweenVisible reports whether the no-break space (of size bytes) at byte
// i of s is between two characters that show, neither white space. The
// caller checks that the character before it shows.
func betweenVisible(s string, i, size int) bool {
	after := runeAt(s, i+size)
	return !asciiSpace(s[i-1]) && after != ' ' && Rune(after)
}

// runeAt returns the character at byte i of s, or -1 if there is none: at
// the end of s, or at invalid UTF-8.
func runeAt(s string, i int) rune {
	r, n := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && n <= 1 {
		return -1
	}
	return r
}

// runeBefore returns the character that ends at byte i of s and its size,
// or -1 if there is none: at the start of s, or after invalid UTF-8.
func runeBefore(s string, i int) (rune, int) {
	r, n := utf8.DecodeLastRuneInString(s[:i])
	if r == utf8.RuneError && n <= 1 {
		return -1, n
	}
	return r, n
}

// hexDigits are the digits of the escapes.
const hexDigits = "0123456789ABCDEF"

// writeEscape writes the escape of u, a single character or byte of invalid
// UTF-8.
func writeEscape(b *strings.Builder, u string) {
	r, size := utf8.DecodeRuneInString(u)
	switch {
	case r == utf8.RuneError && size == 1:
		writeHex(b, `\x`, uint32(u[0]), 2)
	case r == '\n':
		b.WriteString(`\n`)
	case r == '\r':
		b.WriteString(`\r`)
	case r == '\t':
		b.WriteString(`\t`)
	case r == ' ':
		b.WriteString(`\x20`)
	case r <= 0xFFFF:
		writeHex(b, `\u`, uint32(r), 4)
	default:
		writeHex(b, `\U`, uint32(r), 8)
	}
}

// writeHex writes prefix and the lowest digits hex digits of v.
func writeHex(b *strings.Builder, prefix string, v uint32, digits int) {
	b.WriteString(prefix)
	for shift := 4 * (digits - 1); shift >= 0; shift -= 4 {
		b.WriteByte(hexDigits[v>>uint(shift)&0xF])
	}
}
