package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/ba"
	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/visible"
	"github.com/protolambda/webui/web"
)

// tasksPerPage is the page size of the paginated task table.
const tasksPerPage = 3

var variants = []bs.Variant{
	bs.VariantPrimary, bs.VariantSecondary, bs.VariantSuccess, bs.VariantDanger,
	bs.VariantWarning, bs.VariantInfo, bs.VariantLight, bs.VariantDark,
}

// anchor is a section of a page, for the "On this page" links of a
// sidebar.
type anchor struct{ ID, Label string }

// componentPage is a page of the components section.
type componentPage struct {
	Path, Label, Blurb string
	Icon               bi.Icon
	Anchors            []anchor
}

// The pages of the components section, in the order of its sidebar. The
// overview comes first.
const (
	componentsOverview = iota
	componentsControls
	componentsContent
	componentsFeedback
)

var componentPages = []componentPage{
	componentsOverview: {Path: "/components", Label: "Overview", Icon: bi.Grid3x3Gap},
	componentsControls: {
		Path: "/components/controls", Label: "Controls", Icon: bi.ToggleOn,
		Blurb:   "Buttons, button groups, badges and alerts.",
		Anchors: []anchor{{"buttons", "Buttons"}, {"badges", "Badges"}, {"alert-styles", "Alerts"}},
	},
	componentsContent: {
		Path: "/components/content", Label: "Content", Icon: bi.CardText,
		Blurb:   "Cards, list groups, a data table with pagination, breadcrumbs, and untrusted text.",
		Anchors: []anchor{{"cards", "Cards"}, {"lists", "List groups"}, {"table", "Table and pagination"}, {"breadcrumbs", "Breadcrumbs"}, {"untrusted", "Untrusted text"}},
	},
	componentsFeedback: {
		Path: "/components/feedback", Label: "Feedback", Icon: bi.Bell,
		Blurb:   "Spinners, progress bars, icons, and toasts, one of them appended by an out-of-band swap.",
		Anchors: []anchor{{"feedback", "Spinners and progress"}, {"icons", "Icons"}, {"toast-demo", "Toasts"}},
	},
}

func (a *App) home(w http.ResponseWriter, r *http.Request) error {
	cards := make([]elem.Node, 0, len(gallerySections))
	for _, s := range gallerySections {
		cards = append(cards, bs.ColMD(6, attr.Class("col-xl-4 mb-3"))(bs.Card{
			Attrs: attr.Class("h-100"),
			Body: bs.CardBody{
				Title:      elem.Bundle{s.Icon, text.Text(" " + s.Label)},
				TitleLevel: 2,
				Text:       text.Text(s.Blurb),
				Content:    link(s.Path, "Open "+s.Label, attr.Class("card-link")),
			},
		}))
	}
	return web.Render(w, r, http.StatusOK, a.page(r, view{
		Title: siteName,
		Icon:  bi.Grid,
		Main: elem.Bundle{
			para("Every component of webui on one site, rendered by the server: the pages need no application JavaScript, load every asset from this origin with integrity values, and pass a strict content security policy.", attr.Class("lead")),
			bs.Row()(cards...),
		},
	}))
}

// componentsView returns the view of the page of the components section
// with the index current: the sidebar lists the pages of the section, with
// the current one marked, and the sections of the page.
//
// The sidebar marks the current page's link Current only on the page's own
// URL; a page that renders it for another URL (the answer to a toast
// request without htmx) marks it CurrentSection, as it is not that page.
func (a *App) componentsView(r *http.Request, current int, main ...elem.Node) view {
	cp := componentPages[current]
	pages := make([]page.NavItem, 0, len(componentPages))
	for i, p := range componentPages {
		pages = append(pages, page.NavItem{
			Label: p.Label, Href: p.Path, Icon: p.Icon,
			Current:        i == current && r.URL.Path == p.Path,
			CurrentSection: i == current && r.URL.Path != p.Path,
		})
	}
	sections := []page.SideSection{{Title: "Components", Items: pages}}
	if len(cp.Anchors) > 0 {
		anchors := make([]page.NavItem, 0, len(cp.Anchors))
		for _, an := range cp.Anchors {
			anchors = append(anchors, page.NavItem{Label: an.Label, Href: "#" + an.ID})
		}
		sections = append(sections, page.SideSection{Title: "On this page", Items: anchors})
	}
	v := view{
		Title:   cp.Label,
		Icon:    cp.Icon,
		Parents: []page.Crumb{{Label: "Components", Href: "/components"}},
		Sidebar: page.Sidebar{Label: "Components", Sections: sections},
		Main:    elem.Bundle(main),
	}
	if current == componentsOverview {
		v.Title, v.Parents = "Components", nil
	}
	return v
}

// components renders the overview of the components section.
func (a *App) components(w http.ResponseWriter, r *http.Request) error {
	cards := make([]elem.Node, 0, len(componentPages)-1)
	for _, p := range componentPages[componentsOverview+1:] {
		cards = append(cards, bs.ColMD(4, attr.Class("mb-3"))(bs.Card{
			Attrs: attr.Class("h-100"),
			Body: bs.CardBody{
				Title:      elem.Bundle{p.Icon, text.Text(" " + p.Label)},
				TitleLevel: 2,
				Text:       text.Text(p.Blurb),
				Content:    link(p.Path, "Open "+p.Label, attr.Class("card-link")),
			},
		}))
	}
	return web.Render(w, r, http.StatusOK, a.page(r, a.componentsView(r, componentsOverview,
		para("The Bootstrap components of package bs, in three pages. The sidebar lists them: on each page, its link is the current page and the navbar's Components link the current section."),
		bs.Row()(cards...),
	)))
}

func (a *App) componentsControls(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, a.page(r, a.componentsView(r, componentsControls,
		buttonsDemo(), badgesDemo(), alertsDemo(),
	)))
}

func (a *App) componentsContent(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, a.page(r, a.componentsView(r, componentsContent,
		cardsDemo(), listsDemo(), a.tableDemo(r), breadcrumbsDemo(), untrustedDemo(),
	)))
}

func (a *App) componentsFeedback(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, a.feedbackPage(r, nil))
}

// feedbackPage renders the feedback page; extra is a toast to show in the
// toast container, for the answer to a plain request for a toast.
func (a *App) feedbackPage(r *http.Request, extra elem.Node) elem.Node {
	return a.page(r, a.componentsView(r, componentsFeedback, feedbackDemo(), iconsDemo(), a.toastsDemo(extra)))
}

// row lays out demo items in a wrapping row.
func row(items ...elem.Node) elem.Node {
	return div.Div(attr.Class("d-flex flex-wrap align-items-center gap-2 mb-3"))(items...)
}

func buttonsDemo() elem.Node {
	solid := make([]elem.Node, 0, len(variants)+1)
	outline := make([]elem.Node, 0, len(variants))
	for _, v := range variants {
		solid = append(solid, bs.BtnOf(v)(text.Text(capitalize(string(v)))))
		outline = append(outline, bs.BtnOutlineOf(v)(text.Text(capitalize(string(v)))))
	}
	solid = append(solid, bs.BtnLink()(text.Text("Link")))
	return demo("buttons", "Buttons",
		row(solid...),
		row(outline...),
		row(
			bs.BtnPrimary(ba.BtnSM())(text.Text("Small")),
			bs.BtnPrimary()(text.Text("Default")),
			bs.BtnPrimary(ba.BtnLG())(text.Text("Large")),
			bs.BtnSecondary(button.Disabled())(text.Text("Disabled")),
			bs.BtnOutlineSecondary(aria.Label("Settings"))(bi.Gear),
		),
		row(bs.BtnToolbar(aria.Label("Text formatting"))(
			bs.BtnGroup(aria.Label("Style"), attr.Class("me-2"))(
				bs.BtnOutlineSecondary()(text.Strong()(text.Text("Bold"))),
				bs.BtnOutlineSecondary()(text.EM()(text.Text("Italic"))),
				bs.BtnOutlineSecondary()(text.U()(text.Text("Underline"))),
			),
			bs.BtnGroup(aria.Label("Alignment"))(
				bs.BtnOutlineSecondary(aria.Label("Align left"))(bi.TextLeft),
				bs.BtnOutlineSecondary(aria.Label("Center"))(bi.TextCenter),
				bs.BtnOutlineSecondary(aria.Label("Align right"))(bi.TextRight),
			),
		)),
	)
}

func badgesDemo() elem.Node {
	badges := make([]elem.Node, 0, len(variants))
	for _, v := range variants {
		badges = append(badges, bs.BadgeOf(v)(text.Text(capitalize(string(v)))))
	}
	return demo("badges", "Badges",
		row(badges...),
		row(
			bs.BtnPrimary()(text.Text("Inbox "), bs.BadgeLight()(text.Text("4"))),
			text.Span()(text.Text("Release "), bs.BadgeSuccess()(text.Text("v1.2"))),
		),
	)
}

func alertsDemo() elem.Node {
	return demo("alert-styles", "Alerts",
		bs.AlertPrimary(aria.Role("note"))(text.Text("A primary alert, for information that stands out.")),
		bs.AlertWarning(aria.Role("note"))(bi.ExclamationTriangle, text.Text(" A warning alert with a decorative icon.")),
		bs.AlertInfo(aria.Role("note"))(text.Strong()(text.Text("Note: ")), text.Text("alerts take any content, such as "), link("/forms", "a link"), text.Text(".")),
		page.Notice("", text.Text("page.Notice renders the outcome of an action (role status).")),
		page.ErrorAlert(text.Text("page.ErrorAlert renders a refused action (role alert).")),
		para("The demo alerts above carry role=\"note\" instead of their default alert role, so that screen readers do not announce a page full of them as urgent.", attr.Class("small text-body-secondary")),
	)
}

func cardsDemo() elem.Node {
	return demo("cards", "Cards", bs.Row()(
		bs.ColMD(4, attr.Class("mb-3"))(bs.Card{Attrs: attr.Class("h-100"), Body: text.Text("A card with a plain body.")}),
		bs.ColMD(4, attr.Class("mb-3"))(bs.Card{
			Attrs:  attr.Class("h-100"),
			Header: text.Text("Header"),
			Body:   bs.CardBody{Title: text.Text("Card title"), TitleLevel: 3, Text: text.Text("A card with a header, a structured body and a footer.")},
			Footer: text.Small(attr.Class("text-body-secondary"))(text.Text("Updated a minute ago")),
		}),
		bs.ColMD(4, attr.Class("mb-3"))(bs.Card{
			Attrs: attr.Class("h-100"),
			Body: bs.CardBody{
				Title: text.Text("Project"), TitleLevel: 3,
				Subtitle: text.Text("Card subtitle"), SubtitleLevel: 4,
				Text:    text.Text("Cards hold links too."),
				Content: elem.Bundle{link("/htmx", "Tasks", attr.Class("card-link")), link("/notes", "Notes", attr.Class("card-link"))},
			},
		}),
	))
}

func listsDemo() elem.Node {
	return demo("lists", "List groups", bs.Row()(
		bs.ColMD(6, attr.Class("mb-3"))(bs.ListGroup()(
			bs.ListGroupItemActive()(text.Text("Inbox")),
			bs.ListGroupItem(attr.Class("d-flex justify-content-between align-items-center"))(
				text.Text("Drafts"), bs.BadgePrimary()(text.Text("3")),
			),
			bs.ListGroupItem()(text.Text("Archive")),
		)),
		bs.ColMD(6, attr.Class("mb-3"))(bs.ListGroupFlush()(
			bs.ListGroupItem()(text.Text("A flush list group")),
			bs.ListGroupItem()(text.Text("without outer borders")),
			bs.ListGroupItem()(text.Text("for lists inside cards")),
		)),
	))
}

// pageNumber returns the page of a paginated list that r asks for, between
// 1 and pages; anything else (missing, malformed, out of range) means 1.
func pageNumber(r *http.Request, pages int) int {
	n, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || n < 1 || n > pages {
		return 1
	}
	return n
}

func (a *App) tableDemo(r *http.Request) elem.Node {
	all := a.tasks.list(filterAll)
	pages := max(1, (len(all)+tasksPerPage-1)/tasksPerPage)
	n := pageNumber(r, pages)
	shown := all[min(len(all), (n-1)*tasksPerPage):min(len(all), n*tasksPerPage)]

	pagination := make([]elem.Node, 0, pages+2)
	pageHref := func(i int) string { return "/components/content?page=" + strconv.Itoa(i) + "#table" }
	if n > 1 {
		pagination = append(pagination, bs.PageItem()(bs.PageLink(text.Href(pageHref(n-1)))(text.Text("Previous"))))
	} else {
		pagination = append(pagination, bs.PageItemDisabled()(bs.PageLink()(text.Text("Previous"))))
	}
	for i := 1; i <= pages; i++ {
		label := text.Text(strconv.Itoa(i))
		if i == n {
			pagination = append(pagination, bs.PageItemActive()(bs.PageLinkCurrent(text.Href(pageHref(i)))(label)))
		} else {
			pagination = append(pagination, bs.PageItem()(bs.PageLink(text.Href(pageHref(i)))(label)))
		}
	}
	if n < pages {
		pagination = append(pagination, bs.PageItem()(bs.PageLink(text.Href(pageHref(n+1)))(text.Text("Next"))))
	} else {
		pagination = append(pagination, bs.PageItemDisabled()(bs.PageLink()(text.Text("Next"))))
	}

	return demo("table", "Table and pagination",
		taskTable(text.Text(fmt.Sprintf("Tasks, page %d of %d", n, pages)), shown, nil),
		bs.PaginationLabeled("Task pages")(pagination...),
	)
}

// taskTable renders tasks as a DataTable; action, when set, renders the
// cell of an extra actions column for each task. Task titles are text that
// visitors chose: they show through visible.Text.
func taskTable(caption elem.Node, tasks []task, action func(task) elem.Node) elem.Node {
	columns := []bs.Column{
		{Header: text.Text("Task"), RowHeader: true},
		{Header: text.Text("Priority")},
		{Header: text.Text("Labels")},
		{Header: text.Text("State")},
	}
	if action != nil {
		columns = append(columns, bs.Column{Header: text.Text("Actions"), HeaderHidden: true, Attrs: attr.Class("text-end")})
	}
	rows := make([]bs.TableRow, 0, len(tasks))
	for _, t := range tasks {
		cells := []elem.Node{visible.Text(t.Title), priorityBadge(t.Priority), text.Text(strings.Join(t.Labels, ", ")), stateText(t.Done)}
		if action != nil {
			cells = append(cells, action(t))
		}
		rows = append(rows, bs.TableRow{Cells: cells})
	}
	return bs.DataTable{
		Caption: caption,
		Columns: columns,
		Rows:    rows,
		Empty:   text.Text("No tasks match."),
		Small:   true,
		Hover:   true,
	}
}

func priorityBadge(p priority) elem.Node {
	v := bs.VariantSecondary
	switch p {
	case priorityHigh:
		v = bs.VariantDanger
	case priorityLow:
		v = bs.VariantInfo
	}
	return bs.BadgeOf(v)(text.Text(string(p)))
}

// stateText renders the state of a task, its icon kept on the line of its
// word in a narrow column.
func stateText(done bool) elem.Node {
	if done {
		return text.Span(attr.Class("text-nowrap"))(bi.CheckCircle.With(attr.Class("text-success")), text.Text(" Done"))
	}
	return text.Span(attr.Class("text-nowrap"))(bi.Circle, text.Text(" Open"))
}

func breadcrumbsDemo() elem.Node {
	return demo("breadcrumbs", "Breadcrumbs",
		para("The page header above renders its breadcrumb with page.Header; bs.BreadcrumbLabeled renders one anywhere."),
		bs.BreadcrumbLabeled("Example path")(
			bs.BreadcrumbItem()(link("/", "Gallery")),
			bs.BreadcrumbItem()(link("/htmx", "Tasks")),
			bs.BreadcrumbActive()(text.Text("Write the release notes")),
		),
	)
}

// untrustedSamples are values that other users could choose, written with
// escapes in the source: each shows something other than what it is.
var untrustedSamples = []struct{ Label, Value string }{
	{"A right-to-left override", "invoice\u202Efdp.exe"},
	{"A zero-width space", "ad\u200Bmin"},
	{"Two spaces", "two  spaces"},
	{"A no-break space", "Ada\u00A0Lovelace"},
	{"An emoji sequence", "\U0001F469\u200D\U0001F4BB team"},
	{"A look-alike letter", "\u0430dmin"},
}

// untrustedDemo shows untrusted text three ways: as plain text (escaped
// against markup only, isolated in a bdi element so that an override does
// not reorder the table), through visible.Text and through visible.Code.
func untrustedDemo() elem.Node {
	rows := make([]bs.TableRow, 0, len(untrustedSamples))
	for _, s := range untrustedSamples {
		rows = append(rows, bs.TableRow{Cells: []elem.Node{
			text.Text(s.Label),
			text.Bdi()(text.Text(s.Value)),
			visible.Text(s.Value),
			visible.Code(s.Value),
		}})
	}
	return demo("untrusted", "Untrusted text",
		para("HTML escaping keeps text that other users chose from becoming markup, but not from misleading a reader: a direction override reverses the text after it, and invisible or collapsing characters make two different names look the same. Package visible escapes what does not show as itself and adds a marker. visible.Text is for names and titles: it keeps emoji sequences and no-break spaces between words. visible.Code is strict, for values read character by character. Look-alike letters are visible letters, so neither escapes them."),
		bs.DataTable{
			Caption: text.Text("The same values, shown three ways"),
			Columns: []bs.Column{
				{Header: text.Text("Sample"), RowHeader: true},
				{Header: text.Text("As text")},
				{Header: text.Text("visible.Text")},
				{Header: text.Text("visible.Code")},
			},
			Rows:  rows,
			Small: true,
		},
	)
}

func feedbackDemo() elem.Node {
	progress := func(label string, value int, width string, bar ...attr.Node) elem.Node {
		return div.Div(attr.Class("mb-2"))(
			text.Small(attr.Class("text-body-secondary"))(text.Text(fmt.Sprintf("%s: %d%%", label, value))),
			bs.Progress(aria.Label(label), bs.ProgressValue(value, 0, 100))(bs.ProgressBar(append([]attr.Node{attr.Class(width)}, bar...)...)()),
		)
	}
	return demo("feedback", "Spinners and progress",
		row(
			bs.Spinner()(),
			bs.SpinnerGrow(attr.Class("text-primary"))(),
			bs.Spinner(attr.Class("spinner-border-sm"))(text.Text("Saving...")),
		),
		progress("Upload", 25, "w-25"),
		progress("Import", 50, "w-50"),
		progress("Sync", 75, "w-75", attr.Class("progress-bar-striped")),
		progress("Backup", 100, "w-100", attr.Class("bg-success")),
	)
}

func iconsDemo() elem.Node {
	return demo("icons", "Icons",
		para("Decorative icons sit next to text that says the same, and are hidden from assistive technology:"),
		row(
			text.Span()(bi.CheckCircle, text.Text(" Done")),
			text.Span()(bi.Clock, text.Text(" Waiting")),
			text.Span()(bi.Star, text.Text(" Starred")),
		),
		para("Icons that stand alone carry a label, or name the control they sit in:"),
		row(
			bi.ExclamationTriangle.Label("Warning", attr.Class("text-warning fs-4")),
			bi.InfoCircle.Label("Information", attr.Class("text-info fs-4")),
			bs.BtnOutlineDanger(aria.Label("Delete"))(bi.Trash),
		),
	)
}

func (a *App) toastsDemo(extra elem.Node) elem.Node {
	return demo("toast-demo", "Toasts",
		para("A toast rendered by the server is visible without script. The button appends another one to the toast container, a polite live region, with an out-of-band swap."),
		bs.Toast{
			Header: elem.Bundle{bi.Bell, text.Strong(attr.Class("ms-2 me-auto"))(text.Text("Gallery"))},
			Body:   text.Text("This toast was rendered with the page."),
			Shown:  true,
			Polite: true,
			Close:  dismissButton(),
			Attrs:  attr.Class("mb-3"),
		},
		text.A(text.Href("/components/toast"), attr.Class("btn btn-primary"), hx2.Get("/components/toast"), hx2.Swap(hx2.SwapNone))(
			bi.Bell, text.Text(" Show a toast"),
		),
		bs.ToastContainer(attr.ID("toasts"), attr.Class("position-fixed bottom-0 end-0 p-3"))(elem.If(extra != nil, extra)),
	)
}

// dismissButton is the close button of a toast without Bootstrap's script:
// htmx deletes the toast once the (empty) answer arrives.
func dismissButton() elem.Node {
	return bs.CloseButton(aria.Label("Dismiss"), hx2.Get("/components/toast/dismiss"), hx2.Target("closest .toast"), hx2.Swap(hx2.SwapDelete))
}

// toast answers the button of the toasts demo: htmx gets a toast appended
// to the container by an out-of-band swap; other requests get the page with
// the toast in the container.
func (a *App) toast(w http.ResponseWriter, r *http.Request) error {
	n := a.toasts.Add(1)
	t := bs.Toast{
		Header: elem.Bundle{bi.Bell, text.Strong(attr.Class("ms-2 me-auto"))(text.Text("Toast " + strconv.FormatInt(n, 10)))},
		Body:   text.Text("Appended by an out-of-band swap."),
		Shown:  true,
		Close:  dismissButton(),
	}
	fragment := bs.ToastContainer(attr.ID("toasts"), hx2.SwapOOB(hx2.SwapBeforeEnd))(t)
	return web.Choose(w, r, http.StatusOK, fragment, a.feedbackPage(r, t))
}

// dismissToast answers the close button of a toast with an empty fragment,
// on which htmx deletes the toast.
func (a *App) dismissToast(w http.ResponseWriter, r *http.Request) error {
	if !web.IsHTMX(r) {
		web.Redirect(w, r, "/components/feedback#toast-demo")
		return nil
	}
	return web.Render(w, r, http.StatusOK, elem.Noop())
}
