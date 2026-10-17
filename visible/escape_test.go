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

func checkCases(t devtest.T, name string, escape func(string) (string, bool), lenient bool, cases []escapeCase) {
	for _, c := range cases {
		out, escaped := escape(c.in)
		t.Mustf(be.Equal(c.out, out), "%s(%+q)", name, c.in)
		t.Mustf(be.Equal(c.escaped, escaped), "%s(%+q) escaped", name, c.in)
		t.Mustf(be.NoError(checkEscape(c.in, lenient)), "%s(%+q) properties", name, c.in)
	}
}

func TestEscape(gt *testing.T) {
	t := devtest.ParallelT(gt)
	checkCases(t, "Escape", visible.Escape, false, []escapeCase{
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
		{"exa\u034Fmple", bs("exa~u034Fmple"), true},             // combining grapheme joiner
		{"x\u3164\u3164y", bs("x~u3164~u3164y"), true},           // Hangul filler
		{"x\u115F\u1160\uFFA0", bs("x~u115F~u1160~uFFA0"), true}, // Hangul choseong, jungseong and halfwidth fillers
		{"x\u17B4", bs("x~u17B4"), true},                         // Khmer inherent vowel
		{"blank\u2800", bs("blank~u2800"), true},                 // braille pattern blank
		// Variation selectors, except presentation selectors after emoji and in keycaps.
		{"ok\uFE0F", bs("ok~uFE0F"), true},
		{"x\U000E0100", bs("x~U000E0100"), true},
		{"x\u180B", bs("x~u180B"), true},
		{"\uFE0F", bs("~uFE0F"), true},
		{"\u2764\uFE0F thanks", "\u2764\uFE0F thanks", false},
		{"\u2764\uFE0E thanks", "\u2764\uFE0E thanks", false},
		{"\u00A9\uFE0F", "\u00A9\uFE0F", false},
		{"\u2764\uFE0F\uFE0F", bs("\u2764\uFE0F~uFE0F"), true}, // a second one shows as nothing
		{"\xFF\uFE0F", bs("~xFF~uFE0F"), true},
		{"\U0001FC00\uFE0F", bs("~U0001FC00~uFE0F"), true}, // reserved for emoji, not assigned
		{"1\uFE0F\u20E3 #\uFE0F\u20E3 *\uFE0F\u20E3", "1\uFE0F\u20E3 #\uFE0F\u20E3 *\uFE0F\u20E3", false},
		{"1\uFE0F", bs("1~uFE0F"), true},
		{"1\uFE0E\u20E3", bs("1~uFE0E~u20E3"), true},
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
		// Once anything is escaped, backslashes are too: escapes cannot be forged.
		{"fake \\u202E real \u202E", bs("fake ~~u202E real ~u202E"), true},
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
	checkCases(t, "EscapeText", visible.EscapeText, true, []escapeCase{
		{"", "", false},
		{"Quarterly report", "Quarterly report", false},
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
	})
}

func TestRune(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, r := range []rune{'a', 'Z', '0', ' ', '\\', '~', '\u00E9', '\u0301', '\u0645', '\u4E16', '\uFFFD',
		'\U0001F600', '\U0001F3FD', '\U0001F1F3', '\u20E3', '\u00A9'} {
		t.Mustf(be.True(visible.Rune(r)), "Rune(%U)", r)
	}
	for _, r := range []rune{-1, '\x00', '\n', '\t', '\x7F', '\u0085', '\u00A0', '\u00AD', '\u034F', '\u061C',
		'\u115F', '\u1160', '\u17B4', '\u180B', '\u180E', '\u200B', '\u200C', '\u200D', '\u200E', '\u2028',
		'\u202E', '\u202F', '\u2060', '\u2066', '\u2800', '\u3000', '\u3164', 0xD800, '\uE000', '\uFE0E',
		'\uFE0F', '\uFEFF', '\uFFA0', '\uFFFF', '\U000E0001', '\U000E0041', '\U000E0100', '\U0001FC00',
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
				for _, lenient := range []bool{false, true} {
					if err := checkEscape(s, lenient); err != nil {
						t.Errorf("%U in %+q (lenient %t): %v", r, s, lenient, err)
					}
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
		t.Mustf(be.NoError(checkEscape(s, false)), "Escape(%+q)", s)
		t.Mustf(be.NoError(checkEscape(s, true)), "EscapeText(%+q)", s)
	}
	for _, s := range []string{"\u202Egnp.exe", "ad\u200Bmin", "line\nbreak\r\nand\rcarriage", "tab\tand\x00nul"} {
		_, escaped := visible.Escape(s)
		t.Mustf(be.True(escaped), "Escape(%+q)", s)
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
	} {
		f.Add(s)
	}
	f.Fuzz(func(gt *testing.T, s string) {
		t := devtest.SerialT(gt)
		t.Must(be.NoError(checkEscape(s, false)))
		t.Must(be.NoError(checkEscape(s, true)))
	})
}

// checkEscape checks the promises of Escape (or EscapeText if lenient) for
// s: the escaped form is valid UTF-8, at most six times as long as s, shows
// as itself, has no invisible characters other than those that the mode
// keeps, and decodes back to s; and text that is not escaped is s itself.
func checkEscape(s string, lenient bool) error {
	escape := visible.Escape
	if lenient {
		escape = visible.EscapeText
	}
	out, escaped := escape(s)
	if !utf8.ValidString(out) {
		return errors.New("escaped form is not valid UTF-8")
	}
	if len(out) > 6*len(s) {
		return fmt.Errorf("escaped form has %d bytes, more than 6 times %d", len(out), len(s))
	}
	if !escaped && out != s {
		return errors.New("not escaped, but changed")
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
	if again, more := escape(out); more || again != out {
		return fmt.Errorf("escaped form %+q does not show as itself: %+q", out, again)
	}
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
