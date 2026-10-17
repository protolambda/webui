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
	brailleBlank = '\u2800' // braille pattern blank
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
//     fillers and the braille pattern blank;
//   - variation selectors, except a presentation selector (U+FE0E for text,
//     U+FE0F for emoji) right after an emoji (an Extended_Pictographic
//     character), and U+FE0F in a keycap ('0' to '9', '#' or '*', the
//     selector, then U+20E3);
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
// display text.
func Escape(s string) (string, bool) {
	return escape(s, false)
}

// EscapeText is [Escape] for display text, such as names and titles: it
// also keeps these invisible characters, which take part in how common text
// shows:
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
	return escape(s, true)
}

// Rune reports whether r shows as itself on its own: it is a printable
// character (a letter, mark, number, punctuation, symbol, or the ASCII
// space, see [unicode.IsPrint]), but not a default-ignorable character that
// fonts draw as nothing (Other_Default_Ignorable_Code_Point, such as the
// combining grapheme joiner U+034F, the Hangul fillers U+115F, U+1160,
// U+3164 and U+FFA0, and the Khmer vowels U+17B4 and U+17B5), not a
// variation selector, and not the braille pattern blank U+2800.
//
// Within text, the neighbors matter too: [Escape] also replaces ASCII
// spaces that HTML collapses, and combining marks with nothing to combine
// with, and keeps presentation selectors after emoji.
func Rune(r rune) bool {
	return unicode.IsPrint(r) && r != brailleBlank &&
		!unicode.In(r, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector)
}

// escape escapes s (see [Escape]), with the leniency of [EscapeText] if
// lenient.
func escape(s string, lenient bool) (string, bool) {
	if showsAsIs(s, lenient) {
		return s, false
	}
	var b strings.Builder
	b.Grow(len(s) + len(s)/4 + 16)
	prev := false
	for i := 0; i < len(s); {
		n, shows := unit(s, i, prev, lenient)
		switch {
		case !shows:
			writeEscape(&b, s[i:i+n])
		case s[i] == '\\':
			b.WriteString(`\\`)
		default:
			b.WriteString(s[i : i+n])
		}
		prev = shows
		i += n
	}
	return b.String(), true
}

// showsAsIs reports whether every unit of s shows as itself (see unit).
func showsAsIs(s string, lenient bool) bool {
	for i := 0; i < len(s); {
		n, shows := unit(s, i, i > 0, lenient)
		if !shows {
			return false
		}
		i += n
	}
	return true
}

// unit returns the size in bytes of the unit at byte i of s, and whether it
// shows as itself. A unit is a character, a byte of invalid UTF-8, or, if
// lenient, a flag with its tag characters (see [EscapeText]); a unit that
// does not show is always a single character or byte. prev reports whether
// the unit before it shows as itself (false at the start of s): characters
// that show only next to others need a shown character before them.
func unit(s string, i int, prev, lenient bool) (int, bool) {
	if c := s[i]; c < utf8.RuneSelf {
		switch {
		case c == ' ':
			return 1, plainSpace(s, i)
		case c > ' ' && c < 0x7F:
			return 1, true
		}
		return 1, false // a control character
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	switch {
	case r == utf8.RuneError && size == 1:
		return 1, false
	case r == textSelector, r == emojiSelect:
		return size, prev && selects(s, i, r, size)
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

// selects reports whether the presentation selector r (of size bytes) at
// byte i of s selects how the character before it shows: right after an
// emoji, or as U+FE0F in a keycap (see [Escape]). The caller checks that the
// character before it shows.
func selects(s string, i int, r rune, size int) bool {
	base, _ := runeBefore(s, i)
	switch {
	case isEmoji(base):
		return true
	case base >= '0' && base <= '9', base == '#', base == '*':
		return r == emojiSelect && runeAt(s, i+size) == keycap
	}
	return false
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
// i of s is between two characters that show, neither a space. The caller
// checks that the character before it shows.
func betweenVisible(s string, i, size int) bool {
	after := runeAt(s, i+size)
	return s[i-1] != ' ' && after != ' ' && Rune(after)
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
