package page_test

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/meta"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/webuitest"
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
		// The row fills the wrapper so that the columns reach the footer
		// from md up, while stacked lines keep their content height.
		t.Must(view.Find(ct.Class("container-fluid")).Matches(ct.Class("flex-grow-1"), ct.Class("d-flex"), ct.Class("flex-column")))
		t.Must(row.Matches(ct.Class("flex-grow-1"), ct.Class("align-content-start"), ct.Class("align-content-md-stretch")))
		// No border on the column: it would draw a line at the screen edge
		// once the columns stack. The rule is a .vr shown from md up.
		column := row.Find(ct.Class("col-md-3"))
		t.Must(column.Matches(ct.Class("border-end").Not(), ct.Class("position-relative")))
		rule := column.Find(ct.Class("vr"))
		t.Must(rule.Count(1))
		t.Must(rule.Matches(ct.Class("d-none"), ct.Class("d-md-block"), ct.Class("position-absolute"), ct.Class("h-100")))
		t.Must(view.Find(ct.Class("border-end")).None())
		out, err := render(page.Layout{NoSkipLink: true, Sidebar: text.Text("S")})
		t.Must(be.NoError(err))
		t.Must(be.Substring(out, `<div class="col-md-3 col-xl-2 py-3 position-relative">S<div class="vr d-none d-md-block position-absolute top-0 end-0 h-100"></div></div>`))
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
		`<div id="alerts" hx-swap-oob="innerHTML"><div class="alert alert-info">y</div></div>`+
		`<div id="alerts" hx-swap-oob="innerHTML"></div>`, out))

	out, err = render(elem.Bundle{
		page.DialogsOOB(bs.Dialog{ID: "help", Title: text.Text("Help")}),
		page.DialogsOOB(),
		page.AlertsContent(),
	})
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div id="dialogs" hx-swap-oob="innerHTML"><dialog id="help" class="modal" popover aria-labelledby="help-title"><div class="modal-dialog"><div class="modal-content"><div class="modal-header"><h2 class="modal-title h5" id="help-title">Help</h2><button class="btn-close" type="button" aria-label="Close" commandfor="help" command="close" popovertarget="help" popovertargetaction="hide" autofocus></button></div></div></div></dialog></div>`+
		`<div id="dialogs" hx-swap-oob="innerHTML"></div>`, out))
}

// live selects the elements that are live regions: those with aria-live
// and the roles that imply it.
var live = ct.HasAttr("aria-live").Or(ct.Attr("role", "alert"), ct.Attr("role", "status"))

// TestAlertsAnnounceOnce checks that each message has one live region to
// announce it: the alerts region for alerts swapped into it, the alert's
// role for alerts that come with the page or sit elsewhere.
func TestAlertsAnnounceOnce(gt *testing.T) {
	t := devtest.ParallelT(gt)
	alerts := func() []elem.Node {
		return []elem.Node{
			page.ErrorAlert(text.Text("Refused.")),
			page.Notice("", text.Text("Saved.")),
			// Deeper in the content too, as in a list of results.
			div.Div(attr.Class("vstack gap-2"))(page.Notice(bs.VariantWarning, text.Text("Saved, with warnings."))),
		}
	}
	t.Run("out of band", func(t devtest.T) {
		frag := ct.View(page.AlertsOOB(alerts()...))
		t.Must(frag.Find(live).None())
		t.Must(frag.Find(ct.ID(page.AlertsID)).Find(ct.Class("alert")).Texts("Refused.", "Saved.", "Saved, with warnings."))
		t.Must(webuitest.FragmentContract(frag, webuitest.Rules()...))
	})
	t.Run("retargeted answer", func(t devtest.T) {
		frag := ct.View(page.AlertsContent(alerts()...))
		t.Must(frag.Find(live).None())
		t.Must(frag.Find(ct.Class("alert-danger")).Texts("Refused."))
		t.Must(frag.Find(ct.Class("alert-success")).Texts("Saved."))
	})
	t.Run("swapped into the page", func(t devtest.T) {
		// The region after htmx swapped the content in (innerHTML): one
		// live region, the alerts region itself.
		view := ct.View(page.AlertsRegion(page.AlertsContent(alerts()...)))
		t.Must(view.Find(live).Count(1))
		t.Must(view.Find(live).Matches(ct.ID(page.AlertsID), ct.Attr("aria-live", "polite")))
		t.Must(view.Find(live).Find(live).None())
	})
	t.Run("with the page", func(t devtest.T) {
		doc := ct.View(page.Layout{Alerts: alerts(), Main: text.P()(text.Text("x"))})
		region := doc.Find(ct.ID(page.AlertsID))
		t.Must(region.Matches(ct.Attr("aria-live", "polite")))
		t.Must(region.Find(ct.Role("alert")).Texts("Refused."))
		t.Must(region.Find(ct.Role("status")).Texts("Saved.", "Saved, with warnings."))
	})
	t.Run("elsewhere", func(t devtest.T) {
		view := ct.View(text.P()(page.ErrorAlert(text.Text("Check the fields."))))
		t.Must(view.Find(ct.Role("alert")).Texts("Check the fields."))
	})
	t.Run("bs alerts keep their role", func(t devtest.T) {
		// Documented: only the page alerts follow the region.
		view := ct.View(page.AlertsContent(bs.AlertWarning()(text.Text("Check the date."))))
		t.Must(view.Find(ct.Role("alert")).Texts("Check the date."))
	})
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
	t.Must(nav.Find(ct.Attr("aria-current", "page")).Count(1))
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

// TestCurrentSection checks the marking of the section that holds the
// current page: the active style with aria-current="true", never "page".
func TestCurrentSection(gt *testing.T) {
	t := devtest.ParallelT(gt)
	items := []page.NavItem{
		{Label: "Settings", Href: "/settings", CurrentSection: true},
		{Label: "Both", Href: "/both", Current: true, CurrentSection: true},
		{Label: "Plain", Href: "/plain"},
	}
	for name, node := range map[string]elem.Node{
		"navbar":  page.Navbar{Items: items},
		"sidebar": page.Sidebar{Sections: []page.SideSection{{Items: items}}},
	} {
		t.Run(name, func(t devtest.T) {
			view := ct.View(node)
			t.Must(view.Valid(rules()...))
			t.Must(view.Find(ct.Role("link", ct.Named("Settings"))).Matches(
				ct.Class("nav-link"), ct.Class("active"), ct.Attr("aria-current", "true"), ct.Attr("href", "/settings"),
			))
			t.Must(view.Find(ct.Role("link", ct.Named("Both"))).Matches(ct.Class("active"), ct.Attr("aria-current", "page")))
			t.Must(view.Find(ct.Role("link", ct.Named("Plain"))).Matches(ct.Class("active").Not(), ct.HasAttr("aria-current").Not()))
			out, err := render(node)
			t.Must(be.NoError(err))
			t.Must(be.Equal(1, strings.Count(out, "aria-current=\"true\"")))
			t.Must(be.Equal(1, strings.Count(out, "aria-current=\"page\"")))
		})
	}
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
		{"notice variant", page.Notice("pink", text.Text("x")), page.ErrInvalid},
		{"layout dialog title", page.Layout{Dialogs: []elem.Node{bs.Dialog{ID: "help"}}}, page.ErrNoName},
		{"layout field label", page.Layout{Main: bs.Field{ID: "f", Name: "f"}}, page.ErrNoName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			_, err := render(c.node)
			t.Must(be.ErrorIs(err, c.want))
		})
	}
}

// TestSentinelsAreBootstraps checks that page and bs classify the same
// mistakes with the same errors, whichever package's sentinel a caller
// checks.
func TestSentinelsAreBootstraps(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.True(page.ErrInvalid == bs.ErrInvalidOption))
	t.Must(be.True(page.ErrNoName == bs.ErrNoName))

	_, err := render(page.Navbar{Items: []page.NavItem{{Label: "A", Href: "/a", Badge: "1", BadgeVariant: "pink"}}})
	t.Must(be.ErrorIs(err, bs.ErrInvalidOption))
	_, err = render(page.Notice("pink", text.Text("x")))
	t.Must(be.ErrorIs(err, page.ErrInvalid))
	_, err = render(page.Header{})
	t.Must(be.ErrorIs(err, bs.ErrNoName))
	_, err = render(bs.Dialog{ID: "help"})
	t.Must(be.ErrorIs(err, page.ErrNoName))
	for name, node := range map[string]elem.Node{
		"navbar":  page.Navbar{ID: "two words"},
		"sidebar": page.Sidebar{ID: "#x"},
	} {
		_, err := render(node)
		t.Mustf(be.ErrorIs(err, page.ErrInvalid), "%s id", name)
		t.Mustf(be.ErrorIs(err, bs.ErrInvalidID), "%s id", name)
		t.Mustf(be.False(errors.Is(err, page.ErrNoName)), "%s id", name)
		t.Mustf(be.False(strings.Contains(err.Error(), "two words") || strings.Contains(err.Error(), "#x")), "%s id error quotes the id", name)
	}

	// ErrInvalid and bs.ErrInvalidID each catch an invalid id in a page,
	// whichever package's component has it, as the ErrInvalid
	// documentation says.
	for name, node := range map[string]elem.Node{
		"page navbar":  page.Layout{Navbar: page.Navbar{ID: "two words"}},
		"page sidebar": page.Layout{Sidebar: page.Sidebar{ID: "two words"}},
		"bs dialog":    page.Layout{Dialogs: []elem.Node{bs.Dialog{ID: "two words", Title: text.Text("T")}}},
		"bs field":     page.Layout{Main: bs.Field{ID: "two words", Name: "f", Label: text.Text("F")}},
		"bs opener":    page.Header{Title: text.Text("T"), Actions: []elem.Node{bs.BtnPrimary(bs.DialogOpen("two words"))(text.Text("Open"))}},
	} {
		_, err := render(node)
		t.Mustf(be.ErrorIs(err, bs.ErrInvalidID), "%s id", name)
		t.Mustf(be.ErrorIs(err, page.ErrInvalid), "%s id", name)
		t.Mustf(be.False(strings.Contains(err.Error(), "two words")), "%s id error quotes the id", name)
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

func TestHeaderCrumbsLabel(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for label, want := range map[string]string{
		"":           page.DefaultCrumbsLabel,
		" \t":        page.DefaultCrumbsLabel,
		"Brotkrümel": "Brotkrümel",
		`"<a>&`:      `"<a>&`,
	} {
		view := ct.View(page.Header{Title: text.Text("T"), CrumbsLabel: label, Crumbs: []page.Crumb{{Label: "Home", Href: "/"}, {Label: "T"}}})
		t.Mustf(view.Find(ct.Tag("nav")).Matches(ct.Attr("aria-label", want)), "label %q", label)
		t.Mustf(view.Find(ct.Role("navigation", ct.Named(want))), "label %q", label)
	}
	out, err := render(page.Header{Title: text.Text("T"), CrumbsLabel: `"<a>&`, Crumbs: []page.Crumb{{Label: "T"}}})
	t.Must(be.NoError(err))
	t.Must(be.Substring(out, `<nav aria-label="&#34;&lt;a&gt;&amp;">`))
	// A label without crumbs renders nothing.
	out, err = render(page.Header{Title: text.Text("T"), CrumbsLabel: "Brotkrümel"})
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<header id="page-header" class="mb-3"><h1 class="h4 mb-0">T</h1></header>`, out))
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
	// Only links to the current page claim to be it; the navbar marks the
	// section that holds it.
	t.Must(parsed.Find(ct.Tag("a").And(ct.Attr("aria-current", "page"))).Each(ct.Attr("href", "/p/home")))
	t.Must(parsed.Find(ct.Attr("aria-current", "true")).Matches(ct.Tag("a"), ct.Attr("href", "/projects"), ct.Class("active")))
}

// TestWebuitestContract checks a real page against the contract that the
// webuitest package enforces for applications, so that a change to either
// side (head order, region ids, defaults) fails here.
func TestWebuitestContract(gt *testing.T) {
	t := devtest.ParallelT(gt)
	themeSet, app := theme.MustAssets(theme.Options{}), appAssets()
	shell := webuitest.DefaultShell(themeSet, app)
	contract := append(webuitest.Rules(webuitest.WithCSRF("token")), webuitest.LayoutRegions())
	full := gallery(themeSet, app)
	plain := full
	layout := full.Body.(page.Layout)
	layout.Sidebar = nil
	plain.Body = layout
	minimal := page.Document{Title: "Tasks", Assets: []*asset.Set{themeSet, app}, Body: page.Layout{Main: text.P()(text.Text("x"))}}
	for name, doc := range map[string]page.Document{"sidebar": full, "no sidebar": plain, "minimal": minimal} {
		t.Run(name, func(t devtest.T) {
			t.Must(webuitest.PageContract(ct.View(doc), shell, contract...))
			out, err := render(doc)
			t.Must(be.NoError(err))
			t.Must(webuitest.PageContract(cthtml.PageString(out), shell, contract...))
		})
	}
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

	// The alerts name their own component, not the bs one they build on,
	// and never the value.
	for _, node := range []elem.Node{page.Notice("pink"), page.AlertsContent(page.Notice("pink"))} {
		_, err = render(node)
		t.Must(be.ErrorIs(err, page.ErrInvalid))
		t.Must(be.ErrorContains(err, "page.Notice: invalid option: variant"))
		t.Must(be.False(strings.Contains(err.Error(), "pink")))
	}
}

// TestDialogsOOB checks the answer that closes a dialog after an htmx
// action: it passes the fragment contract, replaces the dialogs region
// rather than the region's element, and gives the focus to its heading.
func TestDialogsOOB(gt *testing.T) {
	t := devtest.ParallelT(gt)
	dialog := func(id string) elem.Node {
		return bs.Dialog{ID: id, Title: text.Text("Delete the task?"), Footer: form.Form(
			form.Method(form.MethodPost), form.Action("/tasks/"+id), hx2.Post("/tasks/"+id),
			hx2.Target("#task-list"), hx2.Swap(hx2.SwapOuterHTML+" "+page.ShowAlerts),
		)(
			session.CSRFInput("token"),
			bs.BtnDanger(button.Type(button.TypeSubmit))(text.Text("Delete")),
		)}
	}
	answer := ct.View(elem.Bundle{
		section.Section(attr.ID("task-list"))(
			section.H2(page.FocusTarget())(text.Text("Tasks")),
			bs.BtnOutlineDanger(bs.DialogOpen("task-8"))(text.Text("Delete")),
		),
		page.DialogsOOB(dialog("task-8")),
		page.AlertsOOB(page.Notice("", text.Text("Task deleted."))),
	})
	t.Must(webuitest.FragmentContract(answer, webuitest.Rules(webuitest.WithCSRF("token"))...))
	t.Must(answer.Find(ct.ID(page.DialogsID)).Matches(ct.Attr("hx-swap-oob", "innerHTML")))
	t.Must(answer.Find(ct.ID(page.DialogsID)).Find(ct.Tag("dialog")).Matches(ct.ID("task-8"), ct.HasAttr("open").Not()))
	t.Must(answer.Find(ct.Attr("autofocus", "").And(ct.Tag("h2"))).Matches(ct.Attr("tabindex", "-1"), ct.InnerText("Tasks")))
	t.Must(answer.Find(ct.Attr("hx-swap", "outerHTML show:#alerts:top")).Count(1))
	t.Must(be.Equal("show:#alerts:top", page.ShowAlerts))

	// htmx swaps the out-of-band parts first, and ShowAlerts starts from
	// the main content: the form targets the list, which the answer renders
	// in its main part, outside the dialogs region that it replaces.
	t.Must(answer.Find(ct.Tag("form")).Matches(ct.Attr("hx-target", "#task-list")))
	t.Must(answer.Find(ct.ID("task-list")).Matches(ct.HasAttr("hx-swap-oob").Not()))
	t.Must(answer.Find(ct.ID(page.DialogsID)).Find(ct.ID("task-list")).None())
	t.Must(answer.Find(ct.ID(page.AlertsID)).Find(ct.ID("task-list")).None())
}

// pinnedHTMXVersion is the htmx release that the docs of ShowAlerts and
// ShowWindowTop were checked against in a browser. When the theme moves to
// another release, check again which answers scroll with each modifier
// (an outerHTML answer with out-of-band parts only, the delete swap style,
// a target inside a dialog that the answer replaces), then update this
// constant and the docs.
const pinnedHTMXVersion = "2.0.10"

// TestShowModifiersPinnedHTMX ties the documented scroll behavior to the
// htmx release that the theme ships: ShowAlerts looks the region up from
// the swapped content, while htmx resolves the window of ShowWindowTop to
// the body without it, so ShowWindowTop also scrolls after an answer that
// leaves no content in the page.
func TestShowModifiersPinnedHTMX(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal("show:#alerts:top", page.ShowAlerts))
	t.Must(be.Equal("show:window:top", page.ShowWindowTop))
	t.Must(be.Equal(pinnedHTMXVersion, theme.HTMXVersion))

	data, err := fs.ReadFile(theme.FS(), theme.HTMXJS)
	t.Must(be.NoError(err))
	script := string(data)
	t.Must(be.Substring(script, `version:"`+pinnedHTMXVersion+`"`))
	// The show target "window" becomes "body", which the selector lookup
	// resolves to document.body before it reads the swapped content.
	t.Must(be.Substring(script, `if(t.showTarget==="window"){e="body"}`))
	t.Must(be.Substring(script, `else if(r==="body"){e=document.body}`))
}
