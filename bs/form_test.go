package bs_test

import (
	"net/url"
	"testing"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/webuitest"
)

const testToken = "dG9rZW4"

// The htmx form of the package documentation (ExampleBtnPrimary_htmxForm)
// also works as a plain form: without htmx it posts to the same URL, with
// the CSRF field, and the token never ends up in a URL.
func TestHTMXFormWorksWithoutScript(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(form.Form(form.Method(form.MethodPost), form.Action("/notes"), hx2.Post("/notes"), hx2.Swap(hx2.SwapOuterHTML))(
		session.CSRFInput(testToken),
		bs.Field{ID: "note-title", Name: "title", Label: text.Text("Title"), Value: "Groceries", Error: text.Text("Enter a title.")},
		bs.BtnPrimary(button.Type(button.TypeSubmit), attr.ID("note-save"))(text.Text("Save")),
	))
	t.Must(page.Valid(
		ct.UniqueIDs(),
		ct.ARIAReferences(),
		ct.LabelReferences(),
		webuitest.PostFormsCarryCSRF(testToken),
		webuitest.NoTokenLeaks(testToken),
	))
	sub, err := webuitest.FormData(t.Context(), page, page.Find(ct.Tag("form")), page.Find(ct.ID("note-save")))
	t.Must(be.NoError(err))
	t.Must(be.Equal("POST", sub.Method))
	t.Must(be.Equal("/notes", sub.Action))
	t.Must(be.DeepEqual(url.Values{session.CSRFField: {testToken}, "title": {"Groceries"}}, sub.Values()))
}

// Without method and action, the form that htmx posts falls back to a GET
// of the page's URL: the rules report the missing token, and the token in
// the URL once the field is there.
func TestHTMXFormWithoutMethodFallsBackToGet(gt *testing.T) {
	t := devtest.ParallelT(gt)
	field := bs.Field{ID: "note-title", Name: "title", Label: text.Text("Title")}
	save := bs.BtnPrimary(button.Type(button.TypeSubmit), attr.ID("note-save"))(text.Text("Save"))

	bare := ct.View(form.Form(hx2.Post("/notes"), hx2.Swap(hx2.SwapOuterHTML))(field, save))
	sub, err := webuitest.FormData(t.Context(), bare, bare.Find(ct.Tag("form")), nil)
	t.Must(be.NoError(err))
	t.Must(be.Equal("GET", sub.Method))
	t.Must(be.Equal("", sub.Action))
	got := violations(t, bare, webuitest.PostFormsCarryCSRF(testToken))
	t.Must(be.Len(got, 1))
	t.Must(be.Substring(got[0], `hx-post at form[0] sends no "csrf" field`))

	withToken := ct.View(form.Form(hx2.Post("/notes"), hx2.Swap(hx2.SwapOuterHTML))(session.CSRFInput(testToken), field, save))
	t.Must(withToken.Valid(webuitest.PostFormsCarryCSRF(testToken)))
	got = violations(t, withToken, webuitest.NoTokenLeaks(testToken))
	t.Must(be.Len(got, 1))
	t.Must(be.Substring(got[0], "get form at form[0]"))
}

// violations loads subject and returns what rule reports for it.
func violations(t devtest.T, subject *ct.Subject, rule ct.Rule) []string {
	t.Helper()
	doc, err := subject.Load(t.Context())
	t.Must(be.NoError(err))
	return rule.Violations(doc.Root())
}
