package visible

import (
	"strings"
	"unicode"
)

// blackFlag is the base of the emoji tag sequences (see tagFlags).
const blackFlag = '\U0001F3F4'

// tagFlags are the emoji tag sequences that Unicode recommends, without
// their base blackFlag: the tag characters (U+E0020 to U+E007E) of the
// subdivision codes gbeng, gbsct and gbwls, then the cancel tag U+E007F.
var tagFlags = [...]string{
	"\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F", // England
	"\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F", // Scotland
	"\U000E0067\U000E0062\U000E0077\U000E006C\U000E0073\U000E007F", // Wales
}

// tagFlag returns the length of the tag characters of a flag at the start
// of rest (see tagFlags), or 0 if rest does not start with them.
func tagFlag(rest string) int {
	for _, tags := range tagFlags {
		if strings.HasPrefix(rest, tags) {
			return len(tags)
		}
	}
	return 0
}

// isEmoji reports whether r is an emoji that shows: an Extended_Pictographic
// character that the unicode tables assign (see [Rune]).
func isEmoji(r rune) bool {
	return unicode.Is(extendedPictographic, r) && Rune(r)
}

// isSkinTone reports whether r is an emoji modifier: one of the five skin
// tones U+1F3FB to U+1F3FF.
func isSkinTone(r rune) bool {
	return r >= '\U0001F3FB' && r <= '\U0001F3FF'
}

// extendedPictographic is the Extended_Pictographic property of the Unicode
// Character Database (emoji-data.txt), the same in Unicode 15.0 and 16.0:
// the characters that emoji are made of, with ranges reserved for future
// emoji. It does not include the skin tones, the regional indicators of
// flags, or the keycap bases '0' to '9', '#' and '*'.
var extendedPictographic = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x00A9, Hi: 0x00A9, Stride: 1}, {Lo: 0x00AE, Hi: 0x00AE, Stride: 1}, {Lo: 0x203C, Hi: 0x203C, Stride: 1},
		{Lo: 0x2049, Hi: 0x2049, Stride: 1}, {Lo: 0x2122, Hi: 0x2122, Stride: 1}, {Lo: 0x2139, Hi: 0x2139, Stride: 1},
		{Lo: 0x2194, Hi: 0x2199, Stride: 1}, {Lo: 0x21A9, Hi: 0x21AA, Stride: 1}, {Lo: 0x231A, Hi: 0x231B, Stride: 1},
		{Lo: 0x2328, Hi: 0x2328, Stride: 1}, {Lo: 0x2388, Hi: 0x2388, Stride: 1}, {Lo: 0x23CF, Hi: 0x23CF, Stride: 1},
		{Lo: 0x23E9, Hi: 0x23F3, Stride: 1}, {Lo: 0x23F8, Hi: 0x23FA, Stride: 1}, {Lo: 0x24C2, Hi: 0x24C2, Stride: 1},
		{Lo: 0x25AA, Hi: 0x25AB, Stride: 1}, {Lo: 0x25B6, Hi: 0x25B6, Stride: 1}, {Lo: 0x25C0, Hi: 0x25C0, Stride: 1},
		{Lo: 0x25FB, Hi: 0x25FE, Stride: 1}, {Lo: 0x2600, Hi: 0x2605, Stride: 1}, {Lo: 0x2607, Hi: 0x2612, Stride: 1},
		{Lo: 0x2614, Hi: 0x2685, Stride: 1}, {Lo: 0x2690, Hi: 0x2705, Stride: 1}, {Lo: 0x2708, Hi: 0x2712, Stride: 1},
		{Lo: 0x2714, Hi: 0x2714, Stride: 1}, {Lo: 0x2716, Hi: 0x2716, Stride: 1}, {Lo: 0x271D, Hi: 0x271D, Stride: 1},
		{Lo: 0x2721, Hi: 0x2721, Stride: 1}, {Lo: 0x2728, Hi: 0x2728, Stride: 1}, {Lo: 0x2733, Hi: 0x2734, Stride: 1},
		{Lo: 0x2744, Hi: 0x2744, Stride: 1}, {Lo: 0x2747, Hi: 0x2747, Stride: 1}, {Lo: 0x274C, Hi: 0x274C, Stride: 1},
		{Lo: 0x274E, Hi: 0x274E, Stride: 1}, {Lo: 0x2753, Hi: 0x2755, Stride: 1}, {Lo: 0x2757, Hi: 0x2757, Stride: 1},
		{Lo: 0x2763, Hi: 0x2767, Stride: 1}, {Lo: 0x2795, Hi: 0x2797, Stride: 1}, {Lo: 0x27A1, Hi: 0x27A1, Stride: 1},
		{Lo: 0x27B0, Hi: 0x27B0, Stride: 1}, {Lo: 0x27BF, Hi: 0x27BF, Stride: 1}, {Lo: 0x2934, Hi: 0x2935, Stride: 1},
		{Lo: 0x2B05, Hi: 0x2B07, Stride: 1}, {Lo: 0x2B1B, Hi: 0x2B1C, Stride: 1}, {Lo: 0x2B50, Hi: 0x2B50, Stride: 1},
		{Lo: 0x2B55, Hi: 0x2B55, Stride: 1}, {Lo: 0x3030, Hi: 0x3030, Stride: 1}, {Lo: 0x303D, Hi: 0x303D, Stride: 1},
		{Lo: 0x3297, Hi: 0x3297, Stride: 1}, {Lo: 0x3299, Hi: 0x3299, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1F000, Hi: 0x1F0FF, Stride: 1}, {Lo: 0x1F10D, Hi: 0x1F10F, Stride: 1}, {Lo: 0x1F12F, Hi: 0x1F12F, Stride: 1},
		{Lo: 0x1F16C, Hi: 0x1F171, Stride: 1}, {Lo: 0x1F17E, Hi: 0x1F17F, Stride: 1}, {Lo: 0x1F18E, Hi: 0x1F18E, Stride: 1},
		{Lo: 0x1F191, Hi: 0x1F19A, Stride: 1}, {Lo: 0x1F1AD, Hi: 0x1F1E5, Stride: 1}, {Lo: 0x1F201, Hi: 0x1F20F, Stride: 1},
		{Lo: 0x1F21A, Hi: 0x1F21A, Stride: 1}, {Lo: 0x1F22F, Hi: 0x1F22F, Stride: 1}, {Lo: 0x1F232, Hi: 0x1F23A, Stride: 1},
		{Lo: 0x1F23C, Hi: 0x1F23F, Stride: 1}, {Lo: 0x1F249, Hi: 0x1F3FA, Stride: 1}, {Lo: 0x1F400, Hi: 0x1F53D, Stride: 1},
		{Lo: 0x1F546, Hi: 0x1F64F, Stride: 1}, {Lo: 0x1F680, Hi: 0x1F6FF, Stride: 1}, {Lo: 0x1F774, Hi: 0x1F77F, Stride: 1},
		{Lo: 0x1F7D5, Hi: 0x1F7FF, Stride: 1}, {Lo: 0x1F80C, Hi: 0x1F80F, Stride: 1}, {Lo: 0x1F848, Hi: 0x1F84F, Stride: 1},
		{Lo: 0x1F85A, Hi: 0x1F85F, Stride: 1}, {Lo: 0x1F888, Hi: 0x1F88F, Stride: 1}, {Lo: 0x1F8AE, Hi: 0x1F8FF, Stride: 1},
		{Lo: 0x1F90C, Hi: 0x1F93A, Stride: 1}, {Lo: 0x1F93C, Hi: 0x1F945, Stride: 1}, {Lo: 0x1F947, Hi: 0x1FAFF, Stride: 1},
		{Lo: 0x1FC00, Hi: 0x1FFFD, Stride: 1},
	},
	LatinOffset: 2,
}
