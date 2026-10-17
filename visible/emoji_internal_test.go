package visible

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// checkTable checks the shape of an embedded table: ranges of stride 1 in
// order, R16 below R32, and the LatinOffset. It returns the number of
// ranges.
func checkTable(t devtest.T, tab *unicode.RangeTable) int {
	prev := rune(-1)
	latin := 0
	for _, r := range tab.R16 {
		t.Mustf(be.Equal(uint16(1), r.Stride), "stride of %#x", r.Lo)
		t.Mustf(be.True(rune(r.Lo) > prev && r.Lo <= r.Hi), "range %#x-%#x out of order", r.Lo, r.Hi)
		prev = rune(r.Hi)
		if r.Hi <= unicode.MaxLatin1 {
			latin++
		}
	}
	for _, r := range tab.R32 {
		t.Mustf(be.Equal(uint32(1), r.Stride), "stride of %#x", r.Lo)
		t.Mustf(be.True(rune(r.Lo) > prev && r.Lo <= r.Hi && r.Lo > 0xFFFF), "range %#x-%#x out of order", r.Lo, r.Hi)
		prev = rune(r.Hi)
	}
	t.Must(be.Equal(latin, tab.LatinOffset))
	return len(tab.R16) + len(tab.R32)
}

// TestExtendedPictographicTable checks the shape of the embedded table and
// what the escaping relies on: emoji are neither marks, spaces nor
// controls, so that they show wherever they are.
func TestExtendedPictographicTable(gt *testing.T) {
	t := devtest.ParallelT(gt)
	tab := extendedPictographic
	t.Must(be.Equal(156, checkTable(t, tab)))

	count := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !unicode.Is(tab, r) {
			continue
		}
		count++
		t.Mustf(be.False(unicode.In(r, unicode.M, unicode.Z, unicode.Cc, unicode.Cf)), "%U is a mark, space or control", r)
	}
	t.Must(be.Equal(2848, count)) // the size of the property in emoji-data.txt of Unicode 17.0

	for _, r := range []rune{'\u00A9', '\u00AE', '\u203C', '\u2122', '\u2139', '\u2764', '\u26A7', '\U0001F004',
		'\U0001F308', '\U0001F3F3', '\U0001F3F4', '\U0001F468', '\U0001F525', '\U0001F600', '\U0001F9B0',
		'\U0001FAF8'} {
		t.Mustf(be.True(isEmoji(r)), "isEmoji(%U)", r)
	}
	// Unicode 17.0 dropped symbols that are not emoji, such as the black
	// star, the other mahjong tiles and the chess symbols.
	for _, r := range []rune{'#', '*', '0', '9', 'a', '\u20E3', '\u2190', '\u2605', '\uFE0F', '\U0001F000',
		'\U0001F1E6', '\U0001F1FF', '\U0001F3FB', '\U0001F3FF', '\U0001FA00', '\U0001FC00', -1} {
		t.Mustf(be.False(isEmoji(r)), "isEmoji(%U)", r)
	}
}

// TestVariationTables checks the tables of the emoji that presentation
// selectors apply to: their shape and size, that they are disjoint emoji
// that the unicode tables assign, and some members.
func TestVariationTables(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(116, checkTable(t, textDefault)))
	t.Must(be.Equal(85, checkTable(t, emojiDefaultText)))
	text, emoji := 0, 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		inText, inEmoji := unicode.Is(textDefault, r), unicode.Is(emojiDefaultText, r)
		if inText {
			text++
		}
		if inEmoji {
			emoji++
		}
		if inText || inEmoji {
			t.Mustf(be.True(isEmoji(r)), "%U is not an emoji that shows", r)
			t.Mustf(be.False(inText && inEmoji), "%U is in both tables", r)
		}
		t.Mustf(be.Equal(inText, hasEmojiStyle(r)), "hasEmojiStyle(%U)", r)
		t.Mustf(be.Equal(inText || inEmoji, hasTextStyle(r)), "hasTextStyle(%U)", r)
	}
	// The emoji variation sequences of Unicode 17.0 have 371 bases: these,
	// and the 12 keycap bases.
	t.Must(be.Equal(207, text))
	t.Must(be.Equal(152, emoji))

	for _, r := range []rune{'\u00A9', '\u00AE', '\u2122', '\u263A', '\u2764', '\U0001F170', '\U0001F6F3'} {
		t.Mustf(be.True(hasEmojiStyle(r) && hasTextStyle(r)), "%U shows as text by default", r)
	}
	for _, r := range []rune{'\u231A', '\u2614', '\U0001F004', '\U0001F44D', '\U0001F6BC'} {
		t.Mustf(be.True(!hasEmojiStyle(r) && hasTextStyle(r)), "%U shows as emoji by default, with a text form", r)
	}
	for _, r := range []rune{'#', '1', 'a', '\u2605', '\U0001F000', '\U0001F600', '\U0001F680', '\U0001F3FD',
		'\U0001FC00', -1} {
		t.Mustf(be.False(hasTextStyle(r)), "%U has no variation sequences", r)
	}
}

// TestTagFlags checks the tag sequences: six tag characters each, of
// lowercase letters, ending with the cancel tag.
func TestTagFlags(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, tags := range tagFlags {
		t.Must(be.Equal(24, len(tags)))
		t.Must(be.Equal(6, utf8.RuneCountInString(tags)))
		i := 0
		for _, r := range tags {
			if i < 5 {
				t.Mustf(be.True(r >= 0xE0061 && r <= 0xE007A), "tag %U", r)
			} else {
				t.Must(be.Equal(rune(0xE007F), r))
			}
			i++
		}
		t.Must(be.Equal(len(tags), tagFlag(tags+"x")))
		t.Must(be.Equal(0, tagFlag(tags[:len(tags)-4])))
	}
}
