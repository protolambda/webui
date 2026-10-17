package visible_test

import (
	"errors"
	"fmt"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/table"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/visible"
)

// Names that other users chose render with Text: a name that shows as
// itself in a bdi element, any other escaped, with a marker.
func Example() {
	for _, name := range []string{"Ada Lovelace", "invoice\u202Efdp.exe"} {
		core.Dump(text.P()(text.Text("Shared by "), visible.Text(name)))
	}
	// Output:
	// <p>Shared by <bdi>Ada Lovelace</bdi></p>
	// <p>Shared by <code dir="ltr">invoice\u202Efdp.exe</code> <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span></p>
}

func ExampleEscape() {
	for _, s := range []string{
		"Quarterly report",
		"invoice\u202Efdp.exe", // a right-to-left override: shows as "invoiceexe.pdf"
		"ad\u200Bmin",          // a zero-width space
		"two  spaces",
		" padded ",
		"line\nbreak",
		"bad \xFF byte",
		`C:\notes`, // a backslash, when nothing else is escaped
		"fake \\u202E and real \u202E",
	} {
		out, escaped := visible.Escape(s)
		fmt.Println(escaped, out)
	}
	// Output:
	// false Quarterly report
	// true invoice\u202Efdp.exe
	// true ad\u200Bmin
	// true two\x20\x20spaces
	// true \x20padded\x20
	// true line\nbreak
	// true bad \xFF byte
	// false C:\notes
	// true fake \\u202E and real \u202E
}

// A validator can refuse names that do not show as themselves.
func ExampleEscape_validate() {
	validate := func(name string) error {
		if _, escaped := visible.Escape(name); escaped {
			return errors.New("name has invisible characters or extra spaces")
		}
		return nil
	}
	fmt.Println(validate("Ada Lovelace"))
	fmt.Println(validate("Ada  Lovelace"))
	fmt.Println(validate("Ada\u00A0Lovelace"))
	// Output:
	// <nil>
	// name has invisible characters or extra spaces
	// name has invisible characters or extra spaces
}

// EscapeText keeps the invisible characters of common display text, which
// Escape escapes.
func ExampleEscapeText() {
	for _, c := range []struct{ name, s string }{
		{"emoji with a presentation selector", "I \u2764\uFE0F notes"},
		{"emoji joined into one", "\U0001F469\u200D\U0001F4BB team"},
		{"no-break space", "10\u00A0km"},
		{"Persian word with a non-joiner", "\u0645\u06CC\u200C\u062E\u0648\u0627"},
		{"joiner between Latin letters", "ad\u200Dmin"},
		{"right-to-left override", "invoice\u202Efdp.exe"},
	} {
		_, textEscaped := visible.EscapeText(c.s)
		_, escaped := visible.Escape(c.s)
		fmt.Printf("%s: EscapeText %t, Escape %t\n", c.name, textEscaped, escaped)
	}
	// Output:
	// emoji with a presentation selector: EscapeText false, Escape false
	// emoji joined into one: EscapeText false, Escape true
	// no-break space: EscapeText false, Escape true
	// Persian word with a non-joiner: EscapeText false, Escape true
	// joiner between Latin letters: EscapeText true, Escape true
	// right-to-left override: EscapeText true, Escape true
}

func ExampleRune() {
	for _, r := range []rune{'a', '\u00E9', '\U0001F600', ' ', '\u00A0', '\u200B', '\u202E', '\u3164', '\n'} {
		fmt.Printf("%U %t\n", r, visible.Rune(r))
	}
	// Output:
	// U+0061 true
	// U+00E9 true
	// U+1F600 true
	// U+0020 true
	// U+00A0 false
	// U+200B false
	// U+202E false
	// U+3164 false
	// U+000A false
}

func ExampleText() {
	core.Dump(visible.Text("Team notes"))
	core.Dump(visible.Text("<b>Team</b> notes")) // markup is text, as always
	core.Dump(visible.Text("Team\nnotes"))
	// Output:
	// <bdi>Team notes</bdi>
	// <bdi>&lt;b&gt;Team&lt;/b&gt; notes</bdi>
	// <code dir="ltr">Team\nnotes</code> <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span>
}

// Code shows values that are read character by character.
func ExampleCode() {
	core.Dump(table.TR()(
		table.TD()(visible.Text("Ada Lovelace")),
		table.TD()(visible.Code("ada.lovelace")),
	))
	core.Dump(visible.Code("notes.example\u00A0")) // a trailing no-break space
	// Output:
	// <tr><td><bdi>Ada Lovelace</bdi></td><td><code dir="ltr">ada.lovelace</code></td></tr>
	// <code dir="ltr">notes.example\u00A0</code> <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span>
}

// Marker goes next to escaped text in markup of the application's own.
func ExampleMarker() {
	title := "Weekly\u2028plan"
	v, escaped := visible.EscapeText(title)
	heading := section.H2(attr.Class("h5"), attr.Dir("auto"))(text.Text(v))
	if escaped {
		heading = section.H2(attr.Class("h5"), attr.Dir("ltr"))(text.Text(v), text.Text(" "), visible.Marker())
	}
	core.Dump(heading)
	// Output:
	// <h2 class="h5" dir="ltr">Weekly\u2028plan <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span></h2>
}
