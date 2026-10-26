package webuitest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/ba"
	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/webuitest"
)

// benchRows is the number of table rows of [largePage]: about 5000
// elements in all.
const benchRows = 250

// largePage is a large, realistic page of a project list: page.Document
// around page.Layout with a navbar, a sidebar of links, and a table whose
// rows hold a link, an SVG icon, a badge, an hx-get button and a POST
// form with an icon button, and a dialog with its opener.
func largePage(rows int) elem.Node {
	svg := elem.Name("svg")
	path := elem.Name("path")
	icon := svg.New(attr.KV("viewBox", "0 0 16 16"), attr.KV("width", "16"), attr.KV("height", "16"), attr.KV("aria-hidden", "true"), attr.KV("focusable", "false"))(
		path.New(attr.KV("fill-rule", "evenodd"), attr.KV("d", "M1 2.5A1.5 1.5 0 0 1 2.5 1h3A1.5 1.5 0 0 1 7 2.5v3A1.5 1.5 0 0 1 5.5 7h-3A1.5 1.5 0 0 1 1 5.5z"))(),
	)
	var side []page.NavItem
	for i := range 30 {
		side = append(side, page.NavItem{Label: fmt.Sprintf("Group %d", i), Href: fmt.Sprintf("/groups/%d", i), Icon: bi.Folder, Badge: fmt.Sprint(i), Current: i == 3})
	}
	body := make([]bs.TableRow, rows)
	for i := range body {
		base := fmt.Sprintf("/projects/%d", i)
		body[i] = bs.TableRow{
			Attrs: attr.ID(fmt.Sprintf("project-%d", i)),
			Cells: []elem.Node{
				elem.Bundle{icon, text.Text(" "), text.A(text.Href(base))(text.Text(fmt.Sprintf("Project %d", i)))},
				bs.BadgeOf(bs.VariantSecondary)(text.Text(fmt.Sprint(i % 7))),
				text.Text(fmt.Sprintf("Note about project %d", i)),
				bs.BtnOutlineSecondary(button.Type(button.TypeButton), hx2.Get(base+"/edit"), hx2.Target("closest tr"), hx2.Swap(hx2.SwapOuterHTML))(
					bi.Pencil, text.Span(ba.VisuallyHidden())(text.Text(fmt.Sprintf("Edit project %d", i))),
				),
				session.PostForm(base+"/delete", testToken, hx2.Post(base+"/delete"), hx2.Target("closest tr"), hx2.Swap(hx2.SwapOuterHTML))(
					bs.BtnOutlineDanger(button.Type(button.TypeSubmit))(bi.Trash, text.Span(ba.VisuallyHidden())(text.Text(fmt.Sprintf("Delete project %d", i)))),
				),
			},
		}
	}
	return page.Document{
		Title:  "Projects",
		Assets: []*asset.Set{benchAssets},
		Body: page.Layout{
			Navbar: page.Navbar{
				Brand: page.Brand{Label: "Projects", Href: "/"},
				Items: []page.NavItem{{Label: "Projects", Href: "/projects", CurrentSection: true}, {Label: "Tasks", Href: "/tasks"}, {Label: "Notes", Href: "/notes"}},
				End:   []elem.Node{session.PostForm("/logout", testToken)(bs.BtnSecondary(button.Type(button.TypeSubmit))(text.Text("Sign out")))},
			},
			Sidebar: page.Sidebar{Label: "Groups", Sections: []page.SideSection{{Title: "Groups", Items: side}}},
			Header: page.Header{Title: text.Text("Projects"), Actions: []elem.Node{
				bs.DialogButton("new-project", attr.Class("btn btn-primary"))(text.Text("New project")),
			}},
			Main: bs.DataTable{
				Caption:       text.Text("Projects"),
				CaptionHidden: true,
				Columns: []bs.Column{
					{Header: text.Text("Name"), RowHeader: true},
					{Header: text.Text("Tasks"), Attrs: attr.Class("text-end")},
					{Header: text.Text("Note")},
					{Header: text.Text("Edit"), HeaderHidden: true},
					{Header: text.Text("Delete"), HeaderHidden: true},
				},
				Rows:  body,
				Hover: true,
			},
			Dialogs: []elem.Node{bs.Dialog{
				ID:    "new-project",
				Title: text.Text("New project"),
				Body: session.PostForm("/projects", testToken, attr.ID("new-project-form"))(
					bs.Field{ID: "new-project-name", Name: "name", Label: text.Text("Name"), Attrs: input.Autofocus()},
				),
				Footer: elem.Bundle{
					bs.BtnSecondary(bs.DialogClose("new-project"))(text.Text("Cancel")),
					bs.BtnPrimary(button.Type(button.TypeSubmit), attr.KV("form", "new-project-form"))(text.Text("Create")),
				},
			}},
		},
	}
}

// benchAssets are the assets of [largePage].
var benchAssets = theme.MustAssets(theme.Options{})

// renderPage renders a view as a page handler writes it.
func renderPage(tb testing.TB, view elem.Node) string {
	tb.Helper()
	var b strings.Builder
	if err := core.Render(context.Background(), view, &b); err != nil {
		tb.Fatal(err)
	}
	return b.String()
}

// TestLargePage checks that the page of the benchmarks is a valid webui
// page, so that the benchmarks measure the rules on a page that passes
// them, as most crawled pages do.
func TestLargePage(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := largePage(20)
	shell := webuitest.DefaultShell(benchAssets)
	rules := append(webuitest.Rules(webuitest.WithCSRF(testToken)), webuitest.LayoutRegions())
	t.Must(webuitest.PageContract(ct.View(view), shell, rules...))
	t.Must(webuitest.PageContract(cthtml.PageString(renderPage(t, view)), shell, rules...))
	t.Must(cthtml.PageString(renderPage(t, view)).Find(ct.Tag("svg"), ct.Attr("viewBox", "0 0 16 16")).Count(20))
}

// BenchmarkRules runs every rule of [webuitest.Rules] on a large page,
// loaded once: parsed as a crawl parses it, and as a chord view. The parse
// sub-benchmark is the cost of parsing the page, for comparison.
func BenchmarkRules(b *testing.B) {
	view := largePage(benchRows)
	body := renderPage(b, view)
	rules := webuitest.Rules(webuitest.WithCSRF(testToken))
	b.Run("parse", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := cthtml.PageString(body).Load(b.Context()); err != nil {
				b.Fatal(err)
			}
		}
	})
	for _, c := range []struct {
		name    string
		subject *ct.Subject
	}{
		{"parsed", cthtml.PageString(body)},
		{"view", ct.View(view)},
	} {
		doc, err := c.subject.Load(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		root := doc.Root()
		elements := 0
		for range ct.Elements(root) {
			elements++
		}
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, r := range rules {
					if v := r.Violations(root); len(v) > 0 {
						b.Fatalf("%s: %q", r, v)
					}
				}
			}
			b.ReportMetric(float64(elements), "elements")
		})
	}
}

// BenchmarkPageContract checks a large page as [webuitest.Crawl] checks
// every page it fetches: parse the body, then apply the shell and the
// rules in one contract.
func BenchmarkPageContract(b *testing.B) {
	body := renderPage(b, largePage(benchRows))
	shell := webuitest.DefaultShell(benchAssets)
	b.ReportAllocs()
	for b.Loop() {
		page := cthtml.PageString(body)
		if err := webuitest.PageContract(page, shell, webuitest.Rules(webuitest.WithCSRF(testToken))...).Check(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
}
