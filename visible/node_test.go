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
		{"invoice\u202Efdp.exe", bs(`<code dir="ltr">invoice~u202Efdp.exe</code> `) + marker},
		{"<i>x</i>\u200B", bs(`<code dir="ltr">&lt;i&gt;x&lt;/i&gt;~u200B</code> `) + marker},
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
		{"10\u00A0km", bs(`<code dir="ltr">10~u00A0km</code> `) + marker},
		{`C:\notes` + "\n", bs(`<code dir="ltr">C:~~notes~n</code> `) + marker},
	} {
		t.Mustf(be.Equal(c.out, render(t, visible.Code(c.in))), "Code(%+q)", c.in)
	}
}

// TestMarkerView checks the marker as a reader and a test see it.
func TestMarkerView(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(text.P()(text.Text("Owner: "), visible.Text("ad\u200Bmin"), text.Text(".")))
	t.Must(page.Find(ct.Tag("p")).Matches(ct.InnerText(bs("Owner: ad~u200Bmin escaped."))))
	t.Must(page.Find(ct.Tag("code")).Matches(ct.Attr("dir", "ltr"), ct.TextContent(bs("ad~u200Bmin"))))
	t.Must(page.Find(ct.Class("badge")).Matches(ct.Attr("title", visible.MarkerTitle), ct.Text(visible.MarkerText)))
	t.Must(page.Find(ct.Tag("bdi")).None())

	plain := ct.View(text.P()(text.Text("Owner: "), visible.Text("Ada"), text.Text(".")))
	t.Must(plain.Find(ct.Tag("p")).Matches(ct.InnerText("Owner: Ada.")))
	t.Must(plain.Find(ct.Tag("bdi")).Matches(ct.Text("Ada")))
	t.Must(plain.Find(ct.Class("badge")).None())
	t.Must(plain.Find(ct.Tag("code")).None())
}

// TestNodesHostile renders the hostile inputs of webuitest: they stay
// text, the rendered HTML has no direction controls or other invisible
// format characters, and it satisfies the webui rules (no inline styles).
func TestNodesHostile(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range webuitest.Hostile() {
		for _, node := range []elem.Node{visible.Text(s), visible.Code(s)} {
			html := render(t, node)
			t.Mustf(be.False(strings.ContainsFunc(html, func(r rune) bool {
				return unicode.In(r, unicode.Bidi_Control, unicode.Cc, unicode.Cf)
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
