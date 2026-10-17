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
// itself in a bdi element, any other escaped, after a marker.
func Example() {
	for _, name := range []string{"Ada Lovelace", "invoice\u202Efdp.exe"} {
		core.Dump(text.P()(text.Text("Shared by "), visible.Text(name)))
	}
	// Output:
	// <p>Shared by <bdi>Ada Lovelace</bdi></p>
	// <p>Shared by <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span> <code dir="ltr">invoice\u202Efdp.exe</code></p>
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
		"Ada \U0001F680\uFE0F", // an emoji selector after an emoji that shows as emoji anyway
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
	// true Ada 🚀\uFE0F
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
	fmt.Println(validate("Ada \U0001F680\uFE0F"))
	// Output:
	// <nil>
	// name has invisible characters or extra spaces
	// name has invisible characters or extra spaces
	// name has invisible characters or extra spaces
}

// EscapeText keeps the invisible characters of common display text, which
// Escape escapes.
func ExampleEscapeText() {
	for _, c := range []struct{ name, s string }{
		{"emoji with a presentation selector", "I \u2764\uFE0F notes"},
		{"emoji with a selector that changes nothing", "Ada \U0001F680\uFE0F"},
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
	// emoji with a selector that changes nothing: EscapeText false, Escape true
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
	// <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span> <code dir="ltr">Team\nnotes</code>
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
	// <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span> <code dir="ltr">notes.example\u00A0</code>
}

// Marker goes before escaped text in markup of the application's own. A
// [visible.Marking] renders it with other words.
func ExampleMarker() {
	title := "Weekly\u2028plan"
	v, escaped := visible.EscapeText(title)
	heading := section.H2(attr.Class("h5"), attr.Dir("auto"))(text.Text(v))
	if escaped {
		heading = section.H2(attr.Class("h5"), attr.Dir("ltr"))(visible.Marker(), text.Text(" "), text.Text(v))
	}
	core.Dump(heading)
	// Output:
	// <h2 class="h5" dir="ltr"><span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">escaped</span> Weekly\u2028plan</h2>
}

// A Marking renders the marker with the words of the application, such as
// a translation; the text and code isolation stays the same.
func ExampleMarking() {
	de := visible.Marking{Label: "maskiert", Title: "Mit Escapes gezeigt: der Wert hat unsichtbare Zeichen"}
	core.Dump(de.Text("Ada Lovelace"))
	core.Dump(de.Text("ad\u200Bmin"))
	core.Dump(de.Code("notes.example\u00A0"))
	core.Dump(visible.Marking{Label: "maskiert"}.Marker()) // a blank title means MarkerTitle
	// Output:
	// <bdi>Ada Lovelace</bdi>
	// <span class="badge text-bg-warning" title="Mit Escapes gezeigt: der Wert hat unsichtbare Zeichen">maskiert</span> <code dir="ltr">ad\u200Bmin</code>
	// <span class="badge text-bg-warning" title="Mit Escapes gezeigt: der Wert hat unsichtbare Zeichen">maskiert</span> <code dir="ltr">notes.example\u00A0</code>
	// <span class="badge text-bg-warning" title="Shown with escapes: the value has invisible, control or text direction characters, or spaces that would not show">maskiert</span>
}
