package page_test

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/meta"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/theme"
)

func TestColorModeMatchesTheme(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(theme.ColorMode, string(page.ColorModeDark)))
}

func TestDocumentHead(gt *testing.T) {
	t := devtest.ParallelT(gt)
	themeSet := theme.MustAssets(theme.Options{})
	app := appAssets()
	doc := ct.View(page.Document{
		Title:  "Tasks",
		Assets: []*asset.Set{themeSet, app},
		Head:   []elem.Node{meta.Meta(meta.Name("description"), meta.Content("A task list"))},
	})
	head := doc.Find(ct.Tag("head"))
	t.Must(head)
	t.Must(head.Find(ct.Tag("meta").Or(ct.Tag("title"), ct.Tag("link"), ct.Tag("script"))).InOrder(
		ct.Attr("charset", "utf-8"),
		ct.Attr("name", "viewport").And(ct.Attr("content", page.Viewport)),
		ct.Tag("title").And(ct.Text("Tasks")),
		ct.Attr("name", "color-scheme").And(ct.Attr("content", "dark")),
		ct.Attr("rel", "icon").And(ct.Attr("href", "data:,")),
		ct.Attr("name", "htmx-config"),
		ct.Attr("href", themeSet.URL(theme.BootstrapCSS)),
		ct.Attr("href", themeSet.URL(theme.IconsCSS)),
		ct.Attr("href", themeSet.URL(theme.ThemeCSS)),
		ct.Attr("href", app.URL("app.css")),
		ct.Attr("src", themeSet.URL(theme.HTMXJS)),
		ct.Attr("src", themeSet.URL(theme.SSEJS)),
		ct.Attr("src", app.URL("app.js")),
		ct.Attr("name", "description"),
	))
	t.Must(head.Find(ct.Tag("script")).Each(ct.Attr("defer", ""), ct.HasAttr("integrity"), ct.Attr("crossorigin", "anonymous")))
	t.Must(head.Find(ct.Attr("rel", "stylesheet")).Each(ct.HasAttr("integrity"), ct.Attr("crossorigin", "anonymous")))
	t.Must(doc.Find(ct.Tag("body")).Find(ct.Tag("script")).None())

	want, err := hx2.Config{}.JSON()
	t.Must(be.NoError(err))
	t.Must(head.Find(ct.Attr("name", "htmx-config")).Matches(ct.Attr("content", want)))
}

func TestDocumentDefaults(gt *testing.T) {
	t := devtest.ParallelT(gt)
	out, err := render(page.Document{Title: "Tasks"})
	t.Must(be.NoError(err))
	t.Must(be.Prefix(out, `<!DOCTYPE html><html lang="en" data-bs-theme="dark"><head><meta charset="utf-8"/>`))
	t.Must(be.Suffix(out, `</head><body></body></html>`))

	doc := cthtml.PageString(out)
	t.Must(doc.Find(ct.Tag("html")).Matches(ct.Attr("lang", "en"), ct.Attr("data-bs-theme", "dark")))
	t.Must(doc.Find(ct.Attr("name", "color-scheme")).Matches(ct.Attr("content", "dark")))
	t.Must(doc.Find(ct.Attr("rel", "icon")).Matches(ct.Attr("href", page.DefaultFavicon)))
	t.Must(doc.Find(ct.Tag("title")).Texts("Tasks"))
}

func TestDocumentOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	doc := ct.View(page.Document{
		Lang:      "pt-BR",
		ColorMode: page.ColorModeLight,
		Title:     "Tarefas <&>",
		Favicon:   "/assets/app/icon.svg",
		HTMLAttrs: attr.Data("density", "compact"),
		BodyAttrs: attr.Class("app"),
		Body:      text.P()(text.Text("Olá")),
	})
	t.Must(doc.Find(ct.Tag("html")).Matches(ct.Attr("lang", "pt-BR"), ct.Attr("data-bs-theme", "light"), ct.Attr("data-density", "compact")))
	t.Must(doc.Find(ct.Attr("name", "color-scheme")).Matches(ct.Attr("content", "light")))
	t.Must(doc.Find(ct.Attr("rel", "icon")).Matches(ct.Attr("href", "/assets/app/icon.svg")))
	t.Must(doc.Find(ct.Tag("title")).Texts("Tarefas <&>"))
	t.Must(doc.Find(ct.Tag("body")).Matches(ct.Class("app")))
	t.Must(doc.Find(ct.Tag("body")).Find(ct.Tag("p")).Texts("Olá"))

	for _, icon := range []string{"data:,", "data:image/svg+xml,%3Csvg%3E", "data:image/png;base64,iVBORw0KGgo=", "/favicon.ico", "/a/b.png?v=1"} {
		_, err := render(page.Document{Title: "x", Favicon: icon})
		t.Mustf(be.NoError(err), "favicon %q", icon)
	}
}

func TestDocumentErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const marker = "MARKER"
	cases := []struct {
		name string
		doc  page.Document
		want error
	}{
		{"no title", page.Document{}, page.ErrNoName},
		{"blank title", page.Document{Title: " \t"}, page.ErrNoName},
		{"color mode", page.Document{Title: "x", ColorMode: "auto"}, page.ErrInvalid},
		{"color mode case", page.Document{Title: "x", ColorMode: "Dark"}, page.ErrInvalid},
		{"lang one letter", page.Document{Title: "x", Lang: "e"}, page.ErrInvalid},
		{"lang underscore", page.Document{Title: "x", Lang: "en_US"}, page.ErrInvalid},
		{"lang long subtag", page.Document{Title: "x", Lang: "en-abcdefghi"}, page.ErrInvalid},
		{"lang long", page.Document{Title: "x", Lang: "en-" + strings.Repeat("a-", 16) + "a"}, page.ErrInvalid},
		{"lang trailing hyphen", page.Document{Title: "x", Lang: "en-"}, page.ErrInvalid},
		{"lang leading hyphen", page.Document{Title: "x", Lang: "-en"}, page.ErrInvalid},
		{"lang digit primary", page.Document{Title: "x", Lang: "e1"}, page.ErrInvalid},
		{"lang quote", page.Document{Title: "x", Lang: `en"` + marker}, page.ErrInvalid},
		{"favicon script", page.Document{Title: "x", Favicon: "javascript:" + marker}, page.ErrInvalid},
		{"favicon other origin", page.Document{Title: "x", Favicon: "https://cdn.example/" + marker}, page.ErrInvalid},
		{"favicon scheme relative", page.Document{Title: "x", Favicon: "//cdn.example/" + marker}, page.ErrInvalid},
		{"favicon backslash", page.Document{Title: "x", Favicon: `/\cdn.example/` + marker}, page.ErrInvalid},
		{"favicon tab", page.Document{Title: "x", Favicon: "/\t/cdn.example/" + marker}, page.ErrInvalid},
		{"favicon relative", page.Document{Title: "x", Favicon: "icon.svg"}, page.ErrInvalid},
		{"favicon data html", page.Document{Title: "x", Favicon: "data:text/html," + marker}, page.ErrInvalid},
		{"favicon space", page.Document{Title: "x", Favicon: "/a b.png"}, page.ErrInvalid},
		{"nil asset set", page.Document{Title: "x", Assets: []*asset.Set{nil}}, page.ErrInvalid},
		{"htmx config", page.Document{Title: "x", HTMX: hx2.Config{HistoryCacheSize: -1}}, hx2.ErrInvalidConfig},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			out, err := render(c.doc)
			t.Must(be.ErrorIs(err, c.want))
			t.Must(be.Equal("", out))
			t.Must(be.False(strings.Contains(err.Error(), marker)))
		})
	}
}

func TestLayout(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(page.Layout{
		Navbar:  page.Navbar{Items: []page.NavItem{{Label: "Inbox", Href: "/inbox"}}},
		Header:  page.Header{Title: text.Text("Inbox")},
		Alerts:  []elem.Node{page.Notice("", text.Text("Saved."))},
		Main:    form.Form(attr.Name("method").Raw("post"))(text.Text("form")),
		Footer:  text.Text("Footer"),
		Dialogs: []elem.Node{bs.Dialog{ID: "help", Title: text.Text("Help")}},
	})
	t.Must(view.Valid(rules()...))

	skip := view.Find(ct.Role("link", ct.Named(page.DefaultSkipLabel)))
	t.Must(skip.Matches(ct.Attr("href", "#"+page.MainID), ct.Class("visually-hidden-focusable")))
	// The skip link is the first link of the page.
	t.Must(view.Find(ct.Role("link")).First().Matches(ct.Attr("href", "#main")))

	main := view.Find(ct.Role("main"))
	t.Must(main.Matches(ct.ID(page.MainID)))
	t.Must(main.Find(ct.Class("container")))
	t.Must(main.Find(ct.ID(page.HeaderID).Or(ct.ID(page.AlertsID), ct.Tag("form"))).InOrder(
		ct.ID(page.HeaderID), ct.ID(page.AlertsID), ct.Tag("form"),
	))
	alerts := main.Find(ct.ID(page.AlertsID))
	t.Must(alerts.Matches(ct.Attr("aria-live", "polite")))
	t.Must(alerts.Find(ct.Role("status")).Texts("Saved."))

	t.Must(view.Find(ct.Role("contentinfo")).Texts("Footer"))
	dialogs := view.Find(ct.ID(page.DialogsID))
	t.Must(dialogs.Find(ct.Tag("dialog")).Matches(ct.ID("help")))
	t.Must(view.Find(ct.Tag("form")).Find(ct.Tag("dialog")).None())
	t.Must(main.Find(ct.ID(page.DialogsID)).None())
	t.Must(view.Find(ct.Class("row")).None())
}

func TestLayoutVariants(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("fluid", func(t devtest.T) {
		view := ct.View(page.Layout{Fluid: true, Main: text.Text("x"), Footer: text.Text("f")})
		t.Must(view.Find(ct.Class("container")).None())
		t.Must(view.Find(ct.Class("container-fluid")).Count(2))
	})
	t.Run("sidebar", func(t devtest.T) {
		view := ct.View(page.Layout{
			Sidebar: page.Sidebar{Sections: []page.SideSection{{Items: []page.NavItem{{Label: "A", Href: "/a"}}}}},
			Main:    text.Text("x"),
		})
		t.Must(view.Valid(rules()...))
		row := view.Find(ct.Class("row"))
		t.Must(row)
		t.Must(row.Find(ct.Role("navigation", ct.Named(page.DefaultSidebarLabel)).Or(ct.Role("main"))).InOrder(
			ct.Tag("nav"), ct.ID(page.MainID).And(ct.Class("col-md-9")),
		))
		t.Must(view.Find(ct.Role("main")).Find(ct.ID(page.AlertsID)))
		t.Must(view.Find(ct.Class("container")).None())
	})
	t.Run("skip link", func(t devtest.T) {
		t.Must(ct.View(page.Layout{SkipLabel: "Zum Inhalt"}).Find(ct.Role("link", ct.Named("Zum Inhalt"))))
		t.Must(ct.View(page.Layout{NoSkipLink: true}).Find(ct.Role("link")).None())
	})
	t.Run("empty", func(t devtest.T) {
		out, err := render(page.Layout{NoSkipLink: true})
		t.Must(be.NoError(err))
		t.Must(be.Equal(`<div class="d-flex flex-column min-vh-100"><main id="main" class="flex-grow-1"><div class="container py-3"><div id="alerts" aria-live="polite"></div></div></main></div><div id="dialogs"></div>`, out))
	})
}

func TestRegions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	out, err := render(elem.Bundle{
		page.AlertsRegion(page.ErrorAlert(text.Text("x"))),
		page.DialogsRegion(),
		page.AlertsOOB(page.Notice(bs.VariantInfo, text.Text("y"))),
		page.AlertsOOB(),
	})
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div id="alerts" aria-live="polite"><div class="alert alert-danger" role="alert">x</div></div>`+
		`<div id="dialogs"></div>`+
		`<div id="alerts" hx-swap-oob="innerHTML"><div class="alert alert-info" role="status">y</div></div>`+
		`<div id="alerts" hx-swap-oob="innerHTML"></div>`, out))
}

func TestNavbar(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(page.Navbar{
		Brand: page.Brand{Label: "Tasks", Icon: "check2-square"},
		Items: []page.NavItem{
			{Label: "Inbox", Href: "/inbox", Icon: "inbox", Current: true, Badge: "3", BadgeVariant: bs.VariantWarning},
			{Label: "Notes", Href: "/notes"},
		},
		End: []elem.Node{logout()},
	})
	t.Must(view.Valid(rules()...))
	nav := view.Find(ct.Role("navigation", ct.Named(page.DefaultNavbarLabel)))
	t.Must(nav.Matches(ct.ID(page.NavbarID), ct.Class("navbar-expand")))
	t.Must(nav.Find(ct.Role("link", ct.Named("Tasks"))).Matches(ct.Class("navbar-brand"), ct.Attr("href", "/"), ct.HasAttr("aria-current").Not()))
	t.Must(nav.Find(ct.Role("link", ct.Named("Inbox 3"))).Matches(ct.Class("active"), ct.Attr("aria-current", "page"), ct.Attr("href", "/inbox")))
	t.Must(nav.Find(ct.Role("link", ct.Named("Notes"))).Matches(ct.HasAttr("aria-current").Not(), ct.Class("active").Not()))
	t.Must(nav.Find(ct.Class("badge")).Matches(ct.Class("text-bg-warning")))
	t.Must(nav.Find(ct.Class("bi")).Each(ct.Attr("aria-hidden", "true")))
	t.Must(nav.Find(ct.Role("listitem")).Count(2))
	t.Must(nav.Find(ct.Role("button", ct.Named("Log out"))))
	// No toggler or collapse, which would need Bootstrap's script.
	t.Must(nav.Find(ct.HasAttr("data-bs-toggle").Or(ct.Class("collapse"), ct.Class("navbar-toggler"))).None())
}

func TestNavbarOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(page.Navbar{
		ID:    "top",
		Label: "Account",
		Brand: page.Brand{Label: "Tasks", Href: "/start", Current: true},
		Attrs: hx2.SwapOOB("true"),
	})
	nav := view.Find(ct.Role("navigation", ct.Named("Account")))
	t.Must(nav.Matches(ct.ID("top"), ct.Attr("hx-swap-oob", "true")))
	t.Must(nav.Find(ct.Role("link", ct.Named("Tasks"))).Matches(ct.Attr("href", "/start"), ct.Attr("aria-current", "page")))
	t.Must(nav.Find(ct.Role("list")).None())

	out, err := render(page.Navbar{})
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<nav id="navbar" class="navbar navbar-expand bg-body-tertiary border-bottom" aria-label="Main"><div class="container-fluid flex-wrap gap-2"></div></nav>`, out))
}

// TestLinkURLs checks the link targets that navigation accepts and refuses.
func TestLinkURLs(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const marker = "MARKER"
	good := []string{"/", "/inbox", "/p/a%20b?x=1#top", "?page=2", "#top", "../up", "https://example.com/" + marker, "HTTP://example.com/"}
	bad := []string{
		"", "javascript:" + marker, "JavaScript:" + marker, "vbscript:" + marker, "data:text/html," + marker,
		"mailto:" + marker, " /inbox", "/inbox ", "/in\tbox", "/in\nbox", "/\\" + marker, "/café", "/\x7f",
		"%zz", "file:///" + marker,
	}
	for _, href := range good {
		for name, node := range linkHolders(href) {
			_, err := render(node)
			t.Mustf(be.NoError(err), "%s with href %q", name, href)
		}
	}
	for _, href := range bad {
		for name, node := range linkHolders(href) {
			if href == "" && (name == "crumb" || name == "brand") {
				continue // a crumb without a link is text; a brand links to "/"
			}
			_, err := render(node)
			t.Mustf(be.ErrorIs(err, page.ErrInvalid), "%s with href %q", name, href)
			t.Mustf(be.False(strings.Contains(err.Error(), marker)), "%s error quotes the href", name)
		}
	}
}

// linkHolders returns every component with a link to href.
func linkHolders(href string) map[string]elem.Node {
	return map[string]elem.Node{
		"brand":   page.Navbar{Brand: page.Brand{Label: "B", Href: href}},
		"navbar":  page.Navbar{Items: []page.NavItem{{Label: "A", Href: href}}},
		"sidebar": page.Sidebar{Sections: []page.SideSection{{Items: []page.NavItem{{Label: "A", Href: href}}}}},
		"crumb":   page.Header{Title: text.Text("T"), Crumbs: []page.Crumb{{Label: "A", Href: href}}},
	}
}

func TestComponentErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		node elem.Node
		want error
	}{
		{"navbar item label", page.Navbar{Items: []page.NavItem{{Href: "/a"}}}, page.ErrNoName},
		{"navbar item blank label", page.Navbar{Items: []page.NavItem{{Label: " ", Href: "/a"}}}, page.ErrNoName},
		{"navbar brand icon only", page.Navbar{Brand: page.Brand{Icon: "house"}}, page.ErrNoName},
		{"navbar brand href only", page.Navbar{Brand: page.Brand{Href: "/"}}, page.ErrNoName},
		{"navbar id", page.Navbar{ID: "two words"}, page.ErrInvalid},
		{"navbar badge variant", page.Navbar{Items: []page.NavItem{{Label: "A", Href: "/a", Badge: "1", BadgeVariant: "pink"}}}, page.ErrInvalid},
		{"sidebar id", page.Sidebar{ID: "#x"}, page.ErrInvalid},
		{"sidebar item label", page.Sidebar{Sections: []page.SideSection{{Items: []page.NavItem{{Href: "/a"}}}}}, page.ErrNoName},
		{"header title", page.Header{}, page.ErrNoName},
		{"header crumb label", page.Header{Title: text.Text("T"), Crumbs: []page.Crumb{{Href: "/"}}}, page.ErrNoName},
		{"notice variant", page.Notice("pink", text.Text("x")), bs.ErrInvalidOption},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			_, err := render(c.node)
			t.Must(be.ErrorIs(err, c.want))
		})
	}
}

func TestHeader(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(page.Header{
		Title:   text.Text("Garden"),
		Icon:    "flower1",
		Crumbs:  []page.Crumb{{Label: "Home", Href: "/"}, {Label: "Projects"}, {Label: "Garden"}},
		Actions: []elem.Node{bs.BtnPrimary()(text.Text("Edit"))},
		Below:   text.P()(text.Text("below")),
	})
	t.Must(view.Valid(rules()...))
	header := view.Find(ct.ID(page.HeaderID))
	t.Must(header.Matches(ct.Tag("header")))
	t.Must(header.Find(ct.Role("heading")).Matches(ct.Tag("h1"), ct.InnerText("Garden")))
	t.Must(header.Find(ct.Role("heading", ct.Level(1), ct.Named("Garden"))))
	t.Must(header.Find(ct.Tag("h1")).Find(ct.Class("bi")).Matches(ct.Attr("aria-hidden", "true")))
	crumbs := header.Find(ct.Role("navigation", ct.Named("Breadcrumb")))
	t.Must(crumbs.Find(ct.Role("listitem")).InOrder(
		ct.HasChild(ct.Role("link", ct.Named("Home"))),
		ct.InnerText("Projects").And(ct.HasAttr("aria-current").Not(), ct.HasChild(ct.Tag("a")).Not()),
		ct.InnerText("Garden").And(ct.Attr("aria-current", "page")),
	))
	t.Must(header.Find(ct.Role("heading").Or(ct.Role("button"), ct.Tag("p"))).InOrder(
		ct.Tag("h1"), ct.Role("button", ct.Named("Edit")), ct.Tag("p"),
	))

	out, err := render(page.Header{Title: text.Text("Inbox")})
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<header id="page-header" class="mb-3"><h1 class="h4 mb-0">Inbox</h1></header>`, out))
}

func TestSidebar(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(page.Sidebar{Label: "Projects", Sections: []page.SideSection{
		{Title: "Active", Items: []page.NavItem{{Label: "Home", Href: "/p/home", Current: true, Badge: "2"}, {Label: "Garden", Href: "/p/garden", Icon: "flower1"}}},
		{Items: []page.NavItem{{Label: "Archive", Href: "/p/archive"}}},
	}})
	t.Must(view.Valid(rules()...))
	nav := view.Find(ct.Role("navigation", ct.Named("Projects")))
	t.Must(nav.Matches(ct.ID(page.SidebarID)))
	t.Must(nav.Find(ct.Role("heading")).Matches(ct.Tag("h2"), ct.InnerText("Active")))
	t.Must(nav.Find(ct.Role("list")).Count(2))
	t.Must(nav.Find(ct.Role("link", ct.Named("Home 2"))).Matches(ct.Class("active"), ct.Attr("aria-current", "page")))
	t.Must(nav.Find(ct.Role("link", ct.Named("Garden"))).Matches(ct.HasAttr("aria-current").Not()))
	t.Must(nav.Find(ct.Role("link")).Texts("Home 2", "Garden", "Archive"))
}

func TestAlerts(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(elem.Bundle{
		page.ErrorAlert(text.Text("Refused.")),
		page.Notice("", text.Text("Saved.")),
		page.Notice(bs.VariantWarning, text.Text("Saved, with warnings.")),
	})
	t.Must(view.Find(ct.Role("alert")).Matches(ct.Class("alert-danger"), ct.InnerText("Refused.")))
	t.Must(view.Find(ct.Role("status")).InOrder(
		ct.Class("alert-success").And(ct.InnerText("Saved.")),
		ct.Class("alert-warning").And(ct.InnerText("Saved, with warnings.")),
	))
}

func TestGallery(gt *testing.T) {
	t := devtest.ParallelT(gt)
	node := gallery(appAssets())
	view := ct.View(node)
	t.Must(view.Valid(append(rules(), ct.NoRaw())...))
	out, err := render(node)
	t.Must(be.NoError(err))
	parsed := cthtml.PageString(out)
	t.Must(parsed.Valid(rules()...))
	t.Must(parsed.Find(ct.Role("main")))
	t.Must(parsed.Find(ct.Role("heading", ct.Level(1))))
	t.Must(parsed.Find(ct.Role("navigation")).Count(3)) // navbar, sidebar, breadcrumb
	t.Must(parsed.Find(ct.Tag("dialog")).Matches(ct.HasAttr("popover"), ct.HasAttr("open").Not()))
}

// TestBootstrapClasses checks that every class the components write exists
// in the Bootstrap stylesheet that the theme ships (icon classes are
// checked by the bi package).
func TestBootstrapClasses(gt *testing.T) {
	t := devtest.ParallelT(gt)
	css, err := os.ReadFile("../theme/files/" + theme.BootstrapCSS)
	t.Must(be.NoError(err))
	known := make(map[string]bool)
	for _, m := range regexp.MustCompile(`\.([a-zA-Z][a-zA-Z0-9_-]*)`).FindAllSubmatch(css, -1) {
		known[string(m[1])] = true
	}
	t.Must(be.True(len(known) > 1000))

	out, err := render(gallery(appAssets()))
	t.Must(be.NoError(err))
	var unknown []string
	walk(rootOf(t, cthtml.PageString(out)), func(n ct.Node) {
		class, _ := n.Attr("class")
		for _, c := range strings.Fields(class) {
			if c == "bi" || strings.HasPrefix(c, "bi-") {
				continue
			}
			if !known[c] {
				unknown = append(unknown, c)
			}
		}
	})
	t.Must(be.DeepEqual([]string(nil), unknown))
}

// rootOf loads the subject and returns its root node.
func rootOf(t devtest.T, s *ct.Subject) ct.Node {
	doc, err := s.Load(t.Context())
	t.Must(be.NoError(err))
	return doc.Root()
}

func TestErrorsWrapSentinels(gt *testing.T) {
	t := devtest.ParallelT(gt)
	_, err := render(page.Navbar{Items: []page.NavItem{{Label: "A"}}})
	t.Must(be.ErrorIs(err, page.ErrInvalid))
	t.Must(be.ErrorContains(err, "page.Navbar item 0: invalid option: href"))
	t.Must(be.False(errors.Is(err, page.ErrNoName)))
}
