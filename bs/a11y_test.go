package bs_test

import (
	"strings"
	"testing"

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

// noStyle forbids style attributes, which a strict Content-Security-Policy
// blocks.
var noStyle = ct.CustomRule("noStyle()", func(root ct.Node) []string {
	var out []string
	walk(root, func(n ct.Node) {
		if _, ok := n.Attr("style"); ok {
			out = append(out, "style attribute at "+n.Location())
		}
	})
	return out
})

// buttonsHaveType requires every button, inside a form or not, to state
// its type.
var buttonsHaveType = ct.CustomRule("allButtonsHaveType()", func(root ct.Node) []string {
	var out []string
	walk(root, func(n ct.Node) {
		if n.Tag() == "button" {
			if _, ok := n.Attr("type"); !ok {
				out = append(out, "button without type at "+n.Location())
			}
		}
	})
	return out
})

func walk(n ct.Node, fn func(ct.Node)) {
	for c := range n.Children() {
		if c.Kind() == ct.KindElement {
			fn(c)
			walk(c, fn)
		}
	}
}

// gallery renders every component once, with all optional parts.
func gallery() elem.Node {
	return elem.Bundle{
		form.Form(attr.Name("method").Raw("post"))(
			bs.BtnGroup(aria.Label("Sort"))(bs.BtnPrimary()(text.Text("Up")), bs.BtnOf(bs.VariantDark)(text.Text("Down"))),
			bs.BtnToolbar(aria.Label("Tools"))(bs.BtnOutlineOf(bs.VariantInfo)(text.Text("Tool"))),
			bs.CloseButton(),
			bs.AlertOf(bs.VariantWarning)(text.Text("Warning")),
			bs.AlertDismissible()(text.Text("Dismissible")),
			bs.Card{Header: text.Text("Card"), Body: bs.CardBody{Title: text.Text("Title"), Subtitle: text.Text("Sub"), Text: text.Text("Text")}},
			bs.Breadcrumb()(bs.BreadcrumbItem()(text.A(text.Href("/"))(text.Text("Home"))), bs.BreadcrumbActive()(text.Text("Here"))),
			bs.Pagination()(bs.PageItemActive()(bs.PageLinkCurrent(text.Href("?p=1"))(text.Text("1")))),
			bs.Navbar(aria.Label("Main"))(
				bs.NavbarTogglerFor("menu"),
				bs.NavbarCollapse(attr.ID("menu"))(bs.NavbarNav()(bs.NavItem()(bs.NavLinkActive(text.Href("/"))(text.Text("Home"))))),
			),
			bs.NavTabs()(bs.NavItem()(bs.NavLinkDisabled()(text.Text("Off")))),
			bs.ListGroup()(bs.ListGroupItemActive()(text.Text("Item"))),
			bs.Progress(aria.Label("Upload"), bs.ProgressValue(25, 0, 100))(bs.ProgressBar(attr.Class("w-25"))()),
			bs.Spinner(),
			bs.ToastContainer()(bs.Toast{Header: text.Text("Toast"), Body: text.Text("Body"), Shown: true}),
			bs.Accordion{ID: "acc", Items: []bs.AccordionItem{{Header: text.Text("Q"), Body: text.Text("A"), Show: true}, {Header: text.Text("Q2")}}},
			bs.DetailsAccordion{ID: "dacc", Items: []bs.AccordionItem{{Header: text.Text("Q"), Body: text.Text("A")}}},
			bs.Dropdown{Toggle: text.Text("Menu"), Items: []elem.Node{bs.DropdownItem(text.Href("/a"))(text.Text("A"))}},
			bs.DetailsDropdown{Toggle: text.Text("More"), Items: []elem.Node{bs.DropdownButton(button.Type(button.TypeSubmit))(text.Text("Go"))}},
			bs.Field{ID: "title", Name: "title", Label: text.Text("Title"), Help: text.Text("Help"), Error: text.Text("Error"),
				Before: []elem.Node{bs.InputGroupText()(text.Text("#"))}},
			bs.Textarea{ID: "body", Name: "body", Label: text.Text("Body"), Help: text.Text("Help"), Error: text.Text("Error")},
			bs.Select{ID: "prio", Name: "prio", Label: text.Text("Priority"), Options: []bs.SelectOption{{Value: "1", Label: "One"}}, Help: text.Text("Help")},
			bs.Checkbox{ID: "done", Name: "done", Label: text.Text("Done"), Error: text.Text("Error"), Help: text.Text("Help")},
			bs.Checkbox{ID: "notify", Name: "notify", Label: text.Text("Notify"), Switch: true},
			bs.Radio{ID: "r1", Name: "r", Value: "1", Label: text.Text("One")},
			bs.DataTable{Caption: text.Text("Items"), Columns: []bs.Column{{Header: text.Text("Name"), RowHeader: true}, {Header: text.Text("Actions"), HeaderHidden: true}},
				Rows: []bs.TableRow{{Cells: []elem.Node{text.Text("A"), bs.Btn(bs.DialogOpen("dlg"))(text.Text("Open"))}}}},
		),
		div.Div(attr.ID("dialogs"))(
			bs.Modal{ID: "modal", Title: text.Text("Modal"), Body: text.Text("Body"), Footer: text.Text("Footer")},
			bs.Offcanvas{ID: "side", Title: text.Text("Side"), Body: text.Text("Body")},
			bs.Dialog{ID: "dlg", Title: text.Text("Dialog"), Body: text.Text("Body"), Footer: bs.Btn(bs.DialogClose("dlg"))(text.Text("Cancel"))},
		),
		bs.ModalTrigger("modal")(text.Text("Modal")),
		bs.OffcanvasTrigger("side")(text.Text("Side")),
		bs.DialogButton("dlg")(text.Text("Dialog")),
	}
}

func TestGalleryFollowsDocumentRules(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(gallery())
	t.Must(page.Valid(
		ct.UniqueIDs(),
		ct.ARIAReferences(),
		ct.LabelReferences(),
		ct.ButtonsHaveType(),
		buttonsHaveType,
		ct.NoInlineHandlers(),
		ct.NoRaw(),
		ct.ImagesHaveAlt(),
		noStyle,
	))
	// Every button has an accessible name.
	nodes, err := page.Find(ct.Tag("button")).Nodes(t.Context())
	t.Must(be.NoError(err))
	t.Must(be.NonEmpty(nodes))
	for _, n := range nodes {
		name, err := ct.AccessibleName(n)
		t.Must(be.NoError(err))
		t.Mustf(be.NotEqual("", name), "button at %s has a name", n.Location())
	}
}

func TestLandmarksAndStatusRoles(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(gallery())
	t.Must(page.Find(ct.Role("navigation", ct.Named("Breadcrumb"))))
	t.Must(page.Find(ct.Role("navigation", ct.Named("Pagination"))))
	t.Must(page.Find(ct.Role("navigation", ct.Named("Main"))))
	t.Must(page.Find(ct.Role("group", ct.Named("Sort"))))
	t.Must(page.Find(ct.Role("toolbar", ct.Named("Tools"))))
	t.Must(page.Find(ct.Role("progressbar", ct.Named("Upload"))).Matches(
		ct.Attr("aria-valuenow", "25"), ct.Attr("aria-valuemin", "0"), ct.Attr("aria-valuemax", "100")))
	t.Must(page.Find(ct.Role("status"), ct.Class("spinner-border")).Matches(ct.TextContent("Loading...")))
	// The toast container is the live region of its toasts, which declare none of their own.
	toasts := page.Find(ct.Class("toast-container"))
	t.Must(toasts.Matches(ct.Attr("aria-live", "polite"), ct.HasAttr("role").Not()))
	t.Must(toasts.Find(ct.Class("toast")).Matches(
		ct.HasAttr("role").Not(), ct.HasAttr("aria-live").Not(), ct.HasAttr("aria-atomic").Not()))
	t.Must(page.Find(ct.Role("alert"), ct.Class("alert")).Count(2))
	t.Must(page.Find(ct.Role("listitem"), ct.Class("breadcrumb-item"), ct.Attr("aria-current", "page")).Matches(ct.TextContent("Here")))
	t.Must(page.Find(ct.Role("link", ct.Named("1"))).Matches(ct.Attr("aria-current", "page")))
	t.Must(page.Find(ct.Role("link", ct.Named("Home")), ct.Class("nav-link")).Matches(ct.Attr("aria-current", "page"), ct.Class("active")))
	t.Must(page.Find(ct.Class("list-group-item")).Matches(ct.Attr("aria-current", "true")))
}

func TestInteractiveNamesAndStates(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(gallery())
	t.Must(page.Find(ct.Role("button", ct.Named("Toggle navigation"))).Matches(
		ct.Attr("aria-controls", "menu"), ct.Attr("aria-expanded", "false")))
	t.Must(page.Find(ct.Role("button", ct.Named("Menu"))).Matches(ct.Attr("aria-expanded", "false")))
	t.Must(page.Find(ct.Role("button", ct.Named("Q"))).Matches(ct.Attr("aria-expanded", "true"), ct.Attr("aria-controls", "acc-collapse-0")))
	t.Must(page.Find(ct.Role("button", ct.Named("Q2"))).Matches(ct.Attr("aria-expanded", "false"), ct.Attr("aria-controls", "acc-collapse-1")))
	t.Must(page.Find(ct.Role("heading", ct.Named("Modal"), ct.Level(2))).Matches(ct.ID("modal-title")))
	t.Must(page.Find(ct.ID("modal")).Matches(ct.Attr("aria-labelledby", "modal-title"), ct.Attr("aria-hidden", "true")))
	t.Must(page.Find(ct.Role("heading", ct.Named("Side"), ct.Level(2))).Matches(ct.ID("side-title")))
	t.Must(page.Find(ct.ID("side")).Matches(ct.Attr("aria-labelledby", "side-title")))
	t.Must(page.Find(ct.Role("dialog", ct.Named("Dialog"))).Matches(ct.ID("dlg")))
	t.Must(page.Find(ct.Role("group"), ct.Tag("details")).Count(2))
	t.Must(page.Find(ct.Role("table", ct.Named("Items"))))
	t.Must(page.Find(ct.Role("columnheader")).Count(2))
	t.Must(page.Find(ct.Role("columnheader")).Each(ct.Attr("scope", "col")))
	t.Must(page.Find(ct.Role("rowheader", ct.Named("A"))))
	t.Must(page.Find(ct.Class("table-responsive")).Matches(ct.Class("position-relative")))
}

func TestFieldsAreLabelledAndDescribed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(gallery())
	t.Must(page.Find(ct.Role("textbox", ct.Named("Title"))).Matches(
		ct.Class("is-invalid"), ct.Attr("aria-invalid", "true"), ct.Attr("aria-describedby", "title-error title-help")))
	t.Must(page.Find(ct.ID("title-error")).Matches(ct.Class("invalid-feedback"), ct.TextContent("Error")))
	t.Must(page.Find(ct.ID("title-help")).Matches(ct.Class("form-text"), ct.TextContent("Help")))
	// In an input group, the error sits inside the group after the input.
	t.Must(page.Find(ct.Class("input-group")).Matches(ct.Class("has-validation"), ct.HasChild(ct.ID("title-error"))))
	t.Must(page.Find(ct.Role("textbox", ct.Named("Body"))).Matches(ct.Tag("textarea"), ct.Attr("aria-describedby", "body-error body-help")))
	t.Must(page.Find(ct.Role("combobox", ct.Named("Priority"))).Matches(ct.Attr("aria-describedby", "prio-help")))
	t.Must(page.Find(ct.Role("combobox", ct.Named("Priority"))).Matches(ct.HasAttr("aria-invalid").Not()))
	t.Must(page.Find(ct.Role("checkbox", ct.Named("Done"))).Matches(ct.Attr("aria-invalid", "true"), ct.Attr("aria-describedby", "done-error done-help")))
	t.Must(page.Find(ct.Role("switch", ct.Named("Notify"))).Matches(ct.Attr("type", "checkbox")))
	t.Must(page.Find(ct.Role("radio", ct.Named("One"))))
}

// A caller's aria-describedby is appended to the field's own ids, so that
// both are read.
func TestFieldMergesDescribedBy(gt *testing.T) {
	t := devtest.ParallelT(gt)
	node := elem.Bundle{
		text.P(attr.ID("rules"))(text.Text("Rules")),
		bs.Field{ID: "name", Label: text.Text("Name"), Help: text.Text("Help"), Error: text.Text("Error"),
			Attrs: attr.Bundle{aria.Describedby("rules"), input.Maxlength("20")}},
		bs.Field{ID: "raw", Label: text.Text("Raw"), Attrs: attr.Name("ARIA-DESCRIBEDBY").Raw("rules")},
	}
	page := ct.View(node)
	t.Must(page.Find(ct.ID("name")).Matches(ct.Attr("aria-describedby", "name-error name-help rules"), ct.Attr("maxlength", "20")))
	t.Must(page.Find(ct.ID("raw")).Matches(ct.Attr("aria-describedby", "rules")))
	t.Must(page.Valid(ct.ARIAReferences(), ct.UniqueIDs(), ct.LabelReferences()))
}

// The dialog follows the markup contract that the theme CSS and the webui
// test rules rely on.
func TestDialogContract(gt *testing.T) {
	t := devtest.ParallelT(gt)
	node := elem.Bundle{
		bs.BtnPrimary(bs.DialogOpen("edit"))(text.Text("Edit")),
		bs.Dialog{
			ID:     "edit",
			Title:  text.Text("Edit the task"),
			Body:   text.Text("Body"),
			Footer: bs.BtnSecondary(bs.DialogClose("edit"))(text.Text("Cancel")),
			Size:   bs.ModalXL,
		},
	}
	page := ct.View(node)
	t.Must(page.Valid(ct.UniqueIDs(), ct.ARIAReferences(), buttonsHaveType, noStyle, ct.NoInlineHandlers()))

	dialog := page.Find(ct.Tag("dialog"))
	t.Must(dialog.Matches(ct.ID("edit"), ct.Class("modal"), ct.HasAttr("popover"), ct.Attr("popover", ""),
		ct.Attr("aria-labelledby", "edit-title"), ct.HasAttr("open").Not()))
	t.Must(page.Find(ct.Role("dialog", ct.Named("Edit the task"))))

	// dialog > .modal-dialog.modal-xl > .modal-content > header, body, footer
	t.Must(dialog.Matches(ct.HasChild(ct.Class("modal-dialog").And(ct.Class("modal-xl"),
		ct.HasChild(ct.Class("modal-content").And(
			ct.HasChild(ct.Class("modal-header")),
			ct.HasChild(ct.Class("modal-body")),
			ct.HasChild(ct.Class("modal-footer")),
		))))))
	t.Must(dialog.Find(ct.Class("modal-header")).Matches(ct.HasChild(ct.Tag("h2").And(ct.ID("edit-title"), ct.Class("modal-title"), ct.Class("h5")))))

	// Exactly one autofocus control: the header's close button.
	t.Must(dialog.Find(ct.HasAttr("autofocus")).Matches(
		ct.Tag("button"), ct.Class("btn-close"), ct.Attr("type", "button"), ct.Attr("aria-label", "Close"),
		ct.Attr("commandfor", "edit"), ct.Attr("command", "close"),
		ct.Attr("popovertarget", "edit"), ct.Attr("popovertargetaction", "hide")))

	// The opener and the footer closer.
	t.Must(page.Find(ct.Attr("command", "show-modal")).Matches(
		ct.Tag("button"), ct.Attr("type", "button"), ct.Attr("commandfor", "edit"), ct.Attr("popovertarget", "edit"),
		ct.HasAttr("popovertargetaction").Not()))
	t.Must(dialog.Find(ct.Attr("command", "close")).Count(2))
	t.Must(dialog.Find(ct.Attr("command", "close")).Each(ct.Attr("popovertargetaction", "hide"), ct.Attr("type", "button")))
	// No popovertarget without commandfor.
	t.Must(page.Find(ct.HasAttr("popovertarget").And(ct.HasAttr("commandfor").Not())).None())
}

func TestDialogOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(bs.Dialog{
		ID: "pick", Attrs: attr.Bundle{aria.Label("Pick an item"), attr.Class("fade-in")},
		CloseLabel: "Close the picker", Body: text.Text("Items"),
	})
	t.Must(page.Find(ct.Role("dialog", ct.Named("Pick an item"))).Matches(ct.Class("modal"), ct.Class("fade-in"), ct.HasAttr("aria-labelledby").Not()))
	t.Must(page.Find(ct.Role("heading")).None())
	t.Must(page.Find(ct.Role("button", ct.Named("Close the picker"))).Matches(ct.HasAttr("autofocus")))
}

// Without Bootstrap's script, no-JS components carry no data-bs-* hooks.
func TestNoJSComponentsHaveNoBootstrapHooks(gt *testing.T) {
	t := devtest.ParallelT(gt)
	noJS := map[string]elem.Node{
		"dialog":            bs.Dialog{ID: "d", Title: text.Text("T"), Body: text.Text("B"), Footer: bs.Btn(bs.DialogClose("d"))(text.Text("X"))},
		"dialog button":     bs.DialogButton("d")(text.Text("Open")),
		"details accordion": bs.DetailsAccordion{ID: "a", Items: []bs.AccordionItem{{Header: text.Text("Q"), Body: text.Text("A")}}},
		"details dropdown":  bs.DetailsDropdown{Toggle: text.Text("T"), Items: []elem.Node{bs.DropdownItem()(text.Text("I")), bs.DropdownButton()(text.Text("B"))}},
		"shown toast":       bs.Toast{Header: text.Text("T"), Body: text.Text("B"), Shown: true, Close: elem.Noop()},
		"field":             bs.Field{ID: "f", Label: text.Text("F")},
		"table":             bs.DataTable{Columns: []bs.Column{{Header: text.Text("H")}}},
	}
	hook := ct.Custom("data-bs-* attribute", func(n ct.Node) (bool, error) {
		for k := range n.Attrs() {
			if strings.HasPrefix(k, "data-bs-") {
				return true, nil
			}
		}
		return false, nil
	})
	for name, node := range noJS {
		t.Run(name, func(t devtest.T) {
			t.Must(ct.View(node).Find(hook).None())
		})
	}
}
