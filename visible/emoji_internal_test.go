package visible

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// TestExtendedPictographicTable checks the shape of the embedded table and
// what the escaping relies on: emoji are neither marks, spaces nor
// controls, so that they show wherever they are.
func TestExtendedPictographicTable(gt *testing.T) {
	t := devtest.ParallelT(gt)
	tab := extendedPictographic
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
	t.Must(be.Equal(78, len(tab.R16)+len(tab.R32)))

	count := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !unicode.Is(tab, r) {
			continue
		}
		count++
		t.Mustf(be.False(unicode.In(r, unicode.M, unicode.Z, unicode.Cc, unicode.Cf)), "%U is a mark, space or control", r)
	}
	t.Must(be.Equal(3537, count)) // the size of the property in emoji-data.txt

	for _, r := range []rune{'\u00A9', '\u00AE', '\u203C', '\u2122', '\u2139', '\u2764', '\u26A7', '\U0001F308',
		'\U0001F3F3', '\U0001F3F4', '\U0001F468', '\U0001F525', '\U0001F600', '\U0001F9B0', '\U0001FAF8'} {
		t.Mustf(be.True(isEmoji(r)), "isEmoji(%U)", r)
	}
	for _, r := range []rune{'#', '*', '0', '9', 'a', '\u20E3', '\u2190', '\uFE0F', '\U0001F1E6', '\U0001F1FF',
		'\U0001F3FB', '\U0001F3FF', '\U0001FC00', -1} {
		t.Mustf(be.False(isEmoji(r)), "isEmoji(%U)", r)
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
