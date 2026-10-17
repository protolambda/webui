package bs_test

import (
	"fmt"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
)

// Buttons

func ExampleBtnPrimary() {
	core.Dump(bs.BtnPrimary()(text.Text("Click me")))
	// Output: <button class="btn btn-primary" type="button">Click me</button>
}

// A button submits its form only with an explicit type.
func ExampleBtnPrimary_submit() {
	core.Dump(bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Save")))
	// Output: <button class="btn btn-primary" type="submit">Save</button>
}

func ExampleBtnOutlinePrimary() {
	core.Dump(bs.BtnOutlinePrimary()(text.Text("Outline")))
	// Output: <button class="btn btn-outline-primary" type="button">Outline</button>
}

// BtnOf takes the variant as a value, e.g. from a status of the data.
func ExampleBtnOf() {
	core.Dump(bs.BtnOf(bs.VariantDanger)(text.Text("Delete")))
	core.Dump(bs.BtnOutlineOf(bs.VariantSuccess)(text.Text("Approve")))
	core.Dump(bs.BtnOf("purple")(text.Text("Unknown")))
	// Output:
	// <button class="btn btn-danger" type="button">Delete</button>
	// <button class="btn btn-outline-success" type="button">Approve</button>
	// ERROR: render: at button[0]: attribute: bs.BtnOf: invalid option: unknown variant
}

func ExampleBtnGroup() {
	core.Dump(bs.BtnGroup(aria.Label("Sort order"))(
		bs.BtnPrimary()(text.Text("Left")),
		bs.BtnPrimary()(text.Text("Right")),
	))
	// Output: <div class="btn-group" role="group" aria-label="Sort order"><button class="btn btn-primary" type="button">Left</button><button class="btn btn-primary" type="button">Right</button></div>
}

func ExampleCloseButton() {
	core.Dump(bs.CloseButton())
	core.Dump(bs.CloseButton(aria.Label("Dismiss notice")))
	// Output:
	// <button class="btn-close" type="button" aria-label="Close"></button>
	// <button class="btn-close" type="button" aria-label="Dismiss notice"></button>
}

// Badges

func ExampleBadgePrimary() {
	core.Dump(bs.BadgePrimary()(text.Text("New")))
	// Output: <span class="badge text-bg-primary">New</span>
}

func ExampleBadgeOf() {
	core.Dump(bs.BadgeOf(bs.VariantDark)(text.Text("Archived")))
	// Output: <span class="badge text-bg-dark">Archived</span>
}

// Alerts

func ExampleAlertDanger() {
	core.Dump(bs.AlertDanger()(text.Text("Error occurred!")))
	// Output: <div class="alert alert-danger" role="alert">Error occurred!</div>
}

// A role among the attributes replaces the default role="alert", here for
// a confirmation that should not interrupt.
func ExampleAlertSuccess() {
	core.Dump(bs.AlertSuccess(aria.Role("status"))(text.Text("Operation completed.")))
	// Output: <div class="alert alert-success" role="status">Operation completed.</div>
}

func ExampleAlertOf() {
	core.Dump(bs.AlertOf(bs.VariantLight)(text.Text("Nothing to show.")))
	// Output: <div class="alert alert-light" role="alert">Nothing to show.</div>
}

func ExampleAlertDismissible() {
	core.Dump(bs.AlertDismissible(attr.Class("alert-warning"))(text.Text("Check the settings.")))
	// Output: <div class="alert alert-dismissible fade show alert-warning" role="alert">Check the settings.<button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="alert"></button></div>
}

// Grid

func ExampleContainer() {
	core.Dump(bs.Container()(text.Text("content")))
	// Output: <div class="container">content</div>
}

func ExampleRow() {
	core.Dump(bs.Row()(
		bs.Col6()(text.Text("Left")),
		bs.Col6()(text.Text("Right")),
	))
	// Output: <div class="row"><div class="col-6">Left</div><div class="col-6">Right</div></div>
}

func ExampleColMD() {
	core.Dump(bs.ColMD(4))
	core.Dump(bs.ColMD(13))
	// Output:
	// <div class="col-md-4"></div>
	// ERROR: render: at div[0]: attribute: bs.ColMD: invalid option: columns out of range 1 to 12
}

// Card

func ExampleCard() {
	core.Dump(bs.Card{
		Body: text.Text("Card content"),
	})
	// Output: <div class="card"><div class="card-body">Card content</div></div>
}

func ExampleCard_full() {
	core.Dump(bs.Card{
		Header: text.Text("Header"),
		Body:   text.Text("Body content"),
		Footer: text.Text("Footer"),
	})
	// Output: <div class="card"><div class="card-header">Header</div><div class="card-body">Body content</div><div class="card-footer">Footer</div></div>
}

func ExampleCardBody() {
	core.Dump(bs.Card{Body: bs.CardBody{
		Title:      text.Text("Notes"),
		TitleLevel: 3,
		Text:       text.Text("Three notes this week."),
	}})
	// Output: <div class="card"><div class="card-body"><h3 class="card-title">Notes</h3><p class="card-text">Three notes this week.</p></div></div>
}

// Nav

func ExampleNav() {
	core.Dump(bs.Nav()(
		bs.NavItem()(bs.NavLink(text.Href("#"))(text.Text("Home"))),
		bs.NavItem()(bs.NavLink(text.Href("#"))(text.Text("About"))),
	))
	// Output: <ul class="nav"><li class="nav-item"><a class="nav-link" href="#">Home</a></li><li class="nav-item"><a class="nav-link" href="#">About</a></li></ul>
}

func ExampleNavTabs() {
	core.Dump(bs.NavTabs()(
		bs.NavItem()(bs.NavLinkActive(text.Href("/tasks"))(text.Text("Tasks"))),
		bs.NavItem()(bs.NavLink(text.Href("/notes"))(text.Text("Notes"))),
		bs.NavItem()(bs.NavLinkDisabled()(text.Text("Archive"))),
	))
	// Output: <ul class="nav nav-tabs"><li class="nav-item"><a class="nav-link active" aria-current="page" href="/tasks">Tasks</a></li><li class="nav-item"><a class="nav-link" href="/notes">Notes</a></li><li class="nav-item"><a class="nav-link disabled" aria-disabled="true">Archive</a></li></ul>
}

func ExampleNavbar() {
	core.Dump(bs.Navbar(attr.Class("navbar-expand bg-body-tertiary"), aria.Label("Main"))(
		bs.NavbarBrand(text.Href("/"))(text.Text("Brand")),
	))
	// Output: <nav class="navbar navbar-expand bg-body-tertiary" aria-label="Main"><a class="navbar-brand" href="/">Brand</a></nav>
}

func ExampleNavbarTogglerFor() {
	core.Dump(bs.NavbarTogglerFor("main-menu"))
	// Output: <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#main-menu" aria-controls="main-menu" aria-expanded="false" aria-label="Toggle navigation"><span class="navbar-toggler-icon"></span></button>
}

func ExampleBreadcrumb() {
	core.Dump(bs.Breadcrumb()(
		bs.BreadcrumbItem()(text.A(text.Href("/projects"))(text.Text("Projects"))),
		bs.BreadcrumbActive()(text.Text("Website")),
	))
	// Output: <nav aria-label="Breadcrumb"><ol class="breadcrumb"><li class="breadcrumb-item"><a href="/projects">Projects</a></li><li class="breadcrumb-item active" aria-current="page">Website</li></ol></nav>
}

func ExamplePagination() {
	core.Dump(bs.Pagination()(
		bs.PageItemDisabled()(bs.PageLink()(text.Text("Previous"))),
		bs.PageItemActive()(bs.PageLinkCurrent(text.Href("?page=1"))(text.Text("1"))),
		bs.PageItem()(bs.PageLink(text.Href("?page=2"))(text.Text("2"))),
	))
	// Output: <nav aria-label="Pagination"><ul class="pagination"><li class="page-item disabled"><a class="page-link">Previous</a></li><li class="page-item active"><a class="page-link" aria-current="page" href="?page=1">1</a></li><li class="page-item"><a class="page-link" href="?page=2">2</a></li></ul></nav>
}

func ExampleListGroupItemActive() {
	core.Dump(bs.ListGroup()(
		bs.ListGroupItemActive()(text.Text("Inbox")),
		bs.ListGroupItem()(text.Text("Archive")),
	))
	// Output: <ul class="list-group"><li class="list-group-item active" aria-current="true">Inbox</li><li class="list-group-item">Archive</li></ul>
}

// Feedback

func ExampleSpinner() {
	core.Dump(bs.Spinner())
	core.Dump(bs.Spinner(attr.Class("spinner-border-sm"))(text.Text("Saving...")))
	// Output:
	// <div class="spinner-border" role="status"><span class="visually-hidden">Loading...</span></div>
	// <div class="spinner-border spinner-border-sm" role="status"><span class="visually-hidden">Saving...</span></div>
}

// A progress bar without a style attribute: the width is a class, and the
// exact value is in the ARIA attributes.
func ExampleProgress() {
	core.Dump(bs.Progress(aria.Label("Upload"), bs.ProgressValue(50, 0, 100))(
		bs.ProgressBar(attr.Class("w-50"))(),
	))
	// Output: <div class="progress" role="progressbar" aria-label="Upload" aria-valuenow="50" aria-valuemin="0" aria-valuemax="100"><div class="progress-bar w-50"></div></div>
}

// A toast rendered visible by the server, without Bootstrap's script.
func ExampleToast() {
	core.Dump(bs.ToastContainer(attr.Class("position-fixed bottom-0 end-0 p-3"), attr.ID("toasts"))(
		bs.Toast{Body: text.Text("Note saved."), Shown: true, Polite: true},
	))
	// Output: <div class="toast-container position-fixed bottom-0 end-0 p-3" id="toasts"><div class="toast show" role="status" aria-live="polite" aria-atomic="true"><div class="toast-body">Note saved.</div></div></div>
}

// Bootstrap JS components

func ExampleModal() {
	core.Dump(bs.Modal{
		ID:    "confirm",
		Title: text.Text("Delete the note?"),
		Body:  text.Text("This cannot be undone."),
		Size:  "lg",
	})
	// Output: <div class="modal fade" id="confirm" tabindex="-1" aria-labelledby="confirm-title" aria-hidden="true"><div class="modal-dialog modal-lg"><div class="modal-content"><div class="modal-header"><h2 class="modal-title h5" id="confirm-title">Delete the note?</h2><button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="modal"></button></div><div class="modal-body">This cannot be undone.</div></div></div></div>
}

func ExampleModalTrigger() {
	core.Dump(bs.ModalTrigger("myModal")(text.Text("Open Modal")))
	// Output: <button type="button" data-bs-toggle="modal" data-bs-target="#myModal">Open Modal</button>
}

func ExampleOffcanvasTrigger() {
	core.Dump(bs.OffcanvasTrigger("myOffcanvas")(text.Text("Open Menu")))
	// Output: <button type="button" data-bs-toggle="offcanvas" data-bs-target="#myOffcanvas" aria-controls="myOffcanvas">Open Menu</button>
}

func ExampleDropdown() {
	core.Dump(bs.Dropdown{
		Toggle: text.Text("Actions"),
		Items:  []elem.Node{bs.DropdownItem(text.Href("/export"))(text.Text("Export"))},
	})
	// Output: <div class="dropdown"><button class="btn btn-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown" aria-expanded="false">Actions</button><ul class="dropdown-menu"><li><a class="dropdown-item" href="/export">Export</a></li></ul></div>
}

func ExampleOffcanvas() {
	core.Dump(bs.Offcanvas{ID: "filters", Title: text.Text("Filters"), Body: text.Text("..."), Placement: bs.OffcanvasEnd, Backdrop: true})
	// Output: <div class="offcanvas offcanvas-end" id="filters" tabindex="-1" aria-labelledby="filters-title"><div class="offcanvas-header"><h2 class="offcanvas-title h5" id="filters-title">Filters</h2><button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="offcanvas"></button></div><div class="offcanvas-body">...</div></div>
}

func ExampleAccordion() {
	core.Dump(bs.Accordion{
		ID:    "faq",
		Items: []bs.AccordionItem{{Header: text.Text("What is a project?"), Body: text.Text("A group of tasks."), Show: true}},
	})
	// Output: <div class="accordion" id="faq"><div class="accordion-item" id="faq-item-0"><h2 class="accordion-header"><button class="accordion-button" type="button" data-bs-toggle="collapse" data-bs-target="#faq-collapse-0" aria-expanded="true" aria-controls="faq-collapse-0">What is a project?</button></h2><div id="faq-collapse-0" class="accordion-collapse collapse show" data-bs-parent="#faq"><div class="accordion-body">A group of tasks.</div></div></div></div>
}

// Ids that components use in selectors and id references must be tokens.
func ExampleValidID() {
	for _, id := range []string{"note-7", "7th-note", "my note"} {
		fmt.Println(id, bs.ValidID(id))
	}
	// Output:
	// note-7 true
	// 7th-note false
	// my note false
}

// Combined utilities

func Example_layout() {
	core.Dump(bs.Container()(
		bs.Row()(
			bs.ColMD(4, attr.Class("mb-3"))(bs.Card{Body: text.Text("Card 1")}),
			bs.ColMD(4, attr.Class("mb-3"))(bs.Card{Body: text.Text("Card 2")}),
		),
	))
	// Output: <div class="container"><div class="row"><div class="col-md-4 mb-3"><div class="card"><div class="card-body">Card 1</div></div></div><div class="col-md-4 mb-3"><div class="card"><div class="card-body">Card 2</div></div></div></div></div>
}

// Form class wrappers, for hand-built controls.
func ExampleFormCheck() {
	core.Dump(bs.FormCheck()(
		input.Input(attr.Class("form-check-input"), attr.ID("check1"), input.Type("checkbox")),
		bs.FormCheckLabel(label.For("check1"))(text.Text("Check me")),
	))
	// Output: <div class="form-check"><input class="form-check-input" id="check1" type="checkbox"/><label class="form-check-label" for="check1">Check me</label></div>
}
