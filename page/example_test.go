package page_test

import (
	"testing/fstest"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
)

// A document loads the stylesheets and then the deferred scripts of its
// asset sets, after the htmx configuration. Applications usually pass the
// theme package's set first, then their own.
func ExampleDocument() {
	app := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS: fstest.MapFS{
			"app.css": {Data: []byte("main{margin:0}\n")},
			"app.js":  {Data: []byte("\"use strict\";\n")},
		},
		Files: []asset.File{{Name: "app.css"}, {Name: "app.js"}},
	}))
	core.Dump(page.Document{
		Title:  "Inbox · Tasks",
		Assets: []*asset.Set{app},
		Body:   text.P()(text.Text("Hello")),
	})
	// Output:
	// <!DOCTYPE html><html lang="en" data-bs-theme="dark"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/><title>Inbox · Tasks</title><meta name="color-scheme" content="dark"/><link rel="icon" href="data:,"/><meta name="htmx-config" content="{&#34;allowEval&#34;:false,&#34;allowScriptTags&#34;:false,&#34;attributesToSettle&#34;:[&#34;class&#34;,&#34;width&#34;,&#34;height&#34;],&#34;historyCacheSize&#34;:0,&#34;historyRestoreAsHxRequest&#34;:false,&#34;includeIndicatorStyles&#34;:false,&#34;methodsThatUseUrlParams&#34;:[&#34;get&#34;],&#34;refreshOnHistoryMiss&#34;:true,&#34;responseHandling&#34;:[{&#34;code&#34;:&#34;204&#34;,&#34;swap&#34;:false},{&#34;code&#34;:&#34;[23]..&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;422&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;[45]..&#34;,&#34;swap&#34;:true,&#34;error&#34;:true},{&#34;code&#34;:&#34;...&#34;,&#34;swap&#34;:false}],&#34;selfRequestsOnly&#34;:true}"/><link rel="stylesheet" href="/assets/app/9ee18dcdb205c9d1/app.css" integrity="sha384-M9Q+3dvCRrxarcAhCbMZCt8hEffk11ulIBzQdAH0QRU4BmKBkCWweK5j4NnXsUnA" crossorigin="anonymous"/><script src="/assets/app/9ee18dcdb205c9d1/app.js" defer integrity="sha384-xkfRlW1o/12WPjFaNyRGWMTu+oSgZhrVjoOW5J3gH5+BR9iBauzFwFUOCOKARbI6" crossorigin="anonymous"></script></head><body><p>Hello</p></body></html>
}

// The light color mode and a page icon from an asset set; a document needs
// a title.
func ExampleDocument_options() {
	core.Dump(page.Document{Title: "Tasks", ColorMode: page.ColorModeLight, Lang: "de", Favicon: "/assets/app/icon.svg"})
	core.Dump(page.Document{})
	core.Dump(page.Document{Title: "Tasks", Favicon: "https://cdn.example/icon.svg"})
	// Output:
	// <!DOCTYPE html><html lang="de" data-bs-theme="light"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/><title>Tasks</title><meta name="color-scheme" content="light"/><link rel="icon" href="/assets/app/icon.svg"/><meta name="htmx-config" content="{&#34;allowEval&#34;:false,&#34;allowScriptTags&#34;:false,&#34;attributesToSettle&#34;:[&#34;class&#34;,&#34;width&#34;,&#34;height&#34;],&#34;historyCacheSize&#34;:0,&#34;historyRestoreAsHxRequest&#34;:false,&#34;includeIndicatorStyles&#34;:false,&#34;methodsThatUseUrlParams&#34;:[&#34;get&#34;],&#34;refreshOnHistoryMiss&#34;:true,&#34;responseHandling&#34;:[{&#34;code&#34;:&#34;204&#34;,&#34;swap&#34;:false},{&#34;code&#34;:&#34;[23]..&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;422&#34;,&#34;swap&#34;:true},{&#34;code&#34;:&#34;[45]..&#34;,&#34;swap&#34;:true,&#34;error&#34;:true},{&#34;code&#34;:&#34;...&#34;,&#34;swap&#34;:false}],&#34;selfRequestsOnly&#34;:true}"/></head><body></body></html>
	// ERROR: render: at [0]: page.Document: missing accessible name: title
	// ERROR: render: at [0]: page.Document: invalid option: favicon
}

// The standard layout: the page header first in the main content, then the
// alerts region, the content, the footer, and the dialogs region outside
// every form.
func ExampleLayout() {
	core.Dump(page.Layout{
		Navbar:  page.Navbar{Brand: page.Brand{Label: "Tasks"}},
		Header:  page.Header{Title: text.Text("Inbox")},
		Alerts:  []elem.Node{page.Notice("", text.Text("Task saved."))},
		Main:    text.P()(text.Text("Nothing to do.")),
		Footer:  text.Text("Tasks 1.0"),
		Dialogs: []elem.Node{bs.Dialog{ID: "help", Title: text.Text("Help")}},
	})
	// Output:
	// <div class="d-flex flex-column min-vh-100"><a class="visually-hidden-focusable p-2" href="#main">Skip to main content</a><nav id="navbar" class="navbar navbar-expand bg-body-tertiary border-bottom" aria-label="Main"><div class="container-fluid flex-wrap gap-2"><a class="navbar-brand" href="/">Tasks</a></div></nav><main id="main" class="flex-grow-1"><div class="container py-3"><header id="page-header" class="mb-3"><h1 class="h4 mb-0">Inbox</h1></header><div id="alerts" aria-live="polite"><div class="alert alert-success" role="status">Task saved.</div></div><p>Nothing to do.</p></div></main><footer class="border-top py-3"><div class="container">Tasks 1.0</div></footer></div><div id="dialogs"><dialog id="help" class="modal" popover aria-labelledby="help-title"><div class="modal-dialog"><div class="modal-content"><div class="modal-header"><h2 class="modal-title h5" id="help-title">Help</h2><button class="btn-close" type="button" aria-label="Close" commandfor="help" command="close" popovertarget="help" popovertargetaction="hide" autofocus></button></div></div></div></dialog></div>
}

// With a sidebar, the layout has two columns that stack on narrow screens.
// From Bootstrap's md breakpoint up, the columns reach down to the footer,
// separated by a rule that stacked columns do not draw.
func ExampleLayout_sidebar() {
	core.Dump(page.Layout{
		NoSkipLink: true,
		Sidebar:    page.Sidebar{Label: "Projects", Sections: []page.SideSection{{Items: []page.NavItem{{Label: "Garden", Href: "/p/garden"}}}}},
		Main:       text.P()(text.Text("Garden")),
	})
	// Output:
	// <div class="d-flex flex-column min-vh-100"><div class="container-fluid flex-grow-1 d-flex flex-column"><div class="row flex-grow-1 align-content-start align-content-md-stretch"><div class="col-md-3 col-xl-2 py-3 position-relative"><nav id="sidebar" aria-label="Projects"><ul class="nav nav-pills flex-column mb-3"><li class="nav-item"><a class="nav-link d-flex align-items-center gap-2" href="/p/garden"><span class="text-break">Garden</span></a></li></ul></nav><div class="vr d-none d-md-block position-absolute top-0 end-0 h-100"></div></div><main id="main" class="col-md-9 col-xl-10 py-3"><div id="alerts" aria-live="polite"></div><p>Garden</p></main></div></div></div><div id="dialogs"></div>
}

// A navbar without a toggler: the brand, the links with the current one
// marked, a count badge, and a logout form at the end.
func ExampleNavbar() {
	core.Dump(page.Navbar{
		Brand: page.Brand{Label: "Tasks", Icon: "check2-square"},
		Items: []page.NavItem{
			{Label: "Inbox", Href: "/inbox", Current: true, Badge: "3", BadgeVariant: bs.VariantWarning},
			{Label: "Notes", Href: "/notes", Icon: "journal"},
		},
		End: []elem.Node{
			form.Form(attr.Name("method").Raw("post"), form.Action("/logout"))(
				bs.BtnOutlineSecondary(attr.Class("btn-sm"), button.Type(button.TypeSubmit))(text.Text("Log out")),
			),
		},
	})
	// Output:
	// <nav id="navbar" class="navbar navbar-expand bg-body-tertiary border-bottom" aria-label="Main"><div class="container-fluid flex-wrap gap-2"><a class="navbar-brand" href="/"><i class="bi bi-check2-square me-2" aria-hidden="true"></i>Tasks</a><ul class="navbar-nav nav-underline flex-wrap me-auto"><li class="nav-item"><a class="nav-link active" aria-current="page" href="/inbox">Inbox <span class="badge text-bg-warning">3</span></a></li><li class="nav-item"><a class="nav-link" href="/notes"><i class="bi bi-journal me-1" aria-hidden="true"></i>Notes</a></li></ul><div class="d-flex flex-wrap align-items-center gap-2"><form method="post" action="/logout"><button class="btn btn-outline-secondary btn-sm" type="submit">Log out</button></form></div></div></nav>
}

// Link targets are checked: no script URLs, no characters that browsers
// would drop or reinterpret.
func ExampleNavItem() {
	core.Dump(page.Navbar{Items: []page.NavItem{{Label: "Run", Href: "javascript:alert(1)"}}})
	core.Dump(page.Navbar{Items: []page.NavItem{{Href: "/inbox"}}})
	// Output:
	// ERROR: render: at [0]: page.Navbar item 0: invalid option: href
	// ERROR: render: at [0]: page.Navbar item 0: missing accessible name: label
}

// In a two-level navigation, the navbar marks the section that holds the
// current page (aria-current="true"), and the sidebar the page itself
// (aria-current="page"); both links get the active style.
func ExampleNavItem_currentSection() {
	core.Dump(page.Navbar{Items: []page.NavItem{
		{Label: "Tasks", Href: "/tasks"},
		{Label: "Settings", Href: "/settings", CurrentSection: true},
	}})
	core.Dump(page.Sidebar{Label: "Settings", Sections: []page.SideSection{{Items: []page.NavItem{
		{Label: "Profile", Href: "/settings/profile", Current: true},
		{Label: "Security", Href: "/settings/security"},
	}}}})
	// Output:
	// <nav id="navbar" class="navbar navbar-expand bg-body-tertiary border-bottom" aria-label="Main"><div class="container-fluid flex-wrap gap-2"><ul class="navbar-nav nav-underline flex-wrap me-auto"><li class="nav-item"><a class="nav-link" href="/tasks">Tasks</a></li><li class="nav-item"><a class="nav-link active" href="/settings" aria-current="true">Settings</a></li></ul></div></nav>
	// <nav id="sidebar" aria-label="Settings"><ul class="nav nav-pills flex-column mb-3"><li class="nav-item"><a class="nav-link active d-flex align-items-center gap-2" aria-current="page" href="/settings/profile"><span class="text-break">Profile</span></a></li><li class="nav-item"><a class="nav-link d-flex align-items-center gap-2" href="/settings/security"><span class="text-break">Security</span></a></li></ul></nav>
}

// The page header: a breadcrumb, the page's h1 with actions on its line,
// and tabs below.
func ExampleHeader() {
	core.Dump(page.Header{
		Title:   text.Text("Garden"),
		Crumbs:  []page.Crumb{{Label: "Projects", Href: "/projects"}, {Label: "Garden"}},
		Actions: []elem.Node{bs.BtnPrimary(bs.DialogOpen("new-task"))(text.Text("New task"))},
		Below:   bs.NavTabs()(bs.NavItem()(bs.NavLinkActive(text.Href("/p/garden"))(text.Text("Open")))),
	})
	// Output:
	// <header id="page-header" class="mb-3"><nav aria-label="Breadcrumb"><ol class="breadcrumb mb-2"><li class="breadcrumb-item"><a href="/projects">Projects</a></li><li class="breadcrumb-item active" aria-current="page">Garden</li></ol></nav><div class="d-flex flex-wrap align-items-center gap-2"><h1 class="h4 mb-0 me-auto">Garden</h1><div class="d-flex flex-wrap align-items-center gap-2"><button class="btn btn-primary" type="button" commandfor="new-task" command="show-modal" popovertarget="new-task">New task</button></div></div><ul class="nav nav-tabs"><li class="nav-item"><a class="nav-link active" aria-current="page" href="/p/garden">Open</a></li></ul></header>
}

// The landmark names of a page can be translated, the breadcrumb's too.
func ExampleHeader_crumbsLabel() {
	core.Dump(page.Header{
		Title:       text.Text("Garten"),
		Crumbs:      []page.Crumb{{Label: "Projekte", Href: "/projects"}, {Label: "Garten"}},
		CrumbsLabel: "Brotkrümelnavigation",
	})
	// Output:
	// <header id="page-header" class="mb-3"><nav aria-label="Brotkrümelnavigation"><ol class="breadcrumb mb-2"><li class="breadcrumb-item"><a href="/projects">Projekte</a></li><li class="breadcrumb-item active" aria-current="page">Garten</li></ol></nav><h1 class="h4 mb-0">Garten</h1></header>
}

// A sidebar with a titled section, an icon and a badge.
func ExampleSidebar() {
	core.Dump(page.Sidebar{Label: "Projects", Sections: []page.SideSection{{
		Title: "Active",
		Items: []page.NavItem{
			{Label: "Garden", Href: "/p/garden", Icon: "flower1", Current: true},
			{Label: "Kitchen", Href: "/p/kitchen", Badge: "2"},
		},
	}}})
	// Output:
	// <nav id="sidebar" aria-label="Projects"><h2 class="h6 small text-uppercase text-body-secondary px-3 mb-2">Active</h2><ul class="nav nav-pills flex-column mb-3"><li class="nav-item"><a class="nav-link active d-flex align-items-center gap-2" aria-current="page" href="/p/garden"><i class="bi bi-flower1" aria-hidden="true"></i><span class="text-break">Garden</span></a></li><li class="nav-item"><a class="nav-link d-flex align-items-center gap-2" href="/p/kitchen"><span class="text-break">Kitchen</span> <span class="badge text-bg-secondary ms-auto">2</span></a></li></ul></nav>
}

// Alerts that come with the page: an error interrupts screen readers, a
// notice does not.
func ExampleNotice() {
	core.Dump(page.ErrorAlert(text.Text("The task could not be saved.")))
	core.Dump(page.Notice("", text.Text("Task saved.")))
	core.Dump(page.Notice(bs.VariantInfo, text.Text("The import runs in the background.")))
	core.Dump(page.Notice("pink", text.Text("Saved.")))
	// Output:
	// <div class="alert alert-danger" role="alert">The task could not be saved.</div>
	// <div class="alert alert-success" role="status">Task saved.</div>
	// <div class="alert alert-info" role="status">The import runs in the background.</div>
	// ERROR: render: at [0]: page.Notice: invalid option: variant
}

// An htmx response swaps its main fragment and, out of band, the content of
// the alerts region. The region, a live region already in the page,
// announces the notice, which therefore has no role of its own.
func ExampleAlertsOOB() {
	core.Dump(elem.Bundle{
		text.P(attr.ID("task-7"))(text.Text("Water the plants (done)")),
		page.AlertsOOB(page.Notice("", text.Text("Task completed."))),
	})
	// Output:
	// <p id="task-7">Water the plants (done)</p><div id="alerts" hx-swap-oob="innerHTML"><div class="alert alert-success">Task completed.</div></div>
}

// An out-of-band swap does not scroll. The element that makes the request
// asks for the scroll to the alerts region with the ShowAlerts modifier.
func ExampleAlertsOOB_scroll() {
	core.Dump(form.Form(form.Method(form.MethodPost), form.Action("/tasks/7/done"),
		hx2.Post("/tasks/7/done"), hx2.Target("#task-7"), hx2.Swap(hx2.SwapOuterHTML+" "+page.ShowAlerts))(
		session.CSRFInput("dG9rZW4"),
		bs.BtnSuccess(button.Type(button.TypeSubmit), attr.ID("task-7-done"))(text.Text("Done")),
	))
	// Output:
	// <form method="post" action="/tasks/7/done" hx-post="/tasks/7/done" hx-target="#task-7" hx-swap="outerHTML show:#alerts:top"><input type="hidden" name="csrf" value="dG9rZW4"/><button class="btn btn-success" type="submit" id="task-7-done">Done</button></form>
}

// An answer that htmx swaps into the alerts region (its hx-target, or an
// HX-Retarget header, names the region): the region announces the alerts,
// which have no role of their own.
func ExampleAlertsContent() {
	core.Dump(page.AlertsContent(page.ErrorAlert(text.Text("The task list is full. Delete a task first."))))
	// Output:
	// <div class="alert alert-danger">The task list is full. Delete a task first.</div>
}

// A dialog confirms the deletion of a task, and its form deletes the task
// through htmx. The answer swaps the list, closes the dialog by rendering
// the dialogs region again without it, shows a notice, and gives the focus
// to the list's heading: the button that had it left with the dialog.
func ExampleDialogsOOB() {
	type task struct{ id, title string }
	deleteDialog := func(t task) elem.Node {
		id := "delete-task-" + t.id
		action := "/tasks/" + t.id + "/delete"
		return bs.Dialog{ID: id, Title: text.Text("Delete " + t.title + "?"), Footer: elem.Bundle{
			bs.BtnSecondary(bs.DialogClose(id))(text.Text("Keep")),
			form.Form(form.Method(form.MethodPost), form.Action(action),
				hx2.Post(action), hx2.Target("#task-list"), hx2.Swap(hx2.SwapOuterHTML+" "+page.ShowAlerts))(
				session.CSRFInput("dG9rZW4"),
				bs.BtnDanger(button.Type(button.TypeSubmit))(text.Text("Delete")),
			),
		}}
	}
	// taskList renders the list; focus marks its heading as the focus
	// target, in htmx answers only.
	taskList := func(focus bool, tasks ...task) elem.Node {
		items := make([]elem.Node, 0, len(tasks))
		for _, t := range tasks {
			items = append(items, list.LI()(text.Text(t.title+" "),
				bs.BtnOutlineDanger(bs.DialogOpen("delete-task-"+t.id))(text.Text("Delete"))))
		}
		return section.Section(attr.ID("task-list"))(
			section.H2(attr.If(focus, page.FocusTarget()))(text.Text("Tasks")),
			list.UL()(items...),
		)
	}

	// The answer to the deletion of task 7, with task 8 left.
	left := task{id: "8", title: "Sweep the porch"}
	core.Dump(elem.Bundle{
		taskList(true, left),
		page.DialogsOOB(deleteDialog(left)),
		page.AlertsOOB(page.Notice(bs.VariantInfo, text.Text("Task deleted."))),
	})
	// Output:
	// <section id="task-list"><h2 tabindex="-1" autofocus>Tasks</h2><ul><li>Sweep the porch <button class="btn btn-outline-danger" type="button" commandfor="delete-task-8" command="show-modal" popovertarget="delete-task-8">Delete</button></li></ul></section><div id="dialogs" hx-swap-oob="innerHTML"><dialog id="delete-task-8" class="modal" popover aria-labelledby="delete-task-8-title"><div class="modal-dialog"><div class="modal-content"><div class="modal-header"><h2 class="modal-title h5" id="delete-task-8-title">Delete Sweep the porch?</h2><button class="btn-close" type="button" aria-label="Close" commandfor="delete-task-8" command="close" popovertarget="delete-task-8" popovertargetaction="hide" autofocus></button></div><div class="modal-footer"><button class="btn btn-secondary" type="button" commandfor="delete-task-8" command="close" popovertarget="delete-task-8" popovertargetaction="hide">Keep</button><form method="post" action="/tasks/8/delete" hx-post="/tasks/8/delete" hx-target="#task-list" hx-swap="outerHTML show:#alerts:top"><input type="hidden" name="csrf" value="dG9rZW4"/><button class="btn btn-danger" type="submit">Delete</button></form></div></div></div></dialog></div><div id="alerts" hx-swap-oob="innerHTML"><div class="alert alert-info">Task deleted.</div></div>
}

// FocusTarget lets an element that is not a control take the focus after
// an htmx swap.
func ExampleFocusTarget() {
	core.Dump(section.H2(attr.ID("tasks-title"), page.FocusTarget())(text.Text("Tasks")))
	// Output:
	// <h2 id="tasks-title" tabindex="-1" autofocus>Tasks</h2>
}
