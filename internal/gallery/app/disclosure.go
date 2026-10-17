package app

import (
	"net/http"
	"time"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/web"
)

func (a *App) disclosure(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, a.disclosurePage(r))
}

func (a *App) disclosurePage(r *http.Request) elem.Node {
	return a.page(r, view{
		Title: "Disclosure",
		Icon:  bi.ChevronExpand,
		Main: elem.Bundle{
			para("Accordions and dropdown menus built on the details element: the browser opens and closes them, without script."),
			demo("accordions", "Accordions",
				para("Items of an exclusive accordion share a name, so opening one closes the others."),
				bs.DetailsAccordion{ID: "faq", Attrs: attr.Class("mb-4"), Items: []bs.AccordionItem{
					{Header: text.Text("What is a task?"), Body: para("A piece of work with a title, a priority and labels.", attr.Class("mb-0")), Show: true},
					{Header: text.Text("Who sees my notes?"), Body: para("Only the session that wrote them; they end with it.", attr.Class("mb-0"))},
					{Header: text.Text("Does any of this need script?"), Body: para("No: details elements open and close by themselves.", attr.Class("mb-0"))},
				}},
				para("An accordion that is always open lets several items open together; this one is flush, without outer borders."),
				bs.DetailsAccordion{ID: "settings-help", AlwaysOpen: true, Flush: true, Items: []bs.AccordionItem{
					{Header: text.Text("Notifications"), Body: para("Enter a notify email on the task form.", attr.Class("mb-0"))},
					{Header: text.Text("Visibility"), Body: para("Private, team or public.", attr.Class("mb-0"))},
				}},
			),
			demo("dropdowns", "Dropdown menus",
				para("A details dropdown closes when its toggle is clicked again. Its items are links, buttons, dividers and headers; the last item of the first menu asks the server for the time with htmx. The second dropdown sits at the end of the row, and its menu aligns with the end of the toggle: a details menu does not move away from the edge of the window, as Bootstrap's script would."),
				text.P(attr.Class("small text-body-secondary"))(
					text.Text("Server time: "),
					text.Span(attr.ID("disclosure-time"), aria.Live("polite"))(text.Text("not asked yet")),
				),
				row(
					bs.DetailsDropdown{
						Toggle: text.Text("Go to"),
						Items: []elem.Node{
							bs.DropdownHeader()(text.Text("Sections")),
							bs.DropdownItem(text.Href("/components"))(text.Text("Components")),
							bs.DropdownItem(text.Href("/forms"))(text.Text("Forms")),
							bs.DropdownItem(text.Href("/htmx"))(text.Text("htmx")),
							bs.DropdownDivider(),
							bs.DropdownButton(hx2.Get("/disclosure/time"), hx2.Target("#disclosure-time"))(bi.Clock, text.Text(" Ask the server for the time")),
						},
					},
					bs.DetailsDropdown{
						Toggle:      bi.ThreeDots,
						ToggleAttrs: aria.Label("More actions"),
						Variant:     bs.VariantPrimary,
						MenuAttrs:   attr.Class("dropdown-menu-end"),
						Attrs:       attr.Class("ms-auto"),
						Items: []elem.Node{
							bs.DropdownItem(text.Href("/dialogs"))(text.Text("Dialogs")),
							bs.DropdownItem(text.Href("/live"))(text.Text("Live")),
						},
					},
				),
			),
		},
	})
}

// disclosureTime answers the dropdown button with the server time; other
// requests get the page.
func (a *App) disclosureTime(w http.ResponseWriter, r *http.Request) error {
	return web.Choose(w, r, http.StatusOK, text.Text(a.clock()), a.disclosurePage(r))
}

// clock returns the time of day of the gallery's clock, in UTC.
func (a *App) clock() string {
	return a.now().UTC().Format(time.TimeOnly) + " UTC"
}
