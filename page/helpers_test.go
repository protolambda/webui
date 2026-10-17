package page_test

import (
	"context"
	"strings"
	"testing/fstest"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
)

// render renders node without indentation.
func render(node elem.Node) (string, error) {
	var b strings.Builder
	err := core.Render(context.Background(), node, &b)
	return b.String(), err
}

// walk calls fn for every element below n, in document order.
func walk(n ct.Node, fn func(ct.Node)) {
	for c := range n.Children() {
		if c.Kind() == ct.KindElement {
			fn(c)
			walk(c, fn)
		}
	}
}

// noInlineCode forbids what a strict Content-Security-Policy blocks: style
// elements and attributes, scripts without src, and javascript: URLs.
var noInlineCode = ct.CustomRule("noInlineCode()", func(root ct.Node) []string {
	var out []string
	walk(root, func(n ct.Node) {
		if n.Tag() == "style" {
			out = append(out, "style element at "+n.Location())
		}
		if _, ok := n.Attr("style"); ok {
			out = append(out, "style attribute at "+n.Location())
		}
		if _, ok := n.Attr("src"); n.Tag() == "script" && !ok {
			out = append(out, "inline script at "+n.Location())
		}
		for name, value := range n.Attrs() {
			if strings.HasPrefix(name, "hx-on") {
				out = append(out, "hx-on attribute at "+n.Location())
			}
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "javascript:") {
				out = append(out, "javascript: URL in "+name+" at "+n.Location())
			}
		}
	})
	return out
})

// allButtonsHaveType requires every button, inside a form or not, to
// state its type.
var allButtonsHaveType = ct.CustomRule("allButtonsHaveType()", func(root ct.Node) []string {
	var out []string
	walk(root, func(n ct.Node) {
		if _, ok := n.Attr("type"); n.Tag() == "button" && !ok {
			out = append(out, "button without type at "+n.Location())
		}
	})
	return out
})

// rules are the document rules every page of this package satisfies.
func rules() []ct.Rule {
	return []ct.Rule{
		ct.UniqueIDs(), ct.ARIAReferences(), ct.LabelReferences(), ct.LocalTargets(),
		ct.ImagesHaveAlt(), ct.ButtonsHaveType(), ct.NoInlineHandlers(),
		noInlineCode, allButtonsHaveType,
	}
}

// appAssets is a small application asset set.
func appAssets() *asset.Set {
	return asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS: fstest.MapFS{
			"app.css": {Data: []byte("main{margin:0}\n")},
			"app.js":  {Data: []byte("\"use strict\";\n")},
		},
		Files: []asset.File{{Name: "app.css"}, {Name: "app.js"}},
	}))
}

// logout is a logout form for the navbar's end slot.
func logout() elem.Node {
	return form.Form(attr.Name("method").Raw("post"), form.Action("/logout"))(
		session.CSRFInput("token"),
		bs.BtnOutlineSecondary(attr.Class("btn-sm"), button.Type(button.TypeSubmit))(text.Text("Log out")),
	)
}

// gallery is a full page with every component and all optional parts, on
// a project page of a two-level navigation: the navbar marks the section
// (Projects), the sidebar and the tabs the page itself (/p/home).
func gallery(sets ...*asset.Set) page.Document {
	return page.Document{
		Title:  "Home · Tasks",
		Assets: sets,
		Body: page.Layout{
			Navbar: page.Navbar{
				Brand: page.Brand{Label: "Tasks", Icon: "check2-square"},
				Items: []page.NavItem{
					{Label: "Inbox", Href: "/inbox", Icon: "inbox", Badge: "3", BadgeVariant: bs.VariantWarning},
					{Label: "Projects", Href: "/projects", CurrentSection: true},
					{Label: "Notes", Href: "/notes"},
				},
				End: []elem.Node{logout()},
			},
			Sidebar: page.Sidebar{Label: "Projects", Sections: []page.SideSection{
				{Title: "Active", Items: []page.NavItem{{Label: "Home", Href: "/p/home", Current: true, Badge: "2"}, {Label: "Garden", Href: "/p/garden", Icon: "flower1"}}},
				{Items: []page.NavItem{{Label: "Archive", Href: "/p/archive"}}},
			}},
			Header: page.Header{
				Title:   text.Text("Home"),
				Icon:    "house",
				Crumbs:  []page.Crumb{{Label: "Projects", Href: "/projects"}, {Label: "Home"}},
				Actions: []elem.Node{bs.BtnPrimary(bs.DialogOpen("new-task"))(text.Text("New task"))},
				Below:   bs.NavTabs()(bs.NavItem()(bs.NavLinkActive(text.Href("/p/home"))(text.Text("Open")))),
			},
			Alerts: []elem.Node{page.Notice("", text.Text("Task saved.")), page.ErrorAlert(text.Text("The other task could not be saved."))},
			Main:   text.P()(text.Text("Nothing to do.")),
			Footer: text.Small()(text.Text("Tasks")),
			Dialogs: []elem.Node{bs.Dialog{
				ID:    "new-task",
				Title: text.Text("New task"),
				Body: form.Form(attr.Name("method").Raw("post"), form.Action("/tasks"))(
					session.CSRFInput("token"),
					bs.Field{ID: "task-title", Name: "title", Label: text.Text("Title"), Required: true},
					bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Add")),
				),
			}},
		},
	}
}
