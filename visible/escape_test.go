package visible_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/visible"
	"github.com/protolambda/webui/webuitest"
)

// bs writes a backslash as "~", so that the escapes in the expected output
// read as they show.
var bs = strings.NewReplacer("~", `\`).Replace

// Tag characters of the recommended emoji tag sequences, without the base
// U+1F3F4: England, Scotland and Wales.
const (
	england  = "\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F"
	scotland = "\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F"
	wales    = "\U000E0067\U000E0062\U000E0077\U000E006C\U000E0073\U000E007F"
)

type escapeCase struct {
	in, out string
	escaped bool
}

// escapeMode is one of the escape functions.
type escapeMode int

const (
	modeStrict escapeMode = iota // Escape
	modeText                     // EscapeText
	modeProse                    // EscapeProse
)

var modes = []escapeMode{modeStrict, modeText, modeProse}

func (m escapeMode) String() string {
	return [...]string{"Escape", "EscapeText", "EscapeProse"}[m]
}

func (m escapeMode) escape(s string) (string, bool) {
	switch m {
	case modeText:
		return visible.EscapeText(s)
	case modeProse:
		return visible.EscapeProse(s)
	}
	return visible.Escape(s)
}

func checkCases(t devtest.T, m escapeMode, cases []escapeCase) {
	for _, c := range cases {
		out, escaped := m.escape(c.in)
		t.Mustf(be.Equal(c.out, out), "%s(%+q)", m, c.in)
		t.Mustf(be.Equal(c.escaped, escaped), "%s(%+q) escaped", m, c.in)
		t.Mustf(be.NoError(checkEscape(c.in, m)), "%s(%+q) properties", m, c.in)
	}
}

func TestEscape(gt *testing.T) {
	t := devtest.ParallelT(gt)
	checkCases(t, modeStrict, []escapeCase{
		{"", "", false},
		{"Quarterly report", "Quarterly report", false},
		{"<script>alert(1)</script>", "<script>alert(1)</script>", false},
		{`a\b`, `a\b`, false},
		{`not \u202E an escape`, `not \u202E an escape`, false},
		// Letters, marks, symbols and emoji of any script show as themselves.
		{"Gr\u00FC\u00DFe, \u4E16\u754C", "Gr\u00FC\u00DFe, \u4E16\u754C", false},
		{"\u0645\u0631\u062D\u0628\u0627", "\u0645\u0631\u062D\u0628\u0627", false}, // right-to-left letters
		{"cafe\u0301", "cafe\u0301", false},                                         // a combining accent
		{"a \u0301", "a \u0301", false},                                             // an accent on a space
		{"\U0001F600 smile", "\U0001F600 smile", false},
		{"\U0001F1F3\U0001F1F1", "\U0001F1F3\U0001F1F1", false}, // a flag of regional indicators
		{"\U0001F44D\U0001F3FD", "\U0001F44D\U0001F3FD", false}, // a skin tone
		{"\uFFFD", "\uFFFD", false},                             // the replacement character itself
		{"\U0001F3F4", "\U0001F3F4", false},
		// Control characters.
		{"line\nbreak\ttab\rreturn", bs("line~nbreak~ttab~rreturn"), true},
		{"nul\x00", bs("nul~u0000"), true},
		{"bell\a", bs("bell~u0007"), true},
		{"del\x7F", bs("del~u007F"), true},
		{"next\u0085line", bs("next~u0085line"), true},
		// Direction controls.
		{"invoice\u202Efdp.exe", bs("invoice~u202Efdp.exe"), true},
		{"\u2066isolated\u2069", bs("~u2066isolated~u2069"), true},
		{"a\u200Eb\u200Fc\u061Cd\u202Ae\u202Bf\u202Cg\u202Dh", bs("a~u200Eb~u200Fc~u061Cd~u202Ae~u202Bf~u202Cg~u202Dh"), true},
		// Other format characters.
		{"ad\u200Bmin", bs("ad~u200Bmin"), true},
		{"a\u200Cb\u200Dc\u2060d\uFEFFe", bs("a~u200Cb~u200Dc~u2060d~uFEFFe"), true},
		{"soft\u00ADhyphen", bs("soft~u00ADhyphen"), true},
		{"tag\U000E0041", bs("tag~U000E0041"), true},
		{"hi\U000E0001\U000E0068\U000E0069", bs("hi~U000E0001~U000E0068~U000E0069"), true},
		// Printable characters that show as nothing or as a blank.
		{"exa\u034Fmple", bs("exa~u034Fmple"), true},                 // combining grapheme joiner
		{"x\u3164\u3164y", bs("x~u3164~u3164y"), true},               // Hangul filler
		{"x\u115F\u1160\uFFA0", bs("x~u115F~u1160~uFFA0"), true},     // Hangul choseong, jungseong and halfwidth fillers
		{"x\u17B4", bs("x~u17B4"), true},                             // Khmer inherent vowel
		{"blank\u2800", bs("blank~u2800"), true},                     // braille pattern blank
		{"Ada\U0001D159Lovelace", bs("Ada~U0001D159Lovelace"), true}, // musical symbol null notehead
		{"admin\U0001D159", bs("admin~U0001D159"), true},
		// Variation selectors, except presentation selectors that select how
		// an emoji shows, and U+FE0F in keycaps.
		{"ok\uFE0F", bs("ok~uFE0F"), true},
		{"x\U000E0100", bs("x~U000E0100"), true},
		{"x\u180B", bs("x~u180B"), true},
		{"\uFE0F", bs("~uFE0F"), true},
		{"\u2764\uFE0F thanks", "\u2764\uFE0F thanks", false}, // text by default: both selectors select
		{"\u2764\uFE0E thanks", "\u2764\uFE0E thanks", false},
		{"\u00A9\uFE0F", "\u00A9\uFE0F", false},
		{"\u263A\uFE0F \u263A\uFE0E", "\u263A\uFE0F \u263A\uFE0E", false},
		{"\u231A\uFE0E", "\u231A\uFE0E", false},         // emoji by default, with a text form
		{"\u231A\uFE0F", bs("\u231A~uFE0F"), true},      // emoji anyway
		{"\U0001F004\uFE0E", "\U0001F004\uFE0E", false}, // the mahjong tile that is an emoji
		{"Ada\U0001F600\uFE0F", bs("Ada\U0001F600~uFE0F"), true},
		{"Ada\U0001F600\uFE0E", bs("Ada\U0001F600~uFE0E"), true}, // no text form
		{"x\u2605\uFE0F", bs("x\u2605~uFE0F"), true},             // not an emoji
		{"x\U0001F000\uFE0E", bs("x\U0001F000~uFE0E"), true},
		{"\U0001F469\U0001F3FD\uFE0F", bs("\U0001F469\U0001F3FD~uFE0F"), true},
		{"\u2764\uFE0F\uFE0F", bs("\u2764\uFE0F~uFE0F"), true}, // a second one shows as nothing
		{"\xFF\uFE0F", bs("~xFF~uFE0F"), true},
		{"\U0001FC00\uFE0F", bs("~U0001FC00~uFE0F"), true}, // reserved for emoji, not assigned
		{"1\uFE0F\u20E3 #\uFE0F\u20E3 *\uFE0F\u20E3", "1\uFE0F\u20E3 #\uFE0F\u20E3 *\uFE0F\u20E3", false},
		{"1\uFE0F", bs("1~uFE0F"), true},
		{"1\uFE0E\u20E3", bs("1~uFE0E~u20E3"), true},
		{"#\uFE0E", bs("#~uFE0E"), true},
		// Combining marks with nothing to combine with.
		{"\u0301start", bs("~u0301start"), true},
		{"\u200B\u0301", bs("~u200B~u0301"), true},
		{"\xFF\u0301", bs("~xFF~u0301"), true},
		// Spaces.
		{"a b c", "a b c", false},
		{"two  spaces", bs("two~x20~x20spaces"), true},
		{" lead", bs("~x20lead"), true},
		{"trail ", bs("trail~x20"), true},
		{" ", bs("~x20"), true},
		{"   ", bs("~x20~x20~x20"), true},
		{"no\u00A0break", bs("no~u00A0break"), true},
		{"narrow\u202Fbreak", bs("narrow~u202Fbreak"), true},
		{"wide\u3000space", bs("wide~u3000space"), true},
		{"em\u2003space", bs("em~u2003space"), true},
		{"line\u2028para\u2029", bs("line~u2028para~u2029"), true},
		// Invalid UTF-8, byte by byte.
		{"bad\xFFbyte", bs("bad~xFFbyte"), true},
		{"bad \xFF byte", bs("bad ~xFF byte"), true},
		{"\xED\xA0\x80", bs("~xED~xA0~x80"), true},         // a surrogate
		{"\xC0\x80", bs("~xC0~x80"), true},                 // an overlong NUL
		{"\xF4\x90\x80\x80", bs("~xF4~x90~x80~x80"), true}, // beyond U+10FFFF
		{"cut\xE2\x80", bs("cut~xE2~x80"), true},
		// Private use, unassigned and noncharacters.
		{"x\uE000", bs("x~uE000"), true},
		{"\U000F0000", bs("~U000F0000"), true},
		{"x\uFFFF", bs("x~uFFFF"), true},
		{"x\u0378", bs("x~u0378"), true},
		// Once anything is escaped, backslashes are too: a backslash cannot
		// forge an escape.
		{"fake \\u202E real \u202E", bs("fake ~~u202E real ~u202E"), true},
		// Characters that look like a backslash show as themselves (see the
		// package documentation).
		{"fake \u29F5u202E real \u202E", bs("fake \u29F5u202E real ~u202E"), true},
		{"a\u20E5u202E \u202E", bs("a\u20E5u202E ~u202E"), true},
		{"\u2216 \u29F9 \uFE68 \uFF3C", "\u2216 \u29F9 \uFE68 \uFF3C", false},
		{`a  b \`, bs("a~x20~x20b ~~"), true},
		{`\x20 `, bs("~~x20~x20"), true},
		// What EscapeText keeps, Escape escapes.
		{"\U0001F469\u200D\U0001F4BB", bs("\U0001F469~u200D\U0001F4BB"), true},
		{"Ada\u00A0Lovelace", bs("Ada~u00A0Lovelace"), true},
		{"\u0645\u06CC\u200C\u062E", bs("\u0645\u06CC~u200C\u062E"), true},
		{"\U0001F3F4" + england, bs("\U0001F3F4~U000E0067~U000E0062~U000E0065~U000E006E~U000E0067~U000E007F"), true},
	})
}

func TestEscapeText(gt *testing.T) {
	t := devtest.ParallelT(gt)
	checkCases(t, modeText, []escapeCase{
		{"", "", false},
		{"Quarterly report", "Quarterly report", false},
		// Presentation selectors after any emoji, also where they change nothing.
		{"Ada\U0001F600\uFE0F", "Ada\U0001F600\uFE0F", false},
		{"Ada\U0001F600\uFE0E", "Ada\U0001F600\uFE0E", false},
		{"\u231A\uFE0F \u2764\uFE0E", "\u231A\uFE0F \u2764\uFE0E", false},
		{"x\u2605\uFE0F", bs("x\u2605~uFE0F"), true}, // not an emoji
		{"x\U0001F000\uFE0E", bs("x\U0001F000~uFE0E"), true},
		{"#\uFE0F", bs("#~uFE0F"), true},
		{"\U0001F600\uFE0F\uFE0F", bs("\U0001F600\uFE0F~uFE0F"), true},
		// Emoji joined into one.
		{"\U0001F469\u200D\U0001F4BB dev", "\U0001F469\u200D\U0001F4BB dev", false},                         // technologist
		{"\U0001F469\U0001F3FD\u200D\U0001F4BB", "\U0001F469\U0001F3FD\u200D\U0001F4BB", false},             // with a skin tone
		{"\U0001F3F3\uFE0F\u200D\U0001F308", "\U0001F3F3\uFE0F\u200D\U0001F308", false},                     // rainbow flag
		{"\u2764\uFE0F\u200D\U0001F525", "\u2764\uFE0F\u200D\U0001F525", false},                             // heart on fire
		{"\U0001F468\u200D\U0001F469\u200D\U0001F467", "\U0001F468\u200D\U0001F469\u200D\U0001F467", false}, // family
		// The recommended tag sequences.
		{"\U0001F3F4" + england, "\U0001F3F4" + england, false},
		{"go \U0001F3F4" + scotland + "\U0001F3F4" + wales, "go \U0001F3F4" + scotland + "\U0001F3F4" + wales, false},
		{"\U0001F3F4\U000E0067\U000E0062\U000E0074\U000E0078\U000E007F", bs("\U0001F3F4~U000E0067~U000E0062~U000E0074~U000E0078~U000E007F"), true},
		{"\U0001F3F4" + england[:len(england)-4], bs("\U0001F3F4~U000E0067~U000E0062~U000E0065~U000E006E~U000E0067"), true},
		{"x" + england, bs("x~U000E0067~U000E0062~U000E0065~U000E006E~U000E0067~U000E007F"), true},
		// No-break spaces between characters that show.
		{"10\u00A0km", "10\u00A0km", false},
		{"3:04\u202FPM", "3:04\u202FPM", false},
		{"Ada\u00A0Lovelace", "Ada\u00A0Lovelace", false},
		{"\u2764\uFE0F\u00A0x", "\u2764\uFE0F\u00A0x", false},
		{"a\u00A0\u0301", "a\u00A0\u0301", false},
		// Joiners in the scripts that are written with them.
		{"\u0645\u06CC\u200C\u062E\u0648\u0627\u0647\u0645", "\u0645\u06CC\u200C\u062E\u0648\u0627\u0647\u0645", false}, // Persian
		{"\u0915\u094D\u200D\u0937", "\u0915\u094D\u200D\u0937", false},                                                 // Devanagari half form
		{"\u0915\u094D\u200C\u0937", "\u0915\u094D\u200C\u0937", false},
		{"\u0D23\u0D4D\u200D", bs("\u0D23\u0D4D~u200D"), true}, // nothing after it
		// Joiners elsewhere stay escaped.
		{"ad\u200Dmin", bs("ad~u200Dmin"), true},
		{"ad\u200Cmin", bs("ad~u200Cmin"), true},
		{"\U0001F468\u200D", bs("\U0001F468~u200D"), true},
		{"\u200D\U0001F468", bs("~u200D\U0001F468"), true},
		{"\U0001F468\u200D\u200D\U0001F469", bs("\U0001F468~u200D~u200D\U0001F469"), true},
		{"a\u200D\U0001F469", bs("a~u200D\U0001F469"), true},
		{"\U0001F468\u200Da", bs("\U0001F468~u200Da"), true},
		{"\xFF\u200D\U0001F469", bs("~xFF~u200D\U0001F469"), true},
		{"\U0001FC00\u200D\U0001F469", bs("~U0001FC00~u200D\U0001F469"), true},
		{"\u0645\u200C a", bs("\u0645~u200C a"), true},
		{"\u0645\u200Cx", bs("\u0645~u200Cx"), true},
		{"\u0645\u200C\u0915", bs("\u0645~u200C\u0915"), true},
		{"\u1820\u200D\u180B", bs("\u1820~u200D~u180B"), true}, // a variation selector after it
		// No-break spaces at the edges, next to spaces or to each other.
		{"\u00A0x", bs("~u00A0x"), true},
		{"x\u00A0", bs("x~u00A0"), true},
		{"a \u00A0b", bs("a ~u00A0b"), true},
		{"a\u00A0 b", bs("a~u00A0 b"), true},
		{"a\u00A0\u00A0b", bs("a~u00A0~u00A0b"), true},
		{"a\u00A0\u200Bb", bs("a~u00A0~u200Bb"), true},
		{"a\u00A0\xFF", bs("a~u00A0~xFF"), true},
		{"x\u2007y", bs("x~u2007y"), true}, // other spaces stay escaped
		// Everything else stays escaped.
		{"invoice\u202Efdp.exe", bs("invoice~u202Efdp.exe"), true},
		{"invoice\u202Efdp.exe \U0001F469\u200D\U0001F4BB", bs("invoice~u202Efdp.exe \U0001F469\u200D\U0001F4BB"), true},
		{"a\u2066b\u2069", bs("a~u2066b~u2069"), true},
		{"zero\u200Bwidth", bs("zero~u200Bwidth"), true},
		{"tag\U000E0041", bs("tag~U000E0041"), true},
		{"line\nbreak", bs("line~nbreak"), true},
		{"two  spaces", bs("two~x20~x20spaces"), true},
		{"ok\uFE0F", bs("ok~uFE0F"), true},
		{"Ada\U0001D159Lovelace", bs("Ada~U0001D159Lovelace"), true},
		{"fake \u29F5u202E real \u202E", bs("fake \u29F5u202E real ~u202E"), true},
	})
}

func TestEscapeProse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	checkCases(t, modeProse, []escapeCase{
		{"", "", false},
		{"Quarterly report", "Quarterly report", false},
		// Line breaks, spaces and tabs stay as they are.
		{"line\nbreak", "line\nbreak", false},
		{"line\r\nbreak\r\n", "line\r\nbreak\r\n", false},
		{"\n\nblank lines\n\n", "\n\nblank lines\n\n", false},
		{"  indented\n\ttabbed\t\n trailing  ", "  indented\n\ttabbed\t\n trailing  ", false},
		{" \t \n", " \t \n", false},
		{"C:\\notes\nD:\\more", "C:\\notes\nD:\\more", false},
		// What EscapeText keeps, prose keeps.
		{"10\u00A0km\n\U0001F469\u200D\U0001F4BB team", "10\u00A0km\n\U0001F469\u200D\U0001F4BB team", false},
		{"\u0645\u06CC\u200C\u062E\n\u2764\uFE0F", "\u0645\u06CC\u200C\u062E\n\u2764\uFE0F", false},
		{"a \u0301", "a \u0301", false}, // an accent on a single space
		// Other control characters, a carriage return on its own too.
		{"old\rmac", bs("old~rmac"), true},
		{"cr\r\r\nlf", bs("cr~r\r\nlf"), true},
		{"end\r", bs("end~r"), true},
		{"vt\vff\f", bs("vt~u000Bff~u000C"), true},
		{"nul\x00\nnext", bs("nul~u0000\nnext"), true},
		{"next\u0085line", bs("next~u0085line"), true},
		// The line and paragraph separators are no line breaks.
		{"line\u2028para\u2029", bs("line~u2028para~u2029"), true},
		// Direction controls and invisible characters, on one line or
		// several: the backslashes of every line are escaped too.
		{"invoice\u202Efdp.exe\nnext line", bs("invoice~u202Efdp.exe\nnext line"), true},
		{"C:\\notes\nad\u200Bmin", bs("C:~~notes\nad~u200Bmin"), true},
		{"\u2066isolated\n\u2069", bs("~u2066isolated\n~u2069"), true},
		{"zero\u200Bwidth \u2060joiner\uFEFF", bs("zero~u200Bwidth ~u2060joiner~uFEFF"), true},
		{"tag\U000E0041 \u00AD", bs("tag~U000E0041 ~u00AD"), true},
		// Other spaces stay escaped where EscapeText escapes them.
		{"no\u00A0\u00A0break", bs("no~u00A0~u00A0break"), true},
		{"\u00A0lead", bs("~u00A0lead"), true},
		{"a \u00A0b", bs("a ~u00A0b"), true},
		{"a\t\u00A0b", bs("a\t~u00A0b"), true},
		{"a\u00A0\tb", bs("a~u00A0\tb"), true},
		{"a\u00A0\nb", bs("a~u00A0\nb"), true},
		{"a\n\u00A0b", bs("a\n~u00A0b"), true},
		{"em\u2003space ideographic\u3000", bs("em~u2003space ideographic~u3000"), true},
		{"blank\u2800 \u3164", bs("blank~u2800 ~u3164"), true},
		// Characters that show only after another one are escaped at the
		// start of a line, and after white space other than a single
		// space after a character.
		{"\u0301start", bs("~u0301start"), true},
		{"a\n\u0301b", bs("a\n~u0301b"), true},
		{"a\r\n\u0301b", bs("a\r\n~u0301b"), true},
		{"a\t\u0301", bs("a\t~u0301"), true},
		{"a  \u0301", bs("a  ~u0301"), true},
		{" \u0301", bs(" ~u0301"), true},
		{"a\n \u0301", bs("a\n ~u0301"), true},
		{"\u2764\n\uFE0F", bs("\u2764\n~uFE0F"), true},
		{"\U0001F468\n\u200D\U0001F469", bs("\U0001F468\n~u200D\U0001F469"), true},
		{"\U0001F468\u200D\n\U0001F469", bs("\U0001F468~u200D\n\U0001F469"), true},
		{"\u0645 \u200C\u062E", bs("\u0645 ~u200C\u062E"), true},
		// Invalid UTF-8.
		{"bad\xFF\nbyte", bs("bad~xFF\nbyte"), true},
	})
}

func TestRune(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, r := range []rune{'a', 'Z', '0', ' ', '\\', '~', '\u00E9', '\u0301', '\u0645', '\u4E16', '\uFFFD',
		'\U0001F600', '\U0001F3FD', '\U0001F1F3', '\u20E3', '\u00A9', '\u2605', '\u29F5', '\U0001D158',
		'\U0001D15A'} {
		t.Mustf(be.True(visible.Rune(r)), "Rune(%U)", r)
	}
	for _, r := range []rune{-1, '\x00', '\n', '\t', '\x7F', '\u0085', '\u00A0', '\u00AD', '\u034F', '\u061C',
		'\u115F', '\u1160', '\u17B4', '\u180B', '\u180E', '\u200B', '\u200C', '\u200D', '\u200E', '\u2028',
		'\u202E', '\u202F', '\u2060', '\u2066', '\u2800', '\u3000', '\u3164', 0xD800, '\uE000', '\uFE0E',
		'\uFE0F', '\uFEFF', '\uFFA0', '\uFFFF', '\U000E0001', '\U000E0041', '\U000E0100', '\U0001D159', '\U0001FC00',
		unicode.MaxRune, unicode.MaxRune + 1} {
		t.Mustf(be.False(visible.Rune(r)), "Rune(%U)", r)
	}
}

// TestRuneProperties checks every character against the Unicode properties
// of characters that show as nothing, or that change how others show.
func TestRuneProperties(gt *testing.T) {
	t := devtest.ParallelT(gt)
	invisible := []*unicode.RangeTable{
		unicode.Cc, unicode.Cf, unicode.Co, unicode.Cs, unicode.Zl, unicode.Zp,
		unicode.Bidi_Control, unicode.Join_Control, unicode.Other_Default_Ignorable_Code_Point,
		unicode.Variation_Selector, unicode.Noncharacter_Code_Point,
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !visible.Rune(r) {
			continue
		}
		t.Mustf(be.True(unicode.IsGraphic(r)), "Rune(%U) is not graphic", r)
		t.Mustf(be.False(unicode.In(r, invisible...)), "Rune(%U) is invisible", r)
		t.Mustf(be.True(r == ' ' || !unicode.Is(unicode.White_Space, r)), "Rune(%U) is white space", r)
	}
	// No property marks the printable symbols that fonts draw as a blank.
	for _, r := range []rune{'\u2800', '\U0001D159'} {
		t.Mustf(be.False(visible.Rune(r)), "Rune(%U) is a blank", r)
	}
}

// TestEveryRune escapes every character alone and between others, and
// checks the properties of the escaped form (see checkEscape). Of the
// planes that have no assigned characters (4 to 13) and of the private-use
// planes (15 and 16), it checks a sample.
func TestEveryRune(gt *testing.T) {
	t := devtest.ParallelT(gt)
	if testing.Short() {
		t.Skip("escapes every character")
	}
	const chunk = 0x4000
	var starts []rune
	for r := rune(0); r <= unicode.MaxRune; r += chunk {
		starts = append(starts, r)
	}
	devtest.Parallel(t, starts, func(t devtest.T, start rune) {
		for r := start; r < start+chunk; r++ {
			sparse := r >= 0x40000 && r < 0xE0000 || r >= 0xF0000
			if sparse && r%0x101 != 0 || r >= 0xD800 && r <= 0xDFFF {
				continue
			}
			c := string(r)
			for _, s := range []string{c, "a" + c + "b", "\U0001F469" + c + "\U0001F4BB", "\u0645" + c + "\u0645"} {
				for _, m := range []escapeMode{modeStrict, modeText} {
					if err := checkEscape(s, m); err != nil {
						t.Errorf("%U in %+q (%s): %v", r, s, m, err)
					}
				}
			}
			// In prose: between letters, at the start of a line, after a
			// space after a character, and after a tab.
			for _, s := range []string{"a" + c + "b", "a\n" + c + " " + c + "\t" + c} {
				if err := checkEscape(s, modeProse); err != nil {
					t.Errorf("%U in %+q (%s): %v", r, s, modeProse, err)
				}
			}
		}
	})
}

// TestHostile escapes the hostile inputs of webuitest: the misleading
// ones are escaped, the look-alike letter is not.
func TestHostile(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range webuitest.Hostile() {
		for _, m := range modes {
			t.Mustf(be.NoError(checkEscape(s, m)), "%s(%+q)", m, s)
		}
	}
	for _, s := range []string{"\u202Egnp.exe", "ad\u200Bmin", "line\nbreak\r\nand\rcarriage", "tab\tand\x00nul"} {
		for _, m := range modes {
			_, escaped := m.escape(s)
			t.Mustf(be.True(escaped), "%s(%+q)", m, s)
		}
	}
	out, escaped := visible.Escape("Ad\u043Cin") // a Cyrillic letter that looks like a Latin one
	t.Must(be.Equal("Ad\u043Cin", out))
	t.Must(be.False(escaped))
}

// TestEscapeNoAlloc checks that text that shows as itself is returned
// without allocating.
func TestEscapeNoAlloc(gt *testing.T) {
	t := devtest.SerialT(gt)
	s := "Quarterly report \u2764\uFE0F \U0001F469\u200D\U0001F4BB 10\u00A0km"
	allocs := testing.AllocsPerRun(100, func() {
		if _, escaped := visible.EscapeText(s); escaped {
			panic("escaped")
		}
	})
	t.Must(be.Equal(0.0, allocs))
	prose := s + "\r\n  indented\n\n\tand tabbed  "
	allocs = testing.AllocsPerRun(100, func() {
		if _, escaped := visible.EscapeProse(prose); escaped {
			panic("escaped")
		}
	})
	t.Must(be.Equal(0.0, allocs))
}

// TestEscapeLength checks the bound on the length of the escaped form: six
// bytes for a one-byte control character.
func TestEscapeLength(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := strings.Repeat("\x00", 1000)
	out, escaped := visible.Escape(s)
	t.Must(be.True(escaped))
	t.Must(be.Equal(6*len(s), len(out)))
	t.Must(be.Equal(strings.Repeat(`\u0000`, 1000), out))
}

func FuzzEscape(f *testing.F) {
	for _, s := range webuitest.Hostile() {
		f.Add(s)
	}
	for _, s := range []string{
		"", " ", "a  b", "\xFF", `\u202E`, "invoice\u202Efdp.exe", "\u2764\uFE0F\u200D\U0001F525",
		"\U0001F469\U0001F3FD\u200D\U0001F4BB", "\U0001F3F4" + england, "1\uFE0F\u20E3", "a\u00A0b",
		"\u0645\u06CC\u200C\u062E", "\u0915\u094D\u200D\u0937", "\u0301a", "x\u3164", "\U000E0041",
		"two\r\nlines\n\n", " \t\u0301\n\u00A0\r", "a \u0301\n\u2028",
	} {
		f.Add(s)
	}
	f.Fuzz(func(gt *testing.T, s string) {
		t := devtest.SerialT(gt)
		for _, m := range modes {
			t.Must(be.NoError(checkEscape(s, m)))
		}
	})
}

// checkEscape checks the promises of the escape function of mode m for s:
// the escaped form is valid UTF-8, at most six times as long as s, shows as
// itself, has no invisible characters other than those that the mode
// keeps, and decodes back to s; text that is not escaped is s itself, and
// escaped text is not. For prose, see also checkProse.
func checkEscape(s string, m escapeMode) error {
	out, escaped := m.escape(s)
	if !utf8.ValidString(out) {
		return errors.New("escaped form is not valid UTF-8")
	}
	if len(out) > 6*len(s) {
		return fmt.Errorf("escaped form has %d bytes, more than 6 times %d", len(out), len(s))
	}
	if !escaped && out != s {
		return errors.New("not escaped, but changed")
	}
	if escaped && out == s {
		return errors.New("escaped, but unchanged")
	}
	if escaped {
		back, err := unescape(out)
		if err != nil {
			return fmt.Errorf("escaped form %+q does not decode: %w", out, err)
		}
		if back != s {
			return fmt.Errorf("escaped form %+q decodes to %+q", out, back)
		}
	}
	if again, more := m.escape(out); more || again != out {
		return fmt.Errorf("escaped form %+q does not show as itself: %+q", out, again)
	}
	if m == modeProse {
		return checkProse(s, out, escaped)
	}
	lenient := m == modeText
	if again, more := visible.EscapeText(out); more || again != out {
		return fmt.Errorf("escaped form %+q is escaped by EscapeText: %+q", out, again)
	}
	if strings.HasPrefix(out, " ") || strings.HasSuffix(out, " ") || strings.Contains(out, "  ") {
		return fmt.Errorf("escaped form %+q has spaces that collapse", out)
	}
	for i, r := range out {
		switch {
		case visible.Rune(r):
		case r == '\uFE0E' || r == '\uFE0F':
		case lenient && (r == '\u200C' || r == '\u200D' || r == '\u00A0' || r == '\u202F'):
		case lenient && r >= 0xE0020 && r <= 0xE007F:
		default:
			return fmt.Errorf("escaped form %+q has %U at byte %d", out, r, i)
		}
		if unicode.In(r, unicode.Bidi_Control, unicode.Cc) {
			return fmt.Errorf("escaped form %+q has the control %U", out, r)
		}
	}
	return nil
}

// checkProse checks the escaped prose out of s: it has the characters that
// EscapeText keeps, and the white space that prose keeps; prose without
// white space escapes as EscapeText does; prose that EscapeText does not
// escape is not escaped either; and every line of prose that is not
// escaped, with its spaces and tabs collapsed as HTML collapses them, is
// text that EscapeText does not escape.
func checkProse(s, out string, escaped bool) error {
	for i, r := range out {
		switch {
		case visible.Rune(r), r == '\t', r == '\n':
		case r == '\r' && strings.HasPrefix(out[i+1:], "\n"):
		case r == '\uFE0E' || r == '\uFE0F':
		case r == '\u200C' || r == '\u200D' || r == '\u00A0' || r == '\u202F':
		case r >= 0xE0020 && r <= 0xE007F:
		default:
			return fmt.Errorf("escaped prose %+q has %U at byte %d", out, r, i)
		}
		if unicode.Is(unicode.Bidi_Control, r) {
			return fmt.Errorf("escaped prose %+q has the control %U", out, r)
		}
	}
	text, textEscaped := visible.EscapeText(s)
	if !strings.ContainsAny(s, " \t\n\r") && (text != out || textEscaped != escaped) {
		return fmt.Errorf("prose %+q without white space escapes to %+q, by EscapeText to %+q", s, out, text)
	}
	if escaped && !textEscaped {
		return fmt.Errorf("prose %+q is escaped, but not by EscapeText", s)
	}
	if escaped {
		return nil
	}
	for line := range strings.SplitSeq(s, "\n") {
		words := strings.FieldsFunc(strings.TrimSuffix(line, "\r"), func(r rune) bool { return r == ' ' || r == '\t' })
		if _, more := visible.EscapeText(strings.Join(words, " ")); more {
			return fmt.Errorf("line %+q of prose that is not escaped is escaped by EscapeText, collapsed", line)
		}
	}
	return nil
}

// unescape decodes the escaped form of a value (see the package
// documentation), strictly: every backslash starts an escape, hex digits
// are uppercase, and \x is a space or a byte of invalid UTF-8.
func unescape(s string) (string, error) {
	var b []byte
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			b = append(b, s[i])
			i++
			continue
		}
		if i+1 >= len(s) {
			return "", errors.New("backslash at the end")
		}
		digits := 0
		switch s[i+1] {
		case '\\':
			b = append(b, '\\')
		case 'n':
			b = append(b, '\n')
		case 'r':
			b = append(b, '\r')
		case 't':
			b = append(b, '\t')
		case 'x':
			digits = 2
		case 'u':
			digits = 4
		case 'U':
			digits = 8
		default:
			return "", fmt.Errorf("unknown escape at byte %d", i)
		}
		if digits == 0 {
			i += 2
			continue
		}
		hex := s[i+2 : min(len(s), i+2+digits)]
		if len(hex) != digits || strings.ToUpper(hex) != hex {
			return "", fmt.Errorf("bad hex digits at byte %d", i)
		}
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return "", fmt.Errorf("bad hex digits at byte %d: %w", i, err)
		}
		switch {
		case digits == 2 && v != ' ' && v < utf8.RuneSelf:
			return "", fmt.Errorf("\\x escape of a valid byte at byte %d", i)
		case digits == 2:
			b = append(b, byte(v))
		case digits == 8 && v <= 0xFFFF, !utf8.ValidRune(rune(v)):
			return "", fmt.Errorf("bad character escape at byte %d", i)
		default:
			b = utf8.AppendRune(b, rune(v))
		}
		i += 2 + digits
	}
	return string(b), nil
}
