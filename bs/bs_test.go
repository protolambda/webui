package bs_test

import (
	"context"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
)

func render(t *testing.T, node elem.Node) string {
	t.Helper()
	var out strings.Builder
	if err := core.Render(context.Background(), node, &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	return out.String()
}

func expectHTML(t *testing.T, node elem.Node, want string) {
	t.Helper()
	if got := render(t, node); got != want {
		t.Fatalf("rendered output mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestOffcanvasOptionalFields(t *testing.T) {
	expectHTML(t, bs.Offcanvas{ID: "menu"},
		`<div class="offcanvas offcanvas-start" id="menu" tabindex="-1" data-bs-backdrop="false">`+
			`<div class="offcanvas-header">`+
			`<button class="btn-close" type="button" data-bs-dismiss="offcanvas" aria-label="Close"></button>`+
			`</div></div>`)
	expectHTML(t, bs.Offcanvas{ID: "menu", Title: text.Text("Menu"), Body: text.Text("Links")},
		`<div class="offcanvas offcanvas-start" id="menu" tabindex="-1" data-bs-backdrop="false">`+
			`<div class="offcanvas-header">`+
			`<h5 class="offcanvas-title">Menu</h5>`+
			`<button class="btn-close" type="button" data-bs-dismiss="offcanvas" aria-label="Close"></button>`+
			`</div><div class="offcanvas-body">Links</div></div>`)
}

func TestAccordionOptionalFields(t *testing.T) {
	expectHTML(t, bs.Accordion{ID: "faq", Items: []bs.AccordionItem{{}}},
		`<div class="accordion" id="faq"><div class="accordion-item" id="faq-item-0">`+
			`<h2 class="accordion-header">`+
			`<button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#faq-collapse-0"></button>`+
			`</h2><div id="faq-collapse-0" class="accordion-collapse collapse" data-bs-parent="#faq">`+
			`<div class="accordion-body"></div></div></div></div>`)
}

func TestDropdownOptionalFields(t *testing.T) {
	expectHTML(t, bs.Dropdown{},
		`<div class="dropdown">`+
			`<button class="btn btn-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown"></button>`+
			`<ul class="dropdown-menu"></ul></div>`)
}

func TestPaginationMarkup(t *testing.T) {
	expectHTML(t, bs.Pagination()(
		bs.PageItem()(bs.PageLink(text.Href("?page=1"))(text.Text("1"))),
	),
		`<nav><ul class="pagination"><li class="page-item"><a class="page-link" href="?page=1">1</a></li></ul></nav>`)
}

func TestCloseButtonsHaveAccessibleName(t *testing.T) {
	components := map[string]elem.Node{
		"modal":             bs.Modal{ID: "m", Title: text.Text("Title"), Body: text.Text("Body")},
		"toast":             bs.Toast{Header: text.Text("Header"), Body: text.Text("Body")},
		"offcanvas":         bs.Offcanvas{ID: "o", Title: text.Text("Menu")},
		"dismissible alert": bs.AlertDismissible()(text.Text("Saved")),
	}
	for name, node := range components {
		t.Run(name, func(t *testing.T) {
			closeBtn := ct.View(node).Find(ct.Role("button", ct.Named("Close")))
			if err := closeBtn.Matches(ct.Class("btn-close")).Check(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBtnDefaultsToTypeButton(t *testing.T) {
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
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			expectHTML(t, tc.node, tc.want)
		})
	}

	page := ct.View(form.Form()(bs.BtnPrimary()(text.Text("Cancel")), bs.BtnSuccess(button.Type(button.TypeSubmit))(text.Text("Save"))))
	if err := page.Valid(ct.ButtonsHaveType()).Check(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestBtnDynamicTypeIsADuplicate(t *testing.T) {
	dynamic := attr.Fn(func(context.Context) (attr.Node, error) { return button.Type(button.TypeSubmit), nil })
	var out strings.Builder
	err := core.Render(context.Background(), bs.BtnPrimary(dynamic), &out)
	if err == nil || !strings.Contains(err.Error(), `duplicate attribute "type"`) {
		t.Fatalf("expected a duplicate type attribute, got %v", err)
	}
}
