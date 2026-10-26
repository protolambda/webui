package bs_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/bs"
)

// render renders node and requires the render to succeed.
func render(t devtest.T, node elem.Node) string {
	t.Helper()
	var out strings.Builder
	t.Must(be.NoError(core.Render(t.Context(), node, &out)))
	return out.String()
}

// renderErr renders node and returns the render error.
func renderErr(t devtest.T, node elem.Node) error {
	var out strings.Builder
	return core.Render(t.Context(), node, &out)
}

func TestOffcanvasOptionalFields(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="offcanvas offcanvas-start" id="menu" tabindex="-1" data-bs-backdrop="false">`+
			`<div class="offcanvas-header">`+
			`<button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="offcanvas"></button>`+
			`</div></div>`,
		render(t, bs.Offcanvas{ID: "menu"})))
	t.Must(be.Equal(
		`<div class="offcanvas offcanvas-end" id="menu" tabindex="-1" aria-labelledby="menu-title" data-bs-scroll="true">`+
			`<div class="offcanvas-header">`+
			`<h2 class="offcanvas-title h5" id="menu-title">Menu</h2>`+
			`<button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="offcanvas"></button>`+
			`</div><div class="offcanvas-body">Links</div></div>`,
		render(t, bs.Offcanvas{ID: "menu", Title: text.Text("Menu"), Body: text.Text("Links"), Placement: bs.OffcanvasEnd, Backdrop: true, Scroll: true})))
}

func TestModalStatic(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="modal fade" id="m" tabindex="-1" aria-hidden="true" data-bs-backdrop="static" data-bs-keyboard="false">`+
			`<div class="modal-dialog modal-fullscreen-sm-down modal-dialog-centered"><div class="modal-content">`+
			`<div class="modal-header"><button class="btn-close" type="button" data-bs-dismiss="modal" aria-label="Dismiss"></button></div>`+
			`<div class="modal-footer">OK</div></div></div></div>`,
		render(t, bs.Modal{ID: "m", Footer: text.Text("OK"), Static: true, Size: bs.ModalFullscreenSMDown, Centered: true, CloseLabel: "Dismiss"})))
	// Escape still closes a static modal whose Attrs set data-bs-keyboard.
	t.Must(be.Equal(
		`<div class="modal fade" id="m" tabindex="-1" aria-hidden="true" data-bs-backdrop="static" data-bs-keyboard="true">`+
			`<div class="modal-dialog"><div class="modal-content">`+
			`<div class="modal-header"><button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="modal"></button></div>`+
			`</div></div></div>`,
		render(t, bs.Modal{ID: "m", Static: true, Attrs: attr.Data("bs-keyboard", "true")})))
}

func TestAccordionOptionalFields(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="accordion" id="faq"><div class="accordion-item" id="faq-item-0">`+
			`<h2 class="accordion-header">`+
			`<button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#faq-collapse-0" aria-expanded="false" aria-controls="faq-collapse-0"></button>`+
			`</h2><div id="faq-collapse-0" class="accordion-collapse collapse" data-bs-parent="#faq">`+
			`<div class="accordion-body"></div></div></div></div>`,
		render(t, bs.Accordion{ID: "faq", Items: []bs.AccordionItem{{}}})))
}

func TestAccordionShownAlwaysOpen(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="accordion accordion-flush" id="faq"><div class="accordion-item" id="faq-item-0">`+
			`<h3 class="accordion-header">`+
			`<button class="accordion-button" type="button" data-bs-toggle="collapse" data-bs-target="#faq-collapse-0" aria-expanded="true" aria-controls="faq-collapse-0">Q</button>`+
			`</h3><div id="faq-collapse-0" class="accordion-collapse collapse show">`+
			`<div class="accordion-body">A</div></div></div></div>`,
		render(t, bs.Accordion{ID: "faq", Flush: true, AlwaysOpen: true, HeaderLevel: 3,
			Items: []bs.AccordionItem{{Header: text.Text("Q"), Body: text.Text("A"), Show: true}}})))
}

func TestDetailsAccordionAlwaysOpen(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="accordion accordion-flush" id="faq" data-x="1">`+
			`<details class="accordion-item" open><summary class="accordion-button">Q1</summary><div class="accordion-body">A1</div></details>`+
			`<details class="accordion-item" open><summary class="accordion-button"></summary><div class="accordion-body"></div></details></div>`,
		render(t, bs.DetailsAccordion{ID: "faq", Flush: true, AlwaysOpen: true, Attrs: attr.Data("x", "1"),
			Items: []bs.AccordionItem{{Header: text.Text("Q1"), Body: text.Text("A1"), Show: true}, {Show: true}}})))
	t.Must(be.Equal(`<div class="accordion" id="faq"></div>`, render(t, bs.DetailsAccordion{ID: "faq"})))
}

func TestDropdownOptionalFields(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="dropdown">`+
			`<button class="btn btn-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown" aria-expanded="false"></button>`+
			`<ul class="dropdown-menu"></ul></div>`,
		render(t, bs.Dropdown{})))
	t.Must(be.Equal(
		`<div class="dropdown dropup">`+
			`<button class="btn btn-light dropdown-toggle" type="button" data-bs-toggle="dropdown" aria-expanded="false" id="t">More</button>`+
			`<ul class="dropdown-menu dropdown-menu-end"><li><h6 class="dropdown-header">Group</h6></li>`+
			`<li><a class="dropdown-item active" aria-current="true" href="/a">A</a></li></ul></div>`,
		render(t, bs.Dropdown{
			Toggle: text.Text("More"), Variant: bs.VariantLight,
			ToggleAttrs: attr.ID("t"), MenuAttrs: attr.Class("dropdown-menu-end"), Attrs: attr.Class("dropup"),
			Items: []elem.Node{bs.DropdownHeader()(text.Text("Group")), bs.DropdownItemActive(text.Href("/a"))(text.Text("A"))},
		})))
}

func TestDetailsDropdownOpen(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<details class="dropdown" open id="d"><summary class="btn btn-secondary dropdown-toggle" aria-label="More actions"></summary><ul class="dropdown-menu"></ul></details>`,
		render(t, bs.DetailsDropdown{Open: true, ToggleAttrs: aria.Label("More actions"), Attrs: attr.ID("d")})))
}

func TestToastMarkup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="toast" role="alert" aria-live="assertive" aria-atomic="true">`+
			`<div class="toast-header">Saved<button class="btn-close" type="button" aria-label="Close" data-bs-dismiss="toast"></button></div>`+
			`<div class="toast-body">Note saved.</div></div>`,
		render(t, bs.Toast{Header: text.Text("Saved"), Body: text.Text("Note saved.")})))
	t.Must(be.Equal(
		`<div class="toast show" role="alert" aria-live="assertive" aria-atomic="true"><div class="toast-header">Saved</div></div>`,
		render(t, bs.Toast{Header: text.Text("Saved"), Shown: true, Close: elem.Noop()})))
	t.Must(be.Equal(
		`<div class="toast show" role="alert" aria-live="assertive" aria-atomic="true"><div class="toast-header">Saved`+
			`<button class="btn-close" type="button" aria-label="Close" hx-delete="/toasts/1"></button></div></div>`,
		render(t, bs.Toast{Header: text.Text("Saved"), Shown: true, Close: bs.CloseButton(attr.KV("hx-delete", "/toasts/1"))})))
}

// The container is the one live region of the toasts in it: they render no
// live region attributes of their own, wherever they are below it, while a
// toast outside a container keeps Bootstrap's.
func TestToastContainerAnnounces(gt *testing.T) {
	t := devtest.ParallelT(gt)
	toast := bs.Toast{Body: text.Text("Saved"), Shown: true, Polite: true}
	t.Must(be.Equal(
		`<div class="toast-container" aria-live="polite" id="toasts" hx-swap-oob="beforeend">`+
			`<div class="toast show"><div class="toast-body">Saved</div></div>`+
			`<div class="wrap"><div class="toast"><div class="toast-body">Deeper</div></div></div></div>`+
			`<div class="toast show" role="status" aria-live="polite" aria-atomic="true"><div class="toast-body">Saved</div></div>`,
		render(t, elem.Bundle{
			bs.ToastContainer(attr.ID("toasts"), attr.KV("hx-swap-oob", "beforeend"))(
				toast,
				div.Div(attr.Class("wrap"))(bs.Toast{Body: text.Text("Deeper")}),
			),
			toast,
		})))

	// The container's politeness decides; a caller may still give a toast in
	// it attributes of its own.
	t.Must(be.Equal(
		`<div class="toast-container" aria-live="assertive"><div class="toast" id="t"></div></div>`,
		render(t, bs.ToastContainer(aria.Live("assertive"))(bs.Toast{Attrs: attr.ID("t")}))))

	// One live region per toast in the page, never one inside another.
	page := ct.View(elem.Bundle{
		bs.ToastContainer(attr.ID("toasts"))(
			bs.Toast{Header: text.Text("One"), Body: text.Text("A"), Shown: true},
			bs.Toast{Header: text.Text("Two"), Body: text.Text("B"), Shown: true, Polite: true},
		),
		bs.Toast{Header: text.Text("Three"), Body: text.Text("C"), Shown: true},
	})
	live := ct.HasAttr("aria-live").Or(ct.Attr("role", "alert"), ct.Attr("role", "status"))
	t.Must(page.Find(live).Count(2))
	t.Must(page.Find(live).Find(live).None())
}

func TestPaginationMarkup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<nav aria-label="Pagination"><ul class="pagination"><li class="page-item"><a class="page-link" href="?page=1">1</a></li></ul></nav>`,
		render(t, bs.Pagination()(bs.PageItem()(bs.PageLink(text.Href("?page=1"))(text.Text("1")))))))
	t.Must(be.Equal(
		`<nav aria-label="Notes pages"><ul class="pagination pagination-sm"></ul></nav>`,
		render(t, bs.PaginationLabeled("Notes pages", attr.Class("pagination-sm"))())))
	t.Must(be.Equal(
		`<nav aria-label="Pfad"><ol class="breadcrumb"></ol></nav>`,
		render(t, bs.BreadcrumbLabeled("Pfad")())))
}

func TestVariantConstructors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	variants := []bs.Variant{
		bs.VariantPrimary, bs.VariantSecondary, bs.VariantSuccess, bs.VariantDanger,
		bs.VariantWarning, bs.VariantInfo, bs.VariantLight, bs.VariantDark,
	}
	for _, v := range variants {
		t.Run(string(v), func(t devtest.T) {
			t.Must(be.True(v.Valid()))
			t.Must(be.Equal(`<button class="btn btn-`+string(v)+`" type="button"></button>`, render(t, bs.BtnOf(v))))
			t.Must(be.Equal(`<button class="btn btn-outline-`+string(v)+`" type="button"></button>`, render(t, bs.BtnOutlineOf(v))))
			t.Must(be.Equal(`<div class="alert alert-`+string(v)+`" role="alert"></div>`, render(t, bs.AlertOf(v))))
			t.Must(be.Equal(`<span class="badge text-bg-`+string(v)+`"></span>`, render(t, bs.BadgeOf(v))))
		})
	}
	named := []struct {
		node elem.Node
		want string
	}{
		{bs.BtnOutlineSuccess(), `<button class="btn btn-outline-success" type="button"></button>`},
		{bs.BtnOutlineDanger(), `<button class="btn btn-outline-danger" type="button"></button>`},
		{bs.BtnOutlineWarning(), `<button class="btn btn-outline-warning" type="button"></button>`},
		{bs.BtnOutlineInfo(), `<button class="btn btn-outline-info" type="button"></button>`},
		{bs.BtnOutlineLight(), `<button class="btn btn-outline-light" type="button"></button>`},
		{bs.BtnOutlineDark(), `<button class="btn btn-outline-dark" type="button"></button>`},
		{bs.AlertLight(), `<div class="alert alert-light" role="alert"></div>`},
		{bs.AlertDark(), `<div class="alert alert-dark" role="alert"></div>`},
		{bs.BadgeLight(), `<span class="badge text-bg-light"></span>`},
		{bs.BadgeDark(), `<span class="badge text-bg-dark"></span>`},
		{bs.NavPills(), `<ul class="nav nav-pills"></ul>`},
		{bs.NavUnderline(), `<ul class="nav nav-underline"></ul>`},
		{bs.ListGroupFlush(), `<ul class="list-group list-group-flush"></ul>`},
		{bs.BtnToolbar(), `<div class="btn-toolbar" role="toolbar"></div>`},
		{bs.SpinnerGrow(), `<div class="spinner-grow" role="status"><span class="visually-hidden">Loading...</span></div>`},
		{bs.FormSwitch(), `<div class="form-check form-switch"></div>`},
		{bs.InvalidFeedback(), `<div class="invalid-feedback"></div>`},
	}
	for _, tc := range named {
		t.Must(be.Equal(tc.want, render(t, tc.node)))
	}
	t.Must(be.False(bs.Variant("").Valid()))
	t.Must(be.False(bs.Variant("Primary").Valid()))
}

func TestCloseButtonsHaveAccessibleName(gt *testing.T) {
	t := devtest.ParallelT(gt)
	components := map[string]elem.Node{
		"modal":             bs.Modal{ID: "m", Title: text.Text("Title"), Body: text.Text("Body")},
		"toast":             bs.Toast{Header: text.Text("Header"), Body: text.Text("Body")},
		"offcanvas":         bs.Offcanvas{ID: "o", Title: text.Text("Menu")},
		"dismissible alert": bs.AlertDismissible()(text.Text("Saved")),
		"dialog":            bs.Dialog{ID: "d", Title: text.Text("Title")},
		"close button":      bs.CloseButton(),
	}
	for name, node := range components {
		t.Run(name, func(t devtest.T) {
			closeBtn := ct.View(node).Find(ct.Role("button", ct.Named("Close")))
			t.Must(closeBtn.Matches(ct.Class("btn-close"), ct.Attr("type", "button")))
		})
	}
}

func TestBtnDefaultsToTypeButton(gt *testing.T) {
	t := devtest.ParallelT(gt)
	dynamic := attr.Fn(func(context.Context) (attr.Node, error) { return button.Type(button.TypeSubmit), nil })
	tests := map[string]struct {
		node elem.Node
		want string
	}{
		"default":     {bs.BtnPrimary()(text.Text("Go")), `<button class="btn btn-primary" type="button">Go</button>`},
		"plain":       {bs.Btn(attr.ID("x")), `<button class="btn" type="button" id="x"></button>`},
		"submit":      {bs.BtnPrimary(button.Type(button.TypeSubmit)), `<button class="btn btn-primary" type="submit"></button>`},
		"reset":       {bs.BtnSecondary(attr.ID("r"), button.Type(button.TypeReset)), `<button class="btn btn-secondary" id="r" type="reset"></button>`},
		"runtime key": {bs.BtnDanger(attr.KV("TYPE", "submit")), `<button class="btn btn-danger" TYPE="submit"></button>`},
		"in bundle":   {bs.BtnLink(attr.Bundle{attr.ID("b"), button.Type(button.TypeSubmit)}), `<button class="btn btn-link" id="b" type="submit"></button>`},
		"in sequence": {bs.BtnLink(attr.Cons(attr.ID("b"), button.Type(button.TypeSubmit))), `<button class="btn btn-link" id="b" type="submit"></button>`},
		"lazy":        {bs.BtnPrimary(dynamic), `<button class="btn btn-primary" type="submit"></button>`},
		"noop":        {bs.BtnPrimary(attr.Noop()), `<button class="btn btn-primary" type="button"></button>`},
	}
	for name, tc := range tests {
		t.Run(name, func(t devtest.T) {
			t.Must(be.Equal(tc.want, render(t, tc.node)))
		})
	}

	page := ct.View(form.Form()(bs.BtnPrimary()(text.Text("Cancel")), bs.BtnSuccess(button.Type(button.TypeSubmit))(text.Text("Save"))))
	t.Must(page.Valid(ct.ButtonsHaveType()))
}

// A default attribute of a component is replaced by a caller's attribute of
// the same name, wherever the caller puts it, without a duplicate
// attribute error.
func TestDefaultsReplacedByCallerAttributes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	role := func(v string) attr.Node { return aria.Role(v) }
	lazy := func(n attr.Node) attr.Node {
		return attr.Fn(func(context.Context) (attr.Node, error) { return n, nil })
	}
	oneRow := []bs.TableRow{{Cells: []elem.Node{text.Text("a")}}}
	tests := map[string]struct {
		node      elem.Node
		attr, val string
	}{
		"alert role":             {bs.AlertDanger(role("status")), "role", "status"},
		"alert role in bundle":   {bs.AlertInfo(attr.Bundle{role("note")}), "role", "note"},
		"alert role in sequence": {bs.Alert(attr.Cons(attr.ID("x"), role("status"))), "role", "status"},
		"alert role lazy":        {bs.AlertOf(bs.VariantInfo, lazy(role("status"))), "role", "status"},
		"dismissible alert role": {bs.AlertDismissible(role("status"))(), "role", "status"},
		"btn group role":         {bs.BtnGroup(role("toolbar")), "role", "toolbar"},
		"spinner role":           {bs.Spinner(role("img")), "role", "img"},
		"progress role":          {bs.Progress(role("meter")), "role", "meter"},
		"active link":            {bs.NavLinkActive(aria.Current("location")), "aria-current", "location"},
		"breadcrumb":             {bs.BreadcrumbActive(aria.Current("location")), "aria-current", "location"},
		"close label":            {bs.CloseButton(aria.Label("Dismiss")), "aria-label", "Dismiss"},
		"toggler label":          {bs.NavbarToggler(aria.Label("Menu")), "aria-label", "Menu"},
		"toggler expanded":       {bs.NavbarTogglerFor("nav", aria.Expanded("true")), "aria-expanded", "true"},
		"modal trigger type":     {bs.ModalTrigger("m", button.Type(button.TypeSubmit)), "type", "submit"},
		"dialog button type":     {bs.DialogButton("d", button.Type(button.TypeSubmit)), "type", "submit"},
		"toast role":             {bs.Toast{Attrs: role("log")}, "role", "log"},
		"toast live":             {bs.Toast{Attrs: aria.Live("off")}, "aria-live", "off"},
		"modal tabindex":         {bs.Modal{ID: "m", Attrs: attr.Tabindex("0")}, "tabindex", "0"},
		"modal static keyboard":  {bs.Modal{ID: "m", Static: true, Attrs: attr.Data("bs-keyboard", "true")}, "data-bs-keyboard", "true"},
		"modal static no keys":   {bs.Modal{ID: "m", Static: true, Attrs: attr.Name("data-bs-keyboard").Raw("false")}, "data-bs-keyboard", "false"},
		"toast container live":   {bs.ToastContainer(aria.Live("off"))(), "aria-live", "off"},
		"offcanvas labelledby":   {bs.Offcanvas{ID: "o", Title: text.Text("T"), Attrs: aria.Labelledby("o")}, "aria-labelledby", "o"},
		"dialog popover":         {bs.Dialog{ID: "d", Title: text.Text("T"), Attrs: attr.Popover("manual")}, "popover", "manual"},
		"dialog label":           {bs.Dialog{ID: "d", Attrs: aria.Label("Settings")}, "aria-label", "Settings"},
		"checkbox switch role":   {bs.Checkbox{ID: "c", Label: text.Text("C"), Switch: true, Attrs: role("checkbox")}, "role", "checkbox"},
		"field invalid":          {bs.Field{ID: "f", Label: text.Text("F"), Error: text.Text("E"), Attrs: aria.Invalid("grammar")}, "aria-invalid", "grammar"},
		"table header scope":     {bs.DataTable{Columns: []bs.Column{{HeaderAttrs: attr.KV("scope", "colgroup")}}, Rows: oneRow}, "scope", "colgroup"},
	}
	for name, tc := range tests {
		t.Run(name, func(t devtest.T) {
			// A default that stayed would be a duplicate attribute, which
			// fails the render.
			render(t, tc.node)
			t.Must(ct.View(tc.node).Find(ct.Attr(tc.attr, tc.val)))
		})
	}
}

// Caller attributes are evaluated once per render, also when a component
// inspects them for defaults, and their errors fail the render.
func TestCallerAttributesEvaluatedOnce(gt *testing.T) {
	t := devtest.ParallelT(gt)
	calls := 0
	counted := attr.Fn(func(context.Context) (attr.Node, error) {
		calls++
		return attr.Data("x", "1"), nil
	})
	node := bs.Field{ID: "f", Label: text.Text("F"), Attrs: attr.Bundle{counted}}
	render(t, node)
	t.Must(be.Equal(1, calls))
	render(t, bs.AlertDanger(counted))
	t.Must(be.Equal(2, calls))
	render(t, bs.Dialog{ID: "d", Title: text.Text("T"), Attrs: counted})
	t.Must(be.Equal(3, calls))

	// A single-use sequence works, since it is iterated once.
	used := false
	once := attr.Seq(func(yield func(attr.Node) bool) {
		if used {
			return
		}
		used = true
		yield(aria.Role("status"))
	})
	t.Must(be.Equal(`<div class="alert" role="status"></div>`, render(t, bs.Alert(once))))

	errBroken := errors.New("broken attribute")
	broken := attr.Fn(func(context.Context) (attr.Node, error) { return nil, errBroken })
	for name, node := range map[string]elem.Node{
		"alert":    bs.Alert(broken),
		"btn":      bs.Btn(broken),
		"dialog":   bs.Dialog{ID: "d", Title: text.Text("T"), Attrs: broken},
		"field":    bs.Field{ID: "f", Label: text.Text("F"), Attrs: broken},
		"dropdown": bs.DetailsDropdown{ToggleAttrs: broken},
		"table":    bs.DataTable{Columns: []bs.Column{{Attrs: broken}}},
		"header":   bs.DataTable{Columns: []bs.Column{{HeaderAttrs: broken}}},
	} {
		t.Run(name, func(t devtest.T) {
			t.Must(be.ErrorIs(renderErr(t, node), errBroken))
		})
	}
	t.Must(be.ErrorIs(renderErr(t, bs.Alert(nil)), attr.ErrNilNode))
}

// Attributes that a component needs are not defaults: setting them again is
// a duplicate.
func TestRequiredAttributesAreNotReplaced(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, node := range map[string]elem.Node{
		"accordion id": bs.Accordion{ID: "a", Attrs: attr.ID("b")},
		"dialog id":    bs.Dialog{ID: "a", Title: text.Text("T"), Attrs: attr.ID("b")},
		"modal id":     bs.Modal{ID: "a", Attrs: attr.ID("b")},
		"modal static": bs.Modal{ID: "a", Static: true, Attrs: attr.Data("bs-backdrop", "true")},
		"field name":   bs.Field{ID: "f", Name: "n", Label: text.Text("F"), Attrs: input.Name("m")},
		"field type":   bs.Field{ID: "f", Label: text.Text("F"), Attrs: input.Type(input.Hidden)},
	} {
		t.Run(name, func(t devtest.T) {
			t.Must(be.ErrorContains(renderErr(t, node), "duplicate attribute"))
		})
	}
}

func TestValidID(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, id := range []string{"a", "Z", "note-7", "a_b", "x1-2_3", "A-"} {
		t.Mustf(be.True(bs.ValidID(id)), "valid %q", id)
	}
	for _, id := range []string{"", "1a", "-a", "_a", "a b", "a#b", "a.b", "a:b", `a"b`, "a'b", "a<b", "a/b", "é", "aé", "a\n", "a\x00"} {
		t.Mustf(be.False(bs.ValidID(id)), "invalid %q", id)
	}
}

// Every component that takes an id fails the render for an invalid one,
// without echoing it in the error.
func TestInvalidIDsFailTheRender(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, id := range []string{"", "secret value", "1st", `x"onclick="y`} {
		components := map[string]elem.Node{
			"Modal":             bs.Modal{ID: id},
			"ModalTrigger":      bs.ModalTrigger(id),
			"Offcanvas":         bs.Offcanvas{ID: id},
			"OffcanvasTrigger":  bs.OffcanvasTrigger(id),
			"Accordion":         bs.Accordion{ID: id},
			"DetailsAccordion":  bs.DetailsAccordion{ID: id},
			"Dialog":            bs.Dialog{ID: id, Title: text.Text("T")},
			"DialogOpen":        bs.Btn(bs.DialogOpen(id)),
			"DialogClose":       bs.Btn(bs.DialogClose(id)),
			"DialogButton":      bs.DialogButton(id),
			"NavbarTogglerFor":  bs.NavbarTogglerFor(id),
			"Field":             bs.Field{ID: id, Label: text.Text("L")},
			"Textarea":          bs.Textarea{ID: id, Label: text.Text("L")},
			"Select":            bs.Select{ID: id, Label: text.Text("L")},
			"Checkbox":          bs.Checkbox{ID: id, Label: text.Text("L")},
			"Radio":             bs.Radio{ID: id, Label: text.Text("L")},
			"Dialog in a list":  elem.Bundle{bs.Dialog{ID: "ok", Title: text.Text("T")}, bs.Dialog{ID: id, Title: text.Text("T")}},
			"Field with errors": bs.Field{ID: id, Label: text.Text("L"), Error: text.Text("E")},
		}
		for name, node := range components {
			t.Run(name+"/"+id, func(t devtest.T) {
				err := renderErr(t, node)
				t.Must(be.ErrorIs(err, bs.ErrInvalidID))
				t.Must(be.ErrorIs(err, bs.ErrInvalidOption))
				if id != "" {
					t.Must(be.False(strings.Contains(err.Error(), id)))
				}
			})
		}
	}
}

// Options outside their closed sets, and inconsistent fields, fail the
// render instead of emitting classes that do not exist.
func TestInvalidOptionsFailTheRender(gt *testing.T) {
	t := devtest.ParallelT(gt)
	row := func(cells ...elem.Node) bs.TableRow { return bs.TableRow{Cells: cells} }
	col := bs.Column{Header: text.Text("H")}
	lazyValue := attr.Fn(func(context.Context) (attr.Node, error) { return input.Value("x"), nil })
	tests := map[string]elem.Node{
		"BtnOf":                    bs.BtnOf("purple"),
		"BtnOutlineOf":             bs.BtnOutlineOf(""),
		"AlertOf":                  bs.AlertOf("Danger"),
		"BadgeOf":                  bs.BadgeOf("x y"),
		"Dropdown variant":         bs.Dropdown{Variant: "link"},
		"DetailsDropdown variant":  bs.DetailsDropdown{Toggle: text.Text("T"), Variant: "huge"},
		"Modal size":               bs.Modal{ID: "m", Size: "huge"},
		"Modal size escape":        bs.Modal{ID: "m", Size: `lg" onclick="x`},
		"Dialog size":              bs.Dialog{ID: "d", Title: text.Text("T"), Size: "md"},
		"Offcanvas placement":      bs.Offcanvas{ID: "o", Placement: "left"},
		"Modal heading 7":          bs.Modal{ID: "m", Title: text.Text("T"), TitleLevel: 7},
		"Dialog heading -1":        bs.Dialog{ID: "d", Title: text.Text("T"), TitleLevel: -1},
		"Offcanvas heading 9":      bs.Offcanvas{ID: "o", Title: text.Text("T"), TitleLevel: 9},
		"Accordion heading 7":      bs.Accordion{ID: "a", HeaderLevel: 7, Items: []bs.AccordionItem{{}}},
		"CardBody title 0 ok, 8":   bs.CardBody{Title: text.Text("T"), TitleLevel: 8},
		"CardBody subtitle 7":      bs.CardBody{Subtitle: text.Text("S"), SubtitleLevel: 7},
		"ColSM 0":                  bs.ColSM(0),
		"ColXXL 13":                bs.ColXXL(13),
		"ColLG -1":                 bs.ColLG(-1),
		"ColXL 100":                bs.ColXL(100),
		"Progress min max":         bs.Progress(bs.ProgressValue(5, 10, 10)),
		"Progress below":           bs.Progress(bs.ProgressValue(-1, 0, 100)),
		"Progress above":           bs.Progress(bs.ProgressValue(101, 0, 100)),
		"Field checkbox type":      bs.Field{ID: "f", Label: text.Text("F"), Type: input.Checkbox},
		"Field hidden type":        bs.Field{ID: "f", Label: text.Text("F"), Type: input.Hidden},
		"Field submit type":        bs.Field{ID: "f", Label: text.Text("F"), Type: input.Submit},
		"Field unknown type":       bs.Field{ID: "f", Label: text.Text("F"), Type: "bogus"},
		"Field password value":     bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Value: "x"},
		"Field password attr":      bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Attrs: input.Value(`x"onclick="y`)},
		"Field password VALUE":     bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Attrs: attr.KV("VALUE", "x")},
		"Field password bundle":    bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Attrs: attr.Bundle{input.Minlength("8"), input.Value("x")}},
		"Field password lazy":      bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Attrs: lazyValue},
		"Field input mode":         bs.Field{ID: "f", Label: text.Text("F"), InputMode: "phone"},
		"Field plaintext color":    bs.Field{ID: "f", Label: text.Text("F"), Type: input.Color, Plaintext: true},
		"Field plaintext file":     bs.Field{ID: "f", Label: text.Text("F"), Type: input.File, Plaintext: true},
		"Field plaintext range":    bs.Field{ID: "f", Label: text.Text("F"), Type: input.Range, Plaintext: true},
		"Field plaintext password": bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Plaintext: true},
		"Field plaintext before":   bs.Field{ID: "f", Label: text.Text("F"), Plaintext: true, Before: []elem.Node{bs.InputGroupText()(text.Text("@"))}},
		"Field plaintext after":    bs.Field{ID: "f", Label: text.Text("F"), Plaintext: true, After: []elem.Node{bs.BtnSecondary()(text.Text("Copy"))}},
		"Textarea rows":            bs.Textarea{ID: "t", Label: text.Text("T"), Rows: -1},
		"Select two selected":      bs.Select{ID: "s", Label: text.Text("S"), Options: []bs.SelectOption{{Selected: true}, {Selected: true}}},
		"DetailsAccordion two":     bs.DetailsAccordion{ID: "a", Items: []bs.AccordionItem{{Show: true}, {Show: true}}},
		"Dialog open":              bs.Dialog{ID: "d", Title: text.Text("T"), Attrs: attr.Name("open").Bool()},
		"Dialog OPEN":              bs.Dialog{ID: "d", Title: text.Text("T"), Attrs: attr.KV("OPEN", "")},
		"DataTable no columns":     bs.DataTable{},
		"DataTable short row":      bs.DataTable{Columns: []bs.Column{col, col}, Rows: []bs.TableRow{row(text.Text("a"))}},
		"DataTable long row":       bs.DataTable{Columns: []bs.Column{col}, Rows: []bs.TableRow{row(nil, nil)}},
		"DataTable empty row":      bs.DataTable{Columns: []bs.Column{col}, Rows: []bs.TableRow{{}}},
		"DataTable column id":      bs.DataTable{Columns: []bs.Column{col, {Attrs: attr.Bundle{attr.Class("x"), attr.ID("c")}}}},
		"Select multiple selected": bs.Select{ID: "s", Label: text.Text("S"), Options: []bs.SelectOption{{Selected: true}, {Selected: true}, {Selected: true}}},
	}
	for name, node := range tests {
		t.Run(name, func(t devtest.T) {
			err := renderErr(t, node)
			t.Must(be.ErrorIs(err, bs.ErrInvalidOption))
			t.Must(be.False(errors.Is(err, bs.ErrInvalidID)))
			t.Must(be.False(strings.Contains(err.Error(), "onclick")))
		})
	}

	// The boundaries are valid.
	for name, node := range map[string]elem.Node{
		"ColSM 1":           bs.ColSM(1),
		"ColMD 12":          bs.ColMD(12),
		"Progress at min":   bs.Progress(bs.ProgressValue(0, 0, 1)),
		"Progress at max":   bs.Progress(bs.ProgressValue(1, 0, 1)),
		"Progress negative": bs.Progress(bs.ProgressValue(-5, -10, 0)),
		"heading 1":         bs.Dialog{ID: "d", Title: text.Text("T"), TitleLevel: 1},
		"heading 6":         bs.Modal{ID: "m", Title: text.Text("T"), TitleLevel: 6},
		"multiple selected": bs.Select{ID: "s", Label: text.Text("S"), Multiple: true, Options: []bs.SelectOption{{Selected: true}, {Selected: true}}},
		"always open":       bs.DetailsAccordion{ID: "a", AlwaysOpen: true, Items: []bs.AccordionItem{{Show: true}, {Show: true}}},
		"offcanvas bottom":  bs.Offcanvas{ID: "o", Placement: bs.OffcanvasBottom},
		"fullscreen":        bs.Dialog{ID: "d", Title: text.Text("T"), Size: bs.ModalFullscreen},
		"textarea 0 rows":   bs.Textarea{ID: "t", Label: text.Text("T")},
		"password attrs":    bs.Field{ID: "f", Label: text.Text("F"), Type: input.Password, Attrs: input.Minlength("8")},
		"plaintext email":   bs.Field{ID: "f", Label: text.Text("F"), Type: input.Email, Plaintext: true},
		"plaintext date":    bs.Field{ID: "f", Label: text.Text("F"), Type: input.Date, Plaintext: true},
		"header id":         bs.DataTable{Columns: []bs.Column{{HeaderAttrs: attr.ID("h")}}},
	} {
		t.Run("valid "+name, func(t devtest.T) {
			t.Must(be.NoError(renderErr(t, node)))
		})
	}
}

func TestOptionValidity(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range []bs.ModalSize{bs.ModalDefault, bs.ModalSM, bs.ModalLG, bs.ModalXL, bs.ModalFullscreen,
		bs.ModalFullscreenSMDown, bs.ModalFullscreenMDDown, bs.ModalFullscreenLGDown, bs.ModalFullscreenXLDown, bs.ModalFullscreenXXLDown} {
		t.Must(be.True(s.Valid()))
	}
	t.Must(be.False(bs.ModalSize("md").Valid()))
	for _, p := range []bs.OffcanvasPlacement{"", bs.OffcanvasStart, bs.OffcanvasEnd, bs.OffcanvasTop, bs.OffcanvasBottom} {
		t.Must(be.True(p.Valid()))
	}
	t.Must(be.False(bs.OffcanvasPlacement("left").Valid()))
	for _, m := range []bs.InputMode{"", bs.InputModeNone, bs.InputModeText, bs.InputModeDecimal, bs.InputModeNumeric,
		bs.InputModeTel, bs.InputModeSearch, bs.InputModeEmail, bs.InputModeURL} {
		t.Must(be.True(m.Valid()))
	}
	t.Must(be.False(bs.InputMode("phone").Valid()))
}

// Components without an accessible name fail the render, unless the caller
// names them with an ARIA attribute.
func TestMissingNamesFailTheRender(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, node := range map[string]elem.Node{
		"dialog":           bs.Dialog{ID: "d"},
		"field":            bs.Field{ID: "f"},
		"textarea":         bs.Textarea{ID: "t"},
		"select":           bs.Select{ID: "s"},
		"checkbox":         bs.Checkbox{ID: "c"},
		"radio":            bs.Radio{ID: "r"},
		"details dropdown": bs.DetailsDropdown{},
	} {
		t.Run(name, func(t devtest.T) {
			t.Must(be.ErrorIs(renderErr(t, node), bs.ErrNoName))
		})
	}
	for name, node := range map[string]elem.Node{
		"dialog label":           bs.Dialog{ID: "d", Attrs: aria.Label("Settings")},
		"dialog labelledby":      elem.Bundle{text.Span(attr.ID("x"))(text.Text("X")), bs.Dialog{ID: "d", Attrs: aria.Labelledby("x")}},
		"field label":            bs.Field{ID: "f", Attrs: aria.Label("Search")},
		"textarea label":         bs.Textarea{ID: "t", Attrs: aria.Label("Note")},
		"select label":           bs.Select{ID: "s", Attrs: aria.Label("Sort")},
		"checkbox label":         bs.Checkbox{ID: "c", Attrs: aria.Label("Select all")},
		"details dropdown label": bs.DetailsDropdown{ToggleAttrs: aria.Label("More")},
	} {
		t.Run(name, func(t devtest.T) {
			out := render(t, node)
			t.Must(be.False(strings.Contains(out, "<label")))
		})
	}
}

// Caller text and attribute values are escaped by chord: hostile values
// cannot leave their attribute or element.
func TestHostileValuesStayInert(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const hostile = `"><script>alert(1)</script><x a='`
	nodes := map[string]elem.Node{
		"field": bs.Field{ID: "f", Name: hostile, Label: text.Text(hostile), Value: hostile, Placeholder: hostile,
			Autocomplete: hostile, Help: text.Text(hostile), Error: text.Text(hostile)},
		"textarea": bs.Textarea{ID: "t", Name: hostile, Label: text.Text(hostile), Value: hostile + "</textarea>", Placeholder: hostile},
		"select":   bs.Select{ID: "s", Name: hostile, Label: text.Text(hostile), Options: []bs.SelectOption{{Value: hostile, Label: hostile}}},
		"checkbox": bs.Checkbox{ID: "c", Name: hostile, Value: hostile, Label: text.Text(hostile)},
		"dialog":   bs.Dialog{ID: "d", Title: text.Text(hostile), Body: text.Text(hostile), CloseLabel: hostile},
		"modal":    bs.Modal{ID: "m", Title: text.Text(hostile), CloseLabel: hostile},
		"table":    bs.DataTable{Caption: text.Text(hostile), Columns: []bs.Column{{Header: text.Text(hostile)}}, Rows: []bs.TableRow{{Cells: []elem.Node{text.Text(hostile)}}}},
		"crumbs":   bs.BreadcrumbLabeled(hostile)(),
		"pages":    bs.PaginationLabeled(hostile)(),
	}
	for name, node := range nodes {
		t.Run(name, func(t devtest.T) {
			out := render(t, node)
			t.Must(be.False(strings.Contains(out, "<script")))
			t.Must(be.False(strings.Contains(out, "<x ")))
			t.Must(ct.View(node).Find(ct.Tag("script")).None())
		})
	}
	page := ct.View(nodes["field"])
	t.Must(page.Find(ct.Tag("input")).Matches(ct.Attr("value", hostile), ct.Attr("name", hostile), ct.Attr("placeholder", hostile)))
	t.Must(ct.View(nodes["select"]).Find(ct.Tag("option")).Matches(ct.Attr("value", hostile), ct.TextContent(hostile)))
}

// The simple constructors render their Bootstrap classes, before the
// caller's attributes.
func TestSimpleConstructors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	x := attr.ID("x")
	tests := []struct {
		node elem.Node
		want string
	}{
		{bs.Alert(x), `<div class="alert" role="alert" id="x"></div>`},
		{bs.AlertPrimary(), `<div class="alert alert-primary" role="alert"></div>`},
		{bs.AlertSecondary(), `<div class="alert alert-secondary" role="alert"></div>`},
		{bs.AlertSuccess(), `<div class="alert alert-success" role="alert"></div>`},
		{bs.AlertDanger(), `<div class="alert alert-danger" role="alert"></div>`},
		{bs.AlertWarning(), `<div class="alert alert-warning" role="alert"></div>`},
		{bs.AlertInfo(), `<div class="alert alert-info" role="alert"></div>`},
		{bs.Badge(x), `<span class="badge" id="x"></span>`},
		{bs.BadgePrimary(), `<span class="badge text-bg-primary"></span>`},
		{bs.BadgeSecondary(), `<span class="badge text-bg-secondary"></span>`},
		{bs.BadgeSuccess(), `<span class="badge text-bg-success"></span>`},
		{bs.BadgeDanger(), `<span class="badge text-bg-danger"></span>`},
		{bs.BadgeWarning(), `<span class="badge text-bg-warning"></span>`},
		{bs.BadgeInfo(), `<span class="badge text-bg-info"></span>`},
		{bs.BtnSecondary(), `<button class="btn btn-secondary" type="button"></button>`},
		{bs.BtnSuccess(), `<button class="btn btn-success" type="button"></button>`},
		{bs.BtnDanger(), `<button class="btn btn-danger" type="button"></button>`},
		{bs.BtnWarning(), `<button class="btn btn-warning" type="button"></button>`},
		{bs.BtnInfo(), `<button class="btn btn-info" type="button"></button>`},
		{bs.BtnLight(), `<button class="btn btn-light" type="button"></button>`},
		{bs.BtnDark(), `<button class="btn btn-dark" type="button"></button>`},
		{bs.BtnLink(), `<button class="btn btn-link" type="button"></button>`},
		{bs.BtnOutlineSecondary(), `<button class="btn btn-outline-secondary" type="button"></button>`},
		{bs.Container(x), `<div class="container" id="x"></div>`},
		{bs.ContainerFluid(), `<div class="container-fluid"></div>`},
		{bs.ContainerSM(), `<div class="container-sm"></div>`},
		{bs.ContainerMD(), `<div class="container-md"></div>`},
		{bs.ContainerLG(), `<div class="container-lg"></div>`},
		{bs.ContainerXL(), `<div class="container-xl"></div>`},
		{bs.ContainerXXL(), `<div class="container-xxl"></div>`},
		{bs.Row(), `<div class="row"></div>`},
		{bs.Col(), `<div class="col"></div>`},
		{bs.Col1(), `<div class="col-1"></div>`},
		{bs.Col2(), `<div class="col-2"></div>`},
		{bs.Col3(), `<div class="col-3"></div>`},
		{bs.Col4(), `<div class="col-4"></div>`},
		{bs.Col5(), `<div class="col-5"></div>`},
		{bs.Col6(), `<div class="col-6"></div>`},
		{bs.Col7(), `<div class="col-7"></div>`},
		{bs.Col8(), `<div class="col-8"></div>`},
		{bs.Col9(), `<div class="col-9"></div>`},
		{bs.Col10(), `<div class="col-10"></div>`},
		{bs.Col11(), `<div class="col-11"></div>`},
		{bs.Col12(), `<div class="col-12"></div>`},
		{bs.ColSM(1, x), `<div class="col-sm-1" id="x"></div>`},
		{bs.ColLG(7), `<div class="col-lg-7"></div>`},
		{bs.ColXL(12), `<div class="col-xl-12"></div>`},
		{bs.ColXXL(3), `<div class="col-xxl-3"></div>`},
		{bs.ColAuto(), `<div class="col-auto"></div>`},
		{bs.CardImgOverlay(), `<div class="card-img-overlay"></div>`},
		{bs.Carousel(), `<div class="carousel slide"></div>`},
		{bs.Collapse(x), `<div class="collapse" id="x"></div>`},
		{bs.FormGroup(), `<div class="mb-3"></div>`},
		{bs.FormLabel(), `<label class="form-label"></label>`},
		{bs.FormCheckLabel(), `<label class="form-check-label"></label>`},
		{bs.FormText(), `<div class="form-text"></div>`},
		{bs.InputGroup(), `<div class="input-group"></div>`},
		{bs.InputGroupText(), `<span class="input-group-text"></span>`},
		{bs.Table(), `<table class="table"></table>`},
		{bs.TableStriped(), `<table class="table table-striped"></table>`},
		{bs.TableBordered(), `<table class="table table-bordered"></table>`},
		{bs.TableHover(), `<table class="table table-hover"></table>`},
		{bs.TableResponsive(x), `<div class="table-responsive position-relative" id="x"></div>`},
		{bs.ListGroupItem(), `<li class="list-group-item"></li>`},
		{bs.NavbarNav(), `<ul class="navbar-nav"></ul>`},
		{bs.NavbarCollapse(x), `<div class="collapse navbar-collapse" id="x"></div>`},
		{bs.NavbarToggler(), `<button class="navbar-toggler" type="button" data-bs-toggle="collapse" aria-expanded="false" aria-label="Toggle navigation"><span class="navbar-toggler-icon"></span></button>`},
		{bs.DropdownDivider(), `<li><hr class="dropdown-divider"/></li>`},
		{bs.DropdownButton()(), `<li><button class="dropdown-item" type="button"></button></li>`},
	}
	for _, tc := range tests {
		t.Must(be.Equal(tc.want, render(t, tc.node)))
	}
}

// Invalid attribute objects among a caller's attributes fail the render as
// they would without the component's inspection.
func TestInvalidCallerAttributes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errNested := errors.New("nested")
	nested := attr.Bundle{attr.ID("a"), attr.Fn(func(context.Context) (attr.Node, error) { return nil, errNested })}
	t.Must(be.ErrorIs(renderErr(t, bs.Alert(nested)), errNested))
	t.Must(be.ErrorIs(renderErr(t, bs.Alert(attr.Obj{Key: "x"})), attr.ErrInvalidObj))
	t.Must(be.ErrorIs(renderErr(t, bs.Alert(attr.Bundle{attr.Bundle{nil}})), attr.ErrNilNode))
	t.Must(be.Equal(`<div class="alert" role="alert"></div>`, render(t, bs.Alert(attr.Bundle{}, attr.Obj{Kind: attr.KindBundle}))))
}

func TestFieldTypesAndStates(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="c">Color</label><input class="form-control form-control-color" type="color" id="c" value="#336699" disabled readonly/></div>`,
		render(t, bs.Field{ID: "c", Label: text.Text("Color"), Type: input.Color, Value: "#336699", Disabled: true, ReadOnly: true})))
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="r">Level</label><input class="form-range" type="range" id="r" min="0" max="5"/></div>`,
		render(t, bs.Field{ID: "r", Label: text.Text("Level"), Type: input.Range, Attrs: attr.Bundle{input.Min("0"), input.Max("5")}})))
	t.Must(be.Equal(
		`<div class="mb-3 col-md-6"><label class="form-label" for="t">Note</label><textarea class="form-control is-invalid" id="t" placeholder="Write..." autocomplete="off" required disabled readonly aria-invalid="true" aria-describedby="t-error"></textarea><div class="invalid-feedback" id="t-error">Too long.</div></div>`,
		render(t, bs.Textarea{ID: "t", Label: text.Text("Note"), Placeholder: "Write...", Autocomplete: "off", Required: true, Disabled: true, ReadOnly: true,
			Error: text.Text("Too long."), Wrapper: attr.Class("col-md-6")})))
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="s">Tags</label><select class="form-select" id="s" name="tags" multiple required disabled><option value="a" selected disabled>A</option></select></div>`,
		render(t, bs.Select{ID: "s", Name: "tags", Label: text.Text("Tags"), Multiple: true, Required: true, Disabled: true,
			Options: []bs.SelectOption{{Value: "a", Label: "A", Selected: true, Disabled: true}}})))
	t.Must(be.Equal(
		`<div class="form-check"><input class="form-check-input is-invalid" type="radio" id="r1" name="r" value="1" required disabled aria-invalid="true" aria-describedby="r1-error r1-help"/><label class="form-check-label" for="r1">One</label><div class="invalid-feedback" id="r1-error">Pick one.</div><div class="form-text" id="r1-help">Help</div></div>`,
		render(t, bs.Radio{ID: "r1", Name: "r", Value: "1", Label: text.Text("One"), Required: true, Disabled: true, Error: text.Text("Pick one."), Help: text.Text("Help")})))
	// An input group without an error has no has-validation class.
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="p">Price</label><div class="input-group"><span class="input-group-text">$</span><input class="form-control" type="number" id="p" inputmode="decimal"/></div></div>`,
		render(t, bs.Field{ID: "p", Label: text.Text("Price"), Type: input.Number, InputMode: bs.InputModeDecimal, Before: []elem.Node{bs.InputGroupText()(text.Text("$"))}})))
	// With an error, the feedback is the input group's last child, after
	// every After addon: Bootstrap's has-validation corner rules and its
	// .is-invalid ~ .invalid-feedback selector need this. The help text
	// stays outside the group.
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="u">User</label><div class="input-group has-validation">`+
			`<span class="input-group-text">@</span>`+
			`<input class="form-control is-invalid" type="text" id="u" aria-invalid="true" aria-describedby="u-error u-help"/>`+
			`<button class="btn btn-outline-secondary" type="button">Check</button>`+
			`<div class="invalid-feedback" id="u-error">Taken.</div></div>`+
			`<div class="form-text" id="u-help">Help</div></div>`,
		render(t, bs.Field{ID: "u", Label: text.Text("User"), Error: text.Text("Taken."), Help: text.Text("Help"),
			Before: []elem.Node{bs.InputGroupText()(text.Text("@"))},
			After:  []elem.Node{bs.BtnOutlineSecondary()(text.Text("Check"))}})))
	// A boolean aria-describedby adds no ids.
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="b">B</label><input class="form-control" type="text" id="b"/></div>`,
		render(t, bs.Field{ID: "b", Label: text.Text("B"), Attrs: attr.Name("aria-describedby").Bool()})))
}

// WrapperClass replaces the mb-3 of the field wrappers, and the classes of
// Wrapper add to it; the label and ARIA wiring stay.
func TestFieldWrapperClass(gt *testing.T) {
	t := devtest.ParallelT(gt)
	opts := []bs.SelectOption{{Value: "a", Label: "A"}}
	for name, c := range map[string]struct {
		node elem.Node
		want string
	}{
		"field default":    {bs.Field{ID: "f", Label: text.Text("F")}, "mb-3"},
		"field":            {bs.Field{ID: "f", Label: text.Text("F"), WrapperClass: "mb-0"}, "mb-0"},
		"field wrapper":    {bs.Field{ID: "f", Label: text.Text("F"), WrapperClass: "col-auto", Wrapper: attr.Class("flex-grow-1")}, "col-auto flex-grow-1"},
		"textarea default": {bs.Textarea{ID: "t", Label: text.Text("T")}, "mb-3"},
		"textarea":         {bs.Textarea{ID: "t", Label: text.Text("T"), WrapperClass: "col"}, "col"},
		"select default":   {bs.Select{ID: "s", Label: text.Text("S"), Options: opts}, "mb-3"},
		"select":           {bs.Select{ID: "s", Label: text.Text("S"), Options: opts, WrapperClass: "col-md-4 mb-2"}, "col-md-4 mb-2"},
	} {
		t.Run(name, func(t devtest.T) {
			out := render(t, c.node)
			t.Must(be.True(strings.HasPrefix(out, `<div class="`+c.want+`">`)))
		})
	}

	// The wiring of a field with an error and help stays the same.
	page := ct.View(bs.Field{ID: "q", Label: text.Text("Search"), Error: text.Text("Too short."), Help: text.Text("Two letters or more."),
		WrapperClass: "col-auto"})
	t.Must(page.Find(ct.Tag("div")).First().Matches(ct.Class("col-auto"), ct.Class("mb-3").Not()))
	t.Must(page.Find(ct.Role("textbox", ct.Named("Search"))).
		Matches(ct.Attr("aria-invalid", "true"), ct.Attr("aria-describedby", "q-error q-help")))

	// The class is escaped, as attr.Class escapes it.
	page = ct.View(bs.Field{ID: "f", Label: text.Text("F"), WrapperClass: `x" onclick="alert(1)`})
	t.Must(page.Find(ct.HasAttr("onclick")).None())
	t.Must(page.Find(ct.Tag("div")).First().Matches(ct.Attr("class", `x" onclick="alert(1)`)))
}

// Plaintext replaces form-control with form-control-plaintext, not adding
// to it, and makes the control read-only, also without ReadOnly.
func TestPlaintext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="f">Task</label><input class="form-control-plaintext" type="text" id="f" name="task" value="T-1024" readonly/></div>`,
		render(t, bs.Field{ID: "f", Name: "task", Label: text.Text("Task"), Value: "T-1024", Plaintext: true})))
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="f">Due</label><input class="form-control-plaintext" type="date" id="f" value="2026-11-02" readonly/></div>`,
		render(t, bs.Field{ID: "f", Label: text.Text("Due"), Type: input.Date, Value: "2026-11-02", Plaintext: true, ReadOnly: true})))
	t.Must(be.Equal(
		`<div class="mb-3"><label class="form-label" for="t">Note</label><textarea class="form-control-plaintext" id="t" rows="2" readonly>First line`+"\n"+`second line</textarea></div>`,
		render(t, bs.Textarea{ID: "t", Label: text.Text("Note"), Rows: 2, Value: "First line\nsecond line", Plaintext: true})))

	// The wiring stays: the label names the control, and an error marks
	// it invalid.
	for name, node := range map[string]elem.Node{
		"field":    bs.Field{ID: "p", Label: text.Text("Project"), Value: "x", Plaintext: true, Error: text.Text("Archived."), Help: text.Text("Moved with the task.")},
		"textarea": bs.Textarea{ID: "p", Label: text.Text("Project"), Value: "x", Plaintext: true, Error: text.Text("Archived."), Help: text.Text("Moved with the task.")},
	} {
		t.Run(name, func(t devtest.T) {
			page := ct.View(node)
			control := page.Find(ct.Role("textbox", ct.Named("Project")))
			t.Must(control.Matches(ct.Class("form-control-plaintext"), ct.Class("is-invalid"), ct.Class("form-control").Not(),
				ct.HasAttr("readonly"), ct.Attr("aria-invalid", "true"), ct.Attr("aria-describedby", "p-error p-help")))
			t.Must(page.Find(ct.HasAttr("readonly")).Count(1))
		})
	}
}

func TestCardImage(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="x card"><img class="card-img-top" alt=""/><div class="card-body"><h6 class="card-subtitle mb-2 text-body-secondary">Sub</h6>More</div></div>`,
		render(t, bs.Card{Attrs: attr.Class("x"), Img: elem.Name("img").Void(attr.Class("card-img-top"), attr.KV("alt", "")),
			Body: bs.CardBody{Subtitle: text.Text("Sub"), Content: text.Text("More")}})))
}

// A card heading at another level than the default keeps the default's
// size class, as the titles of dialogs and modals keep h5.
func TestCardBodyHeadingSizes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, tc := range []struct {
		name string
		body bs.CardBody
		want string
	}{
		{"defaults", bs.CardBody{Title: text.Text("T"), Subtitle: text.Text("S")},
			`<h5 class="card-title">T</h5><h6 class="card-subtitle mb-2 text-body-secondary">S</h6>`},
		{"explicit defaults", bs.CardBody{Title: text.Text("T"), TitleLevel: 5, Subtitle: text.Text("S"), SubtitleLevel: 6},
			`<h5 class="card-title">T</h5><h6 class="card-subtitle mb-2 text-body-secondary">S</h6>`},
		{"higher", bs.CardBody{Title: text.Text("T"), TitleLevel: 2, Subtitle: text.Text("S"), SubtitleLevel: 3},
			`<h2 class="card-title h5">T</h2><h3 class="card-subtitle h6 mb-2 text-body-secondary">S</h3>`},
		{"lower", bs.CardBody{Title: text.Text("T"), TitleLevel: 6, Subtitle: text.Text("S"), SubtitleLevel: 5},
			`<h6 class="card-title h5">T</h6><h5 class="card-subtitle h6 mb-2 text-body-secondary">S</h5>`},
	} {
		t.Run(tc.name, func(t devtest.T) {
			t.Must(be.Equal(tc.want, render(t, tc.body)))
		})
	}
	// The level, not the size class, is what assistive technology reads.
	page := ct.View(bs.CardBody{Title: text.Text("Projects"), TitleLevel: 2})
	t.Must(page.Find(ct.Role("heading", ct.Named("Projects"))).Matches(ct.Tag("h2"), ct.Class("h5")))
}

// Column.Attrs go on the header cell and every body cell, evaluated once
// per render; Column.HeaderAttrs go on the header cell only.
func TestDataTableColumnAttrs(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rows := []bs.TableRow{
		{Cells: []elem.Node{text.Text("a"), text.Text("1")}},
		{Cells: []elem.Node{text.Text("b"), text.Text("2")}},
		{Cells: []elem.Node{text.Text("c"), text.Text("3")}},
	}
	sorted := bs.DataTable{Columns: []bs.Column{
		{Header: text.Text("Name"), RowHeader: true, Attrs: attr.Class("text-nowrap"), HeaderAttrs: attr.KV("aria-sort", "ascending")},
		{Header: text.Text("Count"), HeaderAttrs: attr.KV("scope", "colgroup")},
	}, Rows: rows}
	page := ct.View(sorted)
	t.Must(page.Find(ct.HasAttr("aria-sort")).Count(1))
	t.Must(page.Find(ct.Tag("thead")).Find(ct.Tag("th")).AttrValues("scope", "col", "colgroup"))
	t.Must(page.Find(ct.Tag("tbody")).Find(ct.Tag("th")).AttrValues("scope", "row", "row", "row"))
	t.Must(page.Find(ct.Tag("tbody")).Find(ct.Tag("td")).Each(ct.HasAttr("scope").Not()))
	t.Must(page.Find(ct.Class("text-nowrap")).Count(4))
	t.Must(be.Equal(
		`<div class="table-responsive position-relative"><table class="table">`+
			`<thead><tr><th scope="col" class="text-nowrap" aria-sort="ascending">Name</th><th scope="colgroup">Count</th></tr></thead>`+
			`<tbody><tr><th scope="row" class="text-nowrap">a</th><td>1</td></tr>`+
			`<tr><th scope="row" class="text-nowrap">b</th><td>2</td></tr>`+
			`<tr><th scope="row" class="text-nowrap">c</th><td>3</td></tr></tbody></table></div>`,
		render(t, sorted)))

	// A lazy node runs once per render, not once per cell.
	calls := 0
	counted := attr.Fn(func(context.Context) (attr.Node, error) {
		calls++
		return attr.Class("text-end"), nil
	})
	counting := bs.DataTable{Columns: []bs.Column{{Header: text.Text("H")}, {Header: text.Text("N"), Attrs: counted}}, Rows: rows}
	render(t, counting)
	t.Must(be.Equal(1, calls))

	// A single-use sequence reaches every cell of its column.
	used := false
	once := attr.Seq(func(yield func(attr.Node) bool) {
		if used {
			return
		}
		used = true
		yield(attr.Class("text-end"))
	})
	t.Must(be.Equal(
		`<div class="table-responsive position-relative"><table class="table">`+
			`<thead><tr><th scope="col" class="text-end">N</th></tr></thead>`+
			`<tbody><tr><td class="text-end">1</td></tr><tr><td class="text-end">2</td></tr></tbody></table></div>`,
		render(t, bs.DataTable{Columns: []bs.Column{{Header: text.Text("N"), Attrs: once}},
			Rows: []bs.TableRow{{Cells: []elem.Node{text.Text("1")}}, {Cells: []elem.Node{text.Text("2")}}}})))
}

func TestDataTableOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(
		`<div class="table-responsive position-relative mb-0"><table class="table table-striped table-bordered" aria-labelledby="h">`+
			`<thead><tr><th scope="col"></th></tr></thead><tbody><tr id="row-1"><td></td></tr></tbody></table></div>`,
		render(t, bs.DataTable{Striped: true, Bordered: true, Attrs: aria.Labelledby("h"), Wrapper: attr.Class("mb-0"),
			Columns: []bs.Column{{}}, Rows: []bs.TableRow{{Cells: []elem.Node{nil}, Attrs: attr.ID("row-1")}}})))
}

// TableResponsive is positioned, as the wrapper of DataTable is, so that
// visually hidden text in the table stays inside the scroller; a caller's
// classes add to it.
func TestTableResponsivePositioned(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(`<div class="table-responsive position-relative mb-0"><table class="table"></table></div>`,
		render(t, bs.TableResponsive(attr.Class("mb-0"))(bs.Table()()))))
	page := ct.View(bs.DataTable{Columns: []bs.Column{{Header: text.Text("Actions"), HeaderHidden: true}}, Wrapper: attr.Class("mb-0")})
	t.Must(page.Find(ct.Class("table-responsive")).Matches(
		ct.Class("position-relative"), ct.Class("mb-0"), ct.HasDescendant(ct.Class("visually-hidden"))))
}

func TestHeadingLevels(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for level := bs.HeadingLevel(1); level <= 6; level++ {
		page := ct.View(bs.Dialog{ID: "d", Title: text.Text("T"), TitleLevel: level})
		t.Must(page.Find(ct.Role("heading", ct.Named("T"), ct.Level(int(level)))).Matches(ct.ID("d-title")))
	}
}
