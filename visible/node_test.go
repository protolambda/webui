package visible_test

import (
	"context"
	"strings"
	"testing"
	"unicode"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/visible"
	"github.com/protolambda/webui/webuitest"
)

// render renders node without indentation.
func render(t devtest.T, node elem.Node) string {
	var b strings.Builder
	t.Must(be.NoError(core.Render(context.Background(), node, &b)))
	return b.String()
}

const marker = `<span class="badge text-bg-warning" title="` + visible.MarkerTitle + `">escaped</span>`

func TestTextMarkup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct{ in, out string }{
		{"", `<bdi></bdi>`},
		{"Ada Lovelace", `<bdi>Ada Lovelace</bdi>`},
		{"<b>bold</b> & \"quoted\"", `<bdi>&lt;b&gt;bold&lt;/b&gt; &amp; &#34;quoted&#34;</bdi>`},
		{"\u0645\u0631\u062D\u0628\u0627", "<bdi>\u0645\u0631\u062D\u0628\u0627</bdi>"},
		{"10\u00A0km \U0001F469\u200D\U0001F4BB", "<bdi>10\u00A0km \U0001F469\u200D\U0001F4BB</bdi>"},
		{"invoice\u202Efdp.exe", marker + bs(` <code dir="ltr">invoice~u202Efdp.exe</code>`)},
		{"<i>x</i>\u200B", marker + bs(` <code dir="ltr">&lt;i&gt;x&lt;/i&gt;~u200B</code>`)},
	} {
		t.Mustf(be.Equal(c.out, render(t, visible.Text(c.in))), "Text(%+q)", c.in)
	}
}

func TestCodeMarkup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct{ in, out string }{
		{"", `<code dir="ltr"></code>`},
		{"notes.example", `<code dir="ltr">notes.example</code>`},
		{`C:\notes`, `<code dir="ltr">C:\notes</code>`},
		{"https://notes.example/?q=<x>", `<code dir="ltr">https://notes.example/?q=&lt;x&gt;</code>`},
		{"10\u00A0km", marker + bs(` <code dir="ltr">10~u00A0km</code>`)},
		{`C:\notes` + "\n", marker + bs(` <code dir="ltr">C:~~notes~n</code>`)},
	} {
		t.Mustf(be.Equal(c.out, render(t, visible.Code(c.in))), "Code(%+q)", c.in)
	}
}

// TestMarkerView checks the marker as a reader and a test see it.
func TestMarkerView(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(text.P()(text.Text("Owner: "), visible.Text("ad\u200Bmin"), text.Text(".")))
	t.Must(page.Find(ct.Tag("p")).Matches(ct.InnerText(bs("Owner: escaped ad~u200Bmin."))))
	t.Must(page.Find(ct.Tag("code")).Matches(ct.Attr("dir", "ltr"), ct.TextContent(bs("ad~u200Bmin"))))
	t.Must(page.Find(ct.Class("badge")).Matches(ct.Attr("title", visible.MarkerTitle), ct.Text(visible.MarkerText)))
	t.Must(page.Find(ct.Tag("bdi")).None())
	// The marker comes before the value, so that clipping the end keeps it.
	t.Must(page.Find(ct.Tag("p")).Find(ct.Class("badge").Or(ct.Tag("code"))).InOrder(ct.Class("badge"), ct.Tag("code")))

	plain := ct.View(text.P()(text.Text("Owner: "), visible.Text("Ada"), text.Text(".")))
	t.Must(plain.Find(ct.Tag("p")).Matches(ct.InnerText("Owner: Ada.")))
	t.Must(plain.Find(ct.Tag("bdi")).Matches(ct.Text("Ada")))
	t.Must(plain.Find(ct.Class("badge")).None())
	t.Must(plain.Find(ct.Tag("code")).None())
}

func TestStringForms(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct{ in, text, code string }{
		{"", "", ""},
		{"Ada Lovelace", "Ada Lovelace", "Ada Lovelace"},
		{`C:\notes`, `C:\notes`, `C:\notes`},
		{"<b>bold</b>", "<b>bold</b>", "<b>bold</b>"},
		{"10\u00A0km", "10\u00A0km", bs("escaped 10~u00A0km")},
		{"\U0001F469\u200D\U0001F4BB", "\U0001F469\u200D\U0001F4BB", bs("escaped \U0001F469~u200D\U0001F4BB")},
		{"invoice\u202Efdp.exe", bs("escaped invoice~u202Efdp.exe"), bs("escaped invoice~u202Efdp.exe")},
		{"fake \\u202E real \u202E", bs("escaped fake ~~u202E real ~u202E"), bs("escaped fake ~~u202E real ~u202E")},
		{"two  spaces", bs("escaped two~x20~x20spaces"), bs("escaped two~x20~x20spaces")},
		{"Team\nnotes", bs("escaped Team~nnotes"), bs("escaped Team~nnotes")},
	} {
		t.Mustf(be.Equal(c.text, visible.TextString(c.in)), "TextString(%+q)", c.in)
		t.Mustf(be.Equal(c.code, visible.CodeString(c.in)), "CodeString(%+q)", c.in)
		t.Mustf(be.Equal(c.text, visible.Marking{}.TextString(c.in)), "Marking{}.TextString(%+q)", c.in)
		t.Mustf(be.Equal(c.code, visible.Marking{}.CodeString(c.in)), "Marking{}.CodeString(%+q)", c.in)
	}

	m := visible.Marking{Label: "maskiert", Title: "Mit Escapes gezeigt"}
	t.Must(be.Equal("Ada", m.TextString("Ada")))
	t.Must(be.Equal(bs("maskiert ad~u200Bmin"), m.TextString("ad\u200Bmin")))
	t.Must(be.Equal(bs("maskiert 10~u00A0km"), m.CodeString("10\u00A0km")))
	// A blank label falls back to the default, as the badge does.
	t.Must(be.Equal(bs("escaped ad~u200Bmin"), visible.Marking{Label: " \t"}.TextString("ad\u200Bmin")))
}

// TestStringFormsReadAsNodes checks that the string forms read as the
// markup of Text and Code reads: the marker's word, a space and the escaped
// value, without the invisible characters of the input.
func TestStringFormsReadAsNodes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	samples := append(webuitest.Hostile(), "Ada Lovelace", "ad\u200Bmin", `C:\notes`+"\u202E", "two  spaces")
	for _, s := range samples {
		for _, c := range []struct {
			node elem.Node
			str  string
		}{
			{visible.Text(s), visible.TextString(s)},
			{visible.Code(s), visible.CodeString(s)},
		} {
			t.Mustf(be.False(strings.ContainsFunc(c.str, func(r rune) bool {
				return unicode.In(r, unicode.Bidi_Control, unicode.Cc, unicode.Cf)
			})), "string form of %+q has an invisible character", s)
			page := ct.View(text.P()(c.node))
			t.Mustf(page.Find(ct.Tag("p")).Matches(ct.InnerText(c.str)), "string form of %+q", s)
		}
	}
}

func TestProseMarkup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct{ in, out string }{
		{"", `<bdi></bdi>`},
		{"Water the plants", `<bdi>Water the plants</bdi>`},
		{"Water the plants\non Friday", `<bdi>Water the plants</bdi><br/><bdi>on Friday</bdi>`},
		{"a\r\nb\n", `<bdi>a</bdi><br/><bdi>b</bdi><br/><bdi></bdi>`},
		{"a\n\nb", `<bdi>a</bdi><br/><bdi></bdi><br/><bdi>b</bdi>`},
		{"  indented\n\ttabbed  ", "<bdi>  indented</bdi><br/><bdi>\ttabbed  </bdi>"},
		{"<b>bold</b>\n& \"quoted\"", `<bdi>&lt;b&gt;bold&lt;/b&gt;</bdi><br/><bdi>&amp; &#34;quoted&#34;</bdi>`},
		{"\u0645\u0631\u062D\u0628\u0627\nHello", "<bdi>\u0645\u0631\u062D\u0628\u0627</bdi><br/><bdi>Hello</bdi>"},
		{"line\nad\u200Bmin", marker + bs(` <code dir="ltr">line<br/>ad~u200Bmin</code>`)},
		{"C:\\notes\r\n\u202E", marker + bs(` <code dir="ltr">C:~~notes<br/>~u202E</code>`)},
		{"old\rmac", marker + bs(` <code dir="ltr">old~rmac</code>`)},
		{"line\u2028separator", marker + bs(` <code dir="ltr">line~u2028separator</code>`)},
		{"<i>x</i>\n\u00A0", marker + bs(` <code dir="ltr">&lt;i&gt;x&lt;/i&gt;<br/>~u00A0</code>`)},
	} {
		t.Mustf(be.Equal(c.out, render(t, visible.Prose(c.in))), "Prose(%+q)", c.in)
	}
}

// TestProseView checks prose as a reader and a test see it: the lines
// stay lines, a single marker comes first, and nothing in the value reorders
// or hides the text around it.
func TestProseView(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct{ in, text string }{
		// Bidi controls on one line of several.
		{"First line\nevil\u202Etxt.exe\u2066x\nlast line", bs("escaped First line evil~u202Etxt.exe~u2066x last line")},
		// A line separator, which is no line break.
		{"one\u2028two", bs("escaped one~u2028two")},
		// A no-break space next to white space looks like two spaces.
		{"Ada \u00A0Lovelace\nnext", bs("escaped Ada ~u00A0Lovelace next")},
		// Zero-width characters.
		{"ad\u200Bmin\r\nre\u200Dview\u2060", bs("escaped ad~u200Bmin re~u200Dview~u2060")},
	} {
		page := ct.View(text.P()(text.Text("Note: "), visible.Prose(c.in), text.Text(".")))
		p := page.Find(ct.Tag("p"))
		t.Mustf(p.Matches(ct.InnerText("Note: "+c.text+".")), "Prose(%+q)", c.in)
		t.Mustf(p.Find(ct.Class("badge")).Count(1), "Prose(%+q) marker", c.in)
		t.Mustf(p.Find(ct.Class("badge").Or(ct.Tag("code"))).InOrder(ct.Class("badge"), ct.Tag("code")), "Prose(%+q) order", c.in)
		t.Mustf(p.Find(ct.Tag("code")).Matches(ct.Attr("dir", "ltr")), "Prose(%+q) direction", c.in)
		t.Mustf(p.Find(ct.Tag("code")).Find(ct.Tag("br")).Count(strings.Count(c.in, "\n")), "Prose(%+q) lines", c.in)
		t.Mustf(page.Valid(webuitest.Rules()...), "Prose(%+q) breaks the webui rules", c.in)
	}

	// Prose that needs no escape isolates every line.
	page := ct.View(text.P()(visible.Prose("\u0645\u0631\u062D\u0628\u0627\nHello  world\n\tagain")))
	t.Must(page.Find(ct.Tag("bdi")).Count(3))
	t.Must(page.Find(ct.Tag("br")).Count(2))
	t.Must(page.Find(ct.Tag("p")).Matches(ct.InnerText("\u0645\u0631\u062D\u0628\u0627 Hello world again")))
	t.Must(page.Find(ct.Class("badge")).None())
	t.Must(page.Find(ct.Tag("code")).None())
}

// TestNodesHostile renders the hostile inputs of webuitest: they stay
// text, the rendered HTML has no direction controls or other invisible
// format characters, and it satisfies the webui rules (no inline styles).
func TestNodesHostile(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range webuitest.Hostile() {
		for _, c := range []struct {
			node elem.Node
			tabs bool // prose keeps tabs, which HTML collapses
		}{
			{visible.Text(s), false},
			{visible.Code(s), false},
			{visible.Prose(s), true},
		} {
			node := c.node
			html := render(t, node)
			t.Mustf(be.False(strings.ContainsFunc(html, func(r rune) bool {
				return unicode.In(r, unicode.Bidi_Control, unicode.Cc, unicode.Cf) && !(c.tabs && r == '\t')
			})), "rendered %+q has an invisible character", s)
			page := ct.View(node)
			t.Mustf(page.Valid(webuitest.Rules()...), "rendered %+q breaks the webui rules", s)
			t.Mustf(page.Find(ct.Tag("script")).None(), "rendered %+q has a script", s)
			t.Mustf(page.Find(ct.HasAttr("style")).None(), "rendered %+q has a style", s)
			t.Mustf(page.Find(ct.Tag("img").Or(ct.Tag("svg"), ct.Tag("a"), ct.Tag("div"))).None(), "rendered %+q has markup", s)
			t.Mustf(page.Find(ct.HasAttr("hx-get").Or(ct.HasAttr("hx-on:click"), ct.HasAttr("onerror"))).None(),
				"rendered %+q has attributes", s)
		}
	}
}

// TestMarking checks the marker with other words: the zero value renders
// as Text, Code and Marker do, blank words fall back to the defaults, and
// the words stay text.
func TestMarking(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range []string{"", "Ada", "ad\u200Bmin", "10\u00A0km", "invoice\u202Efdp.exe", "two\nlines", "two\nad\u200Bmin"} {
		t.Mustf(be.Equal(render(t, visible.Text(s)), render(t, visible.Marking{}.Text(s))), "Text(%+q)", s)
		t.Mustf(be.Equal(render(t, visible.Code(s)), render(t, visible.Marking{}.Code(s))), "Code(%+q)", s)
		t.Mustf(be.Equal(render(t, visible.Prose(s)), render(t, visible.Marking{}.Prose(s))), "Prose(%+q)", s)
	}
	t.Must(be.Equal(`<span class="badge text-bg-warning" title="`+visible.MarkerTitle+`">maskiert</span>`+bs(` <code dir="ltr">a<br/>b~u200B</code>`),
		render(t, visible.Marking{Label: "maskiert"}.Prose("a\nb\u200B"))))
	t.Must(be.Equal(render(t, visible.Marker()), render(t, visible.Marking{}.Marker())))
	t.Must(be.Equal(marker, render(t, visible.Marking{Label: " ", Title: "\t\n"}.Marker())))

	m := visible.Marking{Label: "<maskiert>", Title: `Mit "Escapes" gezeigt`}
	t.Must(be.Equal(`<span class="badge text-bg-warning" title="Mit &#34;Escapes&#34; gezeigt">&lt;maskiert&gt;</span>`,
		render(t, m.Marker())))
	page := ct.View(text.P()(m.Text("ad\u200Bmin"), text.Text(" "), m.Code("Ada"), text.Text(" "), m.Code("a\tb")))
	t.Must(page.Find(ct.Class("badge")).Each(ct.Attr("title", `Mit "Escapes" gezeigt`), ct.Text("<maskiert>")))
	t.Must(page.Find(ct.Class("badge")).Count(2))
	t.Must(page.Find(ct.Tag("p")).Matches(ct.InnerText(bs("<maskiert> ad~u200Bmin Ada <maskiert> a~tb"))))
	t.Must(page.Find(ct.Tag("maskiert")).None())
	t.Must(page.Valid(webuitest.Rules()...))
}
