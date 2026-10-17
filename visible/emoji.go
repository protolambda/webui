// The emoji tables of this file are derived from the Unicode Character
// Database of Unicode 17.0 (emoji/emoji-data.txt and
// emoji/emoji-variation-sequences.txt, https://www.unicode.org/Public/17.0.0/),
// Copyright © Unicode, Inc., used under the Unicode License v3
// (https://www.unicode.org/license.txt).

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

// hasEmojiStyle reports whether the emoji presentation selector U+FE0F
// changes how r shows: r is an emoji that shows as text by default (see
// textDefault).
func hasEmojiStyle(r rune) bool {
	return unicode.Is(textDefault, r)
}

// hasTextStyle reports whether the text presentation selector U+FE0E
// changes how r shows: r is an emoji that shows as text by default, or one
// that shows as emoji by default but has a text form (see textDefault and
// emojiDefaultText).
func hasTextStyle(r rune) bool {
	return unicode.In(r, textDefault, emojiDefaultText)
}

// extendedPictographic is the Extended_Pictographic property of Unicode 17.0
// (emoji-data.txt): the characters that emoji are made of, with ranges
// reserved for future emoji. It does not include the skin tones, the
// regional indicators of flags, or the keycap bases '0' to '9', '#' and '*'.
var extendedPictographic = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x00A9, Hi: 0x00A9, Stride: 1}, {Lo: 0x00AE, Hi: 0x00AE, Stride: 1}, {Lo: 0x203C, Hi: 0x203C, Stride: 1},
		{Lo: 0x2049, Hi: 0x2049, Stride: 1}, {Lo: 0x2122, Hi: 0x2122, Stride: 1}, {Lo: 0x2139, Hi: 0x2139, Stride: 1},
		{Lo: 0x2194, Hi: 0x2199, Stride: 1}, {Lo: 0x21A9, Hi: 0x21AA, Stride: 1}, {Lo: 0x231A, Hi: 0x231B, Stride: 1},
		{Lo: 0x2328, Hi: 0x2328, Stride: 1}, {Lo: 0x23CF, Hi: 0x23CF, Stride: 1}, {Lo: 0x23E9, Hi: 0x23F3, Stride: 1},
		{Lo: 0x23F8, Hi: 0x23FA, Stride: 1}, {Lo: 0x24C2, Hi: 0x24C2, Stride: 1}, {Lo: 0x25AA, Hi: 0x25AB, Stride: 1},
		{Lo: 0x25B6, Hi: 0x25B6, Stride: 1}, {Lo: 0x25C0, Hi: 0x25C0, Stride: 1}, {Lo: 0x25FB, Hi: 0x25FE, Stride: 1},
		{Lo: 0x2600, Hi: 0x2604, Stride: 1}, {Lo: 0x260E, Hi: 0x260E, Stride: 1}, {Lo: 0x2611, Hi: 0x2611, Stride: 1},
		{Lo: 0x2614, Hi: 0x2615, Stride: 1}, {Lo: 0x2618, Hi: 0x2618, Stride: 1}, {Lo: 0x261D, Hi: 0x261D, Stride: 1},
		{Lo: 0x2620, Hi: 0x2620, Stride: 1}, {Lo: 0x2622, Hi: 0x2623, Stride: 1}, {Lo: 0x2626, Hi: 0x2626, Stride: 1},
		{Lo: 0x262A, Hi: 0x262A, Stride: 1}, {Lo: 0x262E, Hi: 0x262F, Stride: 1}, {Lo: 0x2638, Hi: 0x263A, Stride: 1},
		{Lo: 0x2640, Hi: 0x2640, Stride: 1}, {Lo: 0x2642, Hi: 0x2642, Stride: 1}, {Lo: 0x2648, Hi: 0x2653, Stride: 1},
		{Lo: 0x265F, Hi: 0x2660, Stride: 1}, {Lo: 0x2663, Hi: 0x2663, Stride: 1}, {Lo: 0x2665, Hi: 0x2666, Stride: 1},
		{Lo: 0x2668, Hi: 0x2668, Stride: 1}, {Lo: 0x267B, Hi: 0x267B, Stride: 1}, {Lo: 0x267E, Hi: 0x267F, Stride: 1},
		{Lo: 0x2692, Hi: 0x2697, Stride: 1}, {Lo: 0x2699, Hi: 0x2699, Stride: 1}, {Lo: 0x269B, Hi: 0x269C, Stride: 1},
		{Lo: 0x26A0, Hi: 0x26A1, Stride: 1}, {Lo: 0x26A7, Hi: 0x26A7, Stride: 1}, {Lo: 0x26AA, Hi: 0x26AB, Stride: 1},
		{Lo: 0x26B0, Hi: 0x26B1, Stride: 1}, {Lo: 0x26BD, Hi: 0x26BE, Stride: 1}, {Lo: 0x26C4, Hi: 0x26C5, Stride: 1},
		{Lo: 0x26C8, Hi: 0x26C8, Stride: 1}, {Lo: 0x26CE, Hi: 0x26CF, Stride: 1}, {Lo: 0x26D1, Hi: 0x26D1, Stride: 1},
		{Lo: 0x26D3, Hi: 0x26D4, Stride: 1}, {Lo: 0x26E9, Hi: 0x26EA, Stride: 1}, {Lo: 0x26F0, Hi: 0x26F5, Stride: 1},
		{Lo: 0x26F7, Hi: 0x26FA, Stride: 1}, {Lo: 0x26FD, Hi: 0x26FD, Stride: 1}, {Lo: 0x2702, Hi: 0x2702, Stride: 1},
		{Lo: 0x2705, Hi: 0x2705, Stride: 1}, {Lo: 0x2708, Hi: 0x270D, Stride: 1}, {Lo: 0x270F, Hi: 0x270F, Stride: 1},
		{Lo: 0x2712, Hi: 0x2712, Stride: 1}, {Lo: 0x2714, Hi: 0x2714, Stride: 1}, {Lo: 0x2716, Hi: 0x2716, Stride: 1},
		{Lo: 0x271D, Hi: 0x271D, Stride: 1}, {Lo: 0x2721, Hi: 0x2721, Stride: 1}, {Lo: 0x2728, Hi: 0x2728, Stride: 1},
		{Lo: 0x2733, Hi: 0x2734, Stride: 1}, {Lo: 0x2744, Hi: 0x2744, Stride: 1}, {Lo: 0x2747, Hi: 0x2747, Stride: 1},
		{Lo: 0x274C, Hi: 0x274C, Stride: 1}, {Lo: 0x274E, Hi: 0x274E, Stride: 1}, {Lo: 0x2753, Hi: 0x2755, Stride: 1},
		{Lo: 0x2757, Hi: 0x2757, Stride: 1}, {Lo: 0x2763, Hi: 0x2764, Stride: 1}, {Lo: 0x2795, Hi: 0x2797, Stride: 1},
		{Lo: 0x27A1, Hi: 0x27A1, Stride: 1}, {Lo: 0x27B0, Hi: 0x27B0, Stride: 1}, {Lo: 0x27BF, Hi: 0x27BF, Stride: 1},
		{Lo: 0x2934, Hi: 0x2935, Stride: 1}, {Lo: 0x2B05, Hi: 0x2B07, Stride: 1}, {Lo: 0x2B1B, Hi: 0x2B1C, Stride: 1},
		{Lo: 0x2B50, Hi: 0x2B50, Stride: 1}, {Lo: 0x2B55, Hi: 0x2B55, Stride: 1}, {Lo: 0x3030, Hi: 0x3030, Stride: 1},
		{Lo: 0x303D, Hi: 0x303D, Stride: 1}, {Lo: 0x3297, Hi: 0x3297, Stride: 1}, {Lo: 0x3299, Hi: 0x3299, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1F004, Hi: 0x1F004, Stride: 1}, {Lo: 0x1F02C, Hi: 0x1F02F, Stride: 1}, {Lo: 0x1F094, Hi: 0x1F09F, Stride: 1},
		{Lo: 0x1F0AF, Hi: 0x1F0B0, Stride: 1}, {Lo: 0x1F0C0, Hi: 0x1F0C0, Stride: 1}, {Lo: 0x1F0CF, Hi: 0x1F0D0, Stride: 1},
		{Lo: 0x1F0F6, Hi: 0x1F0FF, Stride: 1}, {Lo: 0x1F170, Hi: 0x1F171, Stride: 1}, {Lo: 0x1F17E, Hi: 0x1F17F, Stride: 1},
		{Lo: 0x1F18E, Hi: 0x1F18E, Stride: 1}, {Lo: 0x1F191, Hi: 0x1F19A, Stride: 1}, {Lo: 0x1F1AE, Hi: 0x1F1E5, Stride: 1},
		{Lo: 0x1F201, Hi: 0x1F20F, Stride: 1}, {Lo: 0x1F21A, Hi: 0x1F21A, Stride: 1}, {Lo: 0x1F22F, Hi: 0x1F22F, Stride: 1},
		{Lo: 0x1F232, Hi: 0x1F23A, Stride: 1}, {Lo: 0x1F23C, Hi: 0x1F23F, Stride: 1}, {Lo: 0x1F249, Hi: 0x1F25F, Stride: 1},
		{Lo: 0x1F266, Hi: 0x1F321, Stride: 1}, {Lo: 0x1F324, Hi: 0x1F393, Stride: 1}, {Lo: 0x1F396, Hi: 0x1F397, Stride: 1},
		{Lo: 0x1F399, Hi: 0x1F39B, Stride: 1}, {Lo: 0x1F39E, Hi: 0x1F3F0, Stride: 1}, {Lo: 0x1F3F3, Hi: 0x1F3F5, Stride: 1},
		{Lo: 0x1F3F7, Hi: 0x1F3FA, Stride: 1}, {Lo: 0x1F400, Hi: 0x1F4FD, Stride: 1}, {Lo: 0x1F4FF, Hi: 0x1F53D, Stride: 1},
		{Lo: 0x1F549, Hi: 0x1F54E, Stride: 1}, {Lo: 0x1F550, Hi: 0x1F567, Stride: 1}, {Lo: 0x1F56F, Hi: 0x1F570, Stride: 1},
		{Lo: 0x1F573, Hi: 0x1F57A, Stride: 1}, {Lo: 0x1F587, Hi: 0x1F587, Stride: 1}, {Lo: 0x1F58A, Hi: 0x1F58D, Stride: 1},
		{Lo: 0x1F590, Hi: 0x1F590, Stride: 1}, {Lo: 0x1F595, Hi: 0x1F596, Stride: 1}, {Lo: 0x1F5A4, Hi: 0x1F5A5, Stride: 1},
		{Lo: 0x1F5A8, Hi: 0x1F5A8, Stride: 1}, {Lo: 0x1F5B1, Hi: 0x1F5B2, Stride: 1}, {Lo: 0x1F5BC, Hi: 0x1F5BC, Stride: 1},
		{Lo: 0x1F5C2, Hi: 0x1F5C4, Stride: 1}, {Lo: 0x1F5D1, Hi: 0x1F5D3, Stride: 1}, {Lo: 0x1F5DC, Hi: 0x1F5DE, Stride: 1},
		{Lo: 0x1F5E1, Hi: 0x1F5E1, Stride: 1}, {Lo: 0x1F5E3, Hi: 0x1F5E3, Stride: 1}, {Lo: 0x1F5E8, Hi: 0x1F5E8, Stride: 1},
		{Lo: 0x1F5EF, Hi: 0x1F5EF, Stride: 1}, {Lo: 0x1F5F3, Hi: 0x1F5F3, Stride: 1}, {Lo: 0x1F5FA, Hi: 0x1F64F, Stride: 1},
		{Lo: 0x1F680, Hi: 0x1F6C5, Stride: 1}, {Lo: 0x1F6CB, Hi: 0x1F6D2, Stride: 1}, {Lo: 0x1F6D5, Hi: 0x1F6E5, Stride: 1},
		{Lo: 0x1F6E9, Hi: 0x1F6E9, Stride: 1}, {Lo: 0x1F6EB, Hi: 0x1F6F0, Stride: 1}, {Lo: 0x1F6F3, Hi: 0x1F6FF, Stride: 1},
		{Lo: 0x1F7DA, Hi: 0x1F7FF, Stride: 1}, {Lo: 0x1F80C, Hi: 0x1F80F, Stride: 1}, {Lo: 0x1F848, Hi: 0x1F84F, Stride: 1},
		{Lo: 0x1F85A, Hi: 0x1F85F, Stride: 1}, {Lo: 0x1F888, Hi: 0x1F88F, Stride: 1}, {Lo: 0x1F8AE, Hi: 0x1F8AF, Stride: 1},
		{Lo: 0x1F8BC, Hi: 0x1F8BF, Stride: 1}, {Lo: 0x1F8C2, Hi: 0x1F8CF, Stride: 1}, {Lo: 0x1F8D9, Hi: 0x1F8FF, Stride: 1},
		{Lo: 0x1F90C, Hi: 0x1F93A, Stride: 1}, {Lo: 0x1F93C, Hi: 0x1F945, Stride: 1}, {Lo: 0x1F947, Hi: 0x1F9FF, Stride: 1},
		{Lo: 0x1FA58, Hi: 0x1FA5F, Stride: 1}, {Lo: 0x1FA6E, Hi: 0x1FAFF, Stride: 1}, {Lo: 0x1FC00, Hi: 0x1FFFD, Stride: 1},
	},
	LatinOffset: 2,
}

// textDefault are the emoji that show as text by default, so that a
// presentation selector after them selects how they show: the bases of the
// emoji variation sequences of Unicode 17.0 (emoji-variation-sequences.txt)
// that have Emoji_Presentation=No, without the keycap bases '0' to '9', '#'
// and '*'. These are exactly the Extended_Pictographic emoji with
// Emoji_Presentation=No, such as U+00A9 and U+2764.
var textDefault = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x00A9, Hi: 0x00A9, Stride: 1}, {Lo: 0x00AE, Hi: 0x00AE, Stride: 1}, {Lo: 0x203C, Hi: 0x203C, Stride: 1},
		{Lo: 0x2049, Hi: 0x2049, Stride: 1}, {Lo: 0x2122, Hi: 0x2122, Stride: 1}, {Lo: 0x2139, Hi: 0x2139, Stride: 1},
		{Lo: 0x2194, Hi: 0x2199, Stride: 1}, {Lo: 0x21A9, Hi: 0x21AA, Stride: 1}, {Lo: 0x2328, Hi: 0x2328, Stride: 1},
		{Lo: 0x23CF, Hi: 0x23CF, Stride: 1}, {Lo: 0x23ED, Hi: 0x23EF, Stride: 1}, {Lo: 0x23F1, Hi: 0x23F2, Stride: 1},
		{Lo: 0x23F8, Hi: 0x23FA, Stride: 1}, {Lo: 0x24C2, Hi: 0x24C2, Stride: 1}, {Lo: 0x25AA, Hi: 0x25AB, Stride: 1},
		{Lo: 0x25B6, Hi: 0x25B6, Stride: 1}, {Lo: 0x25C0, Hi: 0x25C0, Stride: 1}, {Lo: 0x25FB, Hi: 0x25FC, Stride: 1},
		{Lo: 0x2600, Hi: 0x2604, Stride: 1}, {Lo: 0x260E, Hi: 0x260E, Stride: 1}, {Lo: 0x2611, Hi: 0x2611, Stride: 1},
		{Lo: 0x2618, Hi: 0x2618, Stride: 1}, {Lo: 0x261D, Hi: 0x261D, Stride: 1}, {Lo: 0x2620, Hi: 0x2620, Stride: 1},
		{Lo: 0x2622, Hi: 0x2623, Stride: 1}, {Lo: 0x2626, Hi: 0x2626, Stride: 1}, {Lo: 0x262A, Hi: 0x262A, Stride: 1},
		{Lo: 0x262E, Hi: 0x262F, Stride: 1}, {Lo: 0x2638, Hi: 0x263A, Stride: 1}, {Lo: 0x2640, Hi: 0x2640, Stride: 1},
		{Lo: 0x2642, Hi: 0x2642, Stride: 1}, {Lo: 0x265F, Hi: 0x2660, Stride: 1}, {Lo: 0x2663, Hi: 0x2663, Stride: 1},
		{Lo: 0x2665, Hi: 0x2666, Stride: 1}, {Lo: 0x2668, Hi: 0x2668, Stride: 1}, {Lo: 0x267B, Hi: 0x267B, Stride: 1},
		{Lo: 0x267E, Hi: 0x267E, Stride: 1}, {Lo: 0x2692, Hi: 0x2692, Stride: 1}, {Lo: 0x2694, Hi: 0x2697, Stride: 1},
		{Lo: 0x2699, Hi: 0x2699, Stride: 1}, {Lo: 0x269B, Hi: 0x269C, Stride: 1}, {Lo: 0x26A0, Hi: 0x26A0, Stride: 1},
		{Lo: 0x26A7, Hi: 0x26A7, Stride: 1}, {Lo: 0x26B0, Hi: 0x26B1, Stride: 1}, {Lo: 0x26C8, Hi: 0x26C8, Stride: 1},
		{Lo: 0x26CF, Hi: 0x26CF, Stride: 1}, {Lo: 0x26D1, Hi: 0x26D1, Stride: 1}, {Lo: 0x26D3, Hi: 0x26D3, Stride: 1},
		{Lo: 0x26E9, Hi: 0x26E9, Stride: 1}, {Lo: 0x26F0, Hi: 0x26F1, Stride: 1}, {Lo: 0x26F4, Hi: 0x26F4, Stride: 1},
		{Lo: 0x26F7, Hi: 0x26F9, Stride: 1}, {Lo: 0x2702, Hi: 0x2702, Stride: 1}, {Lo: 0x2708, Hi: 0x2709, Stride: 1},
		{Lo: 0x270C, Hi: 0x270D, Stride: 1}, {Lo: 0x270F, Hi: 0x270F, Stride: 1}, {Lo: 0x2712, Hi: 0x2712, Stride: 1},
		{Lo: 0x2714, Hi: 0x2714, Stride: 1}, {Lo: 0x2716, Hi: 0x2716, Stride: 1}, {Lo: 0x271D, Hi: 0x271D, Stride: 1},
		{Lo: 0x2721, Hi: 0x2721, Stride: 1}, {Lo: 0x2733, Hi: 0x2734, Stride: 1}, {Lo: 0x2744, Hi: 0x2744, Stride: 1},
		{Lo: 0x2747, Hi: 0x2747, Stride: 1}, {Lo: 0x2763, Hi: 0x2764, Stride: 1}, {Lo: 0x27A1, Hi: 0x27A1, Stride: 1},
		{Lo: 0x2934, Hi: 0x2935, Stride: 1}, {Lo: 0x2B05, Hi: 0x2B07, Stride: 1}, {Lo: 0x3030, Hi: 0x3030, Stride: 1},
		{Lo: 0x303D, Hi: 0x303D, Stride: 1}, {Lo: 0x3297, Hi: 0x3297, Stride: 1}, {Lo: 0x3299, Hi: 0x3299, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1F170, Hi: 0x1F171, Stride: 1}, {Lo: 0x1F17E, Hi: 0x1F17F, Stride: 1}, {Lo: 0x1F202, Hi: 0x1F202, Stride: 1},
		{Lo: 0x1F237, Hi: 0x1F237, Stride: 1}, {Lo: 0x1F321, Hi: 0x1F321, Stride: 1}, {Lo: 0x1F324, Hi: 0x1F32C, Stride: 1},
		{Lo: 0x1F336, Hi: 0x1F336, Stride: 1}, {Lo: 0x1F37D, Hi: 0x1F37D, Stride: 1}, {Lo: 0x1F396, Hi: 0x1F397, Stride: 1},
		{Lo: 0x1F399, Hi: 0x1F39B, Stride: 1}, {Lo: 0x1F39E, Hi: 0x1F39F, Stride: 1}, {Lo: 0x1F3CB, Hi: 0x1F3CE, Stride: 1},
		{Lo: 0x1F3D4, Hi: 0x1F3DF, Stride: 1}, {Lo: 0x1F3F3, Hi: 0x1F3F3, Stride: 1}, {Lo: 0x1F3F5, Hi: 0x1F3F5, Stride: 1},
		{Lo: 0x1F3F7, Hi: 0x1F3F7, Stride: 1}, {Lo: 0x1F43F, Hi: 0x1F43F, Stride: 1}, {Lo: 0x1F441, Hi: 0x1F441, Stride: 1},
		{Lo: 0x1F4FD, Hi: 0x1F4FD, Stride: 1}, {Lo: 0x1F549, Hi: 0x1F54A, Stride: 1}, {Lo: 0x1F56F, Hi: 0x1F570, Stride: 1},
		{Lo: 0x1F573, Hi: 0x1F579, Stride: 1}, {Lo: 0x1F587, Hi: 0x1F587, Stride: 1}, {Lo: 0x1F58A, Hi: 0x1F58D, Stride: 1},
		{Lo: 0x1F590, Hi: 0x1F590, Stride: 1}, {Lo: 0x1F5A5, Hi: 0x1F5A5, Stride: 1}, {Lo: 0x1F5A8, Hi: 0x1F5A8, Stride: 1},
		{Lo: 0x1F5B1, Hi: 0x1F5B2, Stride: 1}, {Lo: 0x1F5BC, Hi: 0x1F5BC, Stride: 1}, {Lo: 0x1F5C2, Hi: 0x1F5C4, Stride: 1},
		{Lo: 0x1F5D1, Hi: 0x1F5D3, Stride: 1}, {Lo: 0x1F5DC, Hi: 0x1F5DE, Stride: 1}, {Lo: 0x1F5E1, Hi: 0x1F5E1, Stride: 1},
		{Lo: 0x1F5E3, Hi: 0x1F5E3, Stride: 1}, {Lo: 0x1F5E8, Hi: 0x1F5E8, Stride: 1}, {Lo: 0x1F5EF, Hi: 0x1F5EF, Stride: 1},
		{Lo: 0x1F5F3, Hi: 0x1F5F3, Stride: 1}, {Lo: 0x1F5FA, Hi: 0x1F5FA, Stride: 1}, {Lo: 0x1F6CB, Hi: 0x1F6CB, Stride: 1},
		{Lo: 0x1F6CD, Hi: 0x1F6CF, Stride: 1}, {Lo: 0x1F6E0, Hi: 0x1F6E5, Stride: 1}, {Lo: 0x1F6E9, Hi: 0x1F6E9, Stride: 1},
		{Lo: 0x1F6F0, Hi: 0x1F6F0, Stride: 1}, {Lo: 0x1F6F3, Hi: 0x1F6F3, Stride: 1},
	},
	LatinOffset: 2,
}

// emojiDefaultText are the emoji that show as emoji by default but have a
// text form, so that the text presentation selector U+FE0E after them
// selects how they show: the bases of the emoji variation sequences of
// Unicode 17.0 that have Emoji_Presentation=Yes, such as U+231A.
var emojiDefaultText = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x231A, Hi: 0x231B, Stride: 1}, {Lo: 0x23E9, Hi: 0x23EC, Stride: 1}, {Lo: 0x23F0, Hi: 0x23F0, Stride: 1},
		{Lo: 0x23F3, Hi: 0x23F3, Stride: 1}, {Lo: 0x25FD, Hi: 0x25FE, Stride: 1}, {Lo: 0x2614, Hi: 0x2615, Stride: 1},
		{Lo: 0x2648, Hi: 0x2653, Stride: 1}, {Lo: 0x267F, Hi: 0x267F, Stride: 1}, {Lo: 0x2693, Hi: 0x2693, Stride: 1},
		{Lo: 0x26A1, Hi: 0x26A1, Stride: 1}, {Lo: 0x26AA, Hi: 0x26AB, Stride: 1}, {Lo: 0x26BD, Hi: 0x26BE, Stride: 1},
		{Lo: 0x26C4, Hi: 0x26C5, Stride: 1}, {Lo: 0x26CE, Hi: 0x26CE, Stride: 1}, {Lo: 0x26D4, Hi: 0x26D4, Stride: 1},
		{Lo: 0x26EA, Hi: 0x26EA, Stride: 1}, {Lo: 0x26F2, Hi: 0x26F3, Stride: 1}, {Lo: 0x26F5, Hi: 0x26F5, Stride: 1},
		{Lo: 0x26FA, Hi: 0x26FA, Stride: 1}, {Lo: 0x26FD, Hi: 0x26FD, Stride: 1}, {Lo: 0x2705, Hi: 0x2705, Stride: 1},
		{Lo: 0x270A, Hi: 0x270B, Stride: 1}, {Lo: 0x2728, Hi: 0x2728, Stride: 1}, {Lo: 0x274C, Hi: 0x274C, Stride: 1},
		{Lo: 0x274E, Hi: 0x274E, Stride: 1}, {Lo: 0x2753, Hi: 0x2755, Stride: 1}, {Lo: 0x2757, Hi: 0x2757, Stride: 1},
		{Lo: 0x2795, Hi: 0x2797, Stride: 1}, {Lo: 0x27B0, Hi: 0x27B0, Stride: 1}, {Lo: 0x27BF, Hi: 0x27BF, Stride: 1},
		{Lo: 0x2B1B, Hi: 0x2B1C, Stride: 1}, {Lo: 0x2B50, Hi: 0x2B50, Stride: 1}, {Lo: 0x2B55, Hi: 0x2B55, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1F004, Hi: 0x1F004, Stride: 1}, {Lo: 0x1F21A, Hi: 0x1F21A, Stride: 1}, {Lo: 0x1F22F, Hi: 0x1F22F, Stride: 1},
		{Lo: 0x1F30D, Hi: 0x1F30F, Stride: 1}, {Lo: 0x1F315, Hi: 0x1F315, Stride: 1}, {Lo: 0x1F31C, Hi: 0x1F31C, Stride: 1},
		{Lo: 0x1F378, Hi: 0x1F378, Stride: 1}, {Lo: 0x1F393, Hi: 0x1F393, Stride: 1}, {Lo: 0x1F3A7, Hi: 0x1F3A7, Stride: 1},
		{Lo: 0x1F3AC, Hi: 0x1F3AE, Stride: 1}, {Lo: 0x1F3C2, Hi: 0x1F3C2, Stride: 1}, {Lo: 0x1F3C4, Hi: 0x1F3C4, Stride: 1},
		{Lo: 0x1F3C6, Hi: 0x1F3C6, Stride: 1}, {Lo: 0x1F3CA, Hi: 0x1F3CA, Stride: 1}, {Lo: 0x1F3E0, Hi: 0x1F3E0, Stride: 1},
		{Lo: 0x1F3ED, Hi: 0x1F3ED, Stride: 1}, {Lo: 0x1F408, Hi: 0x1F408, Stride: 1}, {Lo: 0x1F415, Hi: 0x1F415, Stride: 1},
		{Lo: 0x1F41F, Hi: 0x1F41F, Stride: 1}, {Lo: 0x1F426, Hi: 0x1F426, Stride: 1}, {Lo: 0x1F442, Hi: 0x1F442, Stride: 1},
		{Lo: 0x1F446, Hi: 0x1F449, Stride: 1}, {Lo: 0x1F44D, Hi: 0x1F44E, Stride: 1}, {Lo: 0x1F453, Hi: 0x1F453, Stride: 1},
		{Lo: 0x1F46A, Hi: 0x1F46A, Stride: 1}, {Lo: 0x1F47D, Hi: 0x1F47D, Stride: 1}, {Lo: 0x1F4A3, Hi: 0x1F4A3, Stride: 1},
		{Lo: 0x1F4B0, Hi: 0x1F4B0, Stride: 1}, {Lo: 0x1F4B3, Hi: 0x1F4B3, Stride: 1}, {Lo: 0x1F4BB, Hi: 0x1F4BB, Stride: 1},
		{Lo: 0x1F4BF, Hi: 0x1F4BF, Stride: 1}, {Lo: 0x1F4CB, Hi: 0x1F4CB, Stride: 1}, {Lo: 0x1F4DA, Hi: 0x1F4DA, Stride: 1},
		{Lo: 0x1F4DF, Hi: 0x1F4DF, Stride: 1}, {Lo: 0x1F4E4, Hi: 0x1F4E6, Stride: 1}, {Lo: 0x1F4EA, Hi: 0x1F4ED, Stride: 1},
		{Lo: 0x1F4F7, Hi: 0x1F4F7, Stride: 1}, {Lo: 0x1F4F9, Hi: 0x1F4FB, Stride: 1}, {Lo: 0x1F508, Hi: 0x1F508, Stride: 1},
		{Lo: 0x1F50D, Hi: 0x1F50D, Stride: 1}, {Lo: 0x1F512, Hi: 0x1F513, Stride: 1}, {Lo: 0x1F550, Hi: 0x1F567, Stride: 1},
		{Lo: 0x1F610, Hi: 0x1F610, Stride: 1}, {Lo: 0x1F687, Hi: 0x1F687, Stride: 1}, {Lo: 0x1F68D, Hi: 0x1F68D, Stride: 1},
		{Lo: 0x1F691, Hi: 0x1F691, Stride: 1}, {Lo: 0x1F694, Hi: 0x1F694, Stride: 1}, {Lo: 0x1F698, Hi: 0x1F698, Stride: 1},
		{Lo: 0x1F6AD, Hi: 0x1F6AD, Stride: 1}, {Lo: 0x1F6B2, Hi: 0x1F6B2, Stride: 1}, {Lo: 0x1F6B9, Hi: 0x1F6BA, Stride: 1},
		{Lo: 0x1F6BC, Hi: 0x1F6BC, Stride: 1},
	},
}
