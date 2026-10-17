package ba_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/ba"
)

// Utilities are attributes for any element; their classes merge into one
// class attribute.
func Example() {
	core.Dump(div.Div(ba.DFlex(), ba.AlignItemsCenter(), ba.Gap(2), ba.MB(3))(
		text.Span(ba.FWSemibold())(text.Text("Tasks")),
		text.Span(ba.MSAuto(), ba.TextMuted())(text.Text("3 open")),
	))
	// Output: <div class="d-flex align-items-center gap-2 mb-3"><span class="fw-semibold">Tasks</span><span class="ms-auto text-body-secondary">3 open</span></div>
}

// Spacing takes the Bootstrap scale 0..5.
func ExampleM() {
	core.Dump(div.Div(ba.M(2), ba.PX(3), ba.PY(1))(text.Text("Note")))
	// Output: <div class="m-2 px-3 py-1">Note</div>
}

// A value without a Bootstrap class fails the render instead of producing a
// class that does not exist.
func ExampleErrInvalidValue() {
	var out strings.Builder
	err := core.Render(context.Background(), div.Div(ba.MT(9))(text.Text("Note")), &out)
	fmt.Println(err)
	fmt.Println(errors.Is(err, ba.ErrInvalidValue))
	// Output:
	// render: at div[0]: attribute: ba: mt-9: invalid utility value (want 0 to 5)
	// true
}

// An auto margin pushes the following items to the end of a flex row.
func ExampleMSAuto() {
	core.Dump(div.Div(ba.DFlex(), ba.Gap(2))(
		text.A(text.Href("/tasks"))(text.Text("Tasks")),
		text.A(text.Href("/settings"), ba.MSAuto())(text.Text("Settings")),
	))
	// Output: <div class="d-flex gap-2"><a href="/tasks">Tasks</a><a href="/settings" class="ms-auto">Settings</a></div>
}

// Responsive utilities apply from a breakpoint upwards: this sidebar shows
// from md up, and the toolbar stacks on small screens.
func ExampleDNoneAt() {
	core.Dump(div.Div(ba.DNone(), ba.DBlockAt(ba.MD))(text.Text("Sidebar")))
	core.Dump(div.Div(ba.DFlex(), ba.FlexColumn(), ba.FlexRowAt(ba.LG), ba.DPrintNone())(text.Text("Toolbar")))
	// Output:
	// <div class="d-none d-md-block">Sidebar</div>
	// <div class="d-flex flex-column flex-lg-row d-print-none">Toolbar</div>
}

// Responsive grid columns.
func ExampleColAt() {
	core.Dump(div.Div(ba.Row(), ba.RowGap(3))(
		div.Div(ba.Col12(), ba.ColAt(ba.MD, 8))(text.Text("Items")),
		div.Div(ba.Col12(), ba.ColMD(4), ba.OffsetAt(ba.XL, 0))(text.Text("Details")),
	))
	// Output: <div class="row row-gap-3"><div class="col-12 col-md-8">Items</div><div class="col-12 col-md-4 offset-xl-0">Details</div></div>
}

// Visually hidden text names an icon-only button for assistive technology.
func ExampleVisuallyHidden() {
	core.Dump(button.Button(button.Type(button.TypeButton), attr.Class("btn btn-outline-secondary"), ba.BtnSM())(
		text.Span(attr.Class("bi bi-trash"), aria.Hidden("true"))(),
		text.Span(ba.VisuallyHidden())(text.Text("Delete note")),
	))
	// Output: <button type="button" class="btn btn-outline-secondary btn-sm"><span class="bi bi-trash" aria-hidden="true"></span><span class="visually-hidden">Delete note</span></button>
}

// A skip link that appears on keyboard focus.
func ExampleVisuallyHiddenFocusable() {
	core.Dump(text.A(text.Href("#main"), ba.VisuallyHiddenFocusable())(text.Text("Skip to content")))
	// Output: <a href="#main" class="visually-hidden-focusable">Skip to content</a>
}

// A server-side validation error on a form control.
func ExampleIsInvalid() {
	core.Dump(div.Div(ba.MB(3))(
		label.Label(label.For("title"), attr.Class("form-label"))(text.Text("Title")),
		input.Input(ba.FormControl(ba.IsInvalid()), attr.ID("title"), input.Name("title"),
			aria.Invalid("true"), aria.Describedby("title-error")),
		div.Div(ba.InvalidFeedback(), attr.ID("title-error"))(text.Text("Enter a title.")),
	))
	// Output: <div class="mb-3"><label for="title" class="form-label">Title</label><input class="form-control is-invalid" id="title" name="title" aria-invalid="true" aria-describedby="title-error"/><div class="invalid-feedback" id="title-error">Enter a title.</div></div>
}

// Small and large form controls and buttons.
func ExampleFormControlSM() {
	core.Dump(input.Input(ba.FormControl(ba.FormControlSM()), input.Name("q")))
	core.Dump(button.Button(button.Type(button.TypeSubmit), attr.Class("btn btn-primary"), ba.BtnLG())(text.Text("Save")))
	// Output:
	// <input class="form-control form-control-sm" name="q"/>
	// <button type="submit" class="btn btn-primary btn-lg">Save</button>
}

// Theme colors: a contrasting text and background pair, and the subtle
// variants that follow the color mode.
func ExampleTextBgPrimary() {
	core.Dump(text.Span(ba.TextBgPrimary(), ba.PX(2), ba.RoundedPill())(text.Text("New")))
	core.Dump(div.Div(ba.BgDangerSubtle(), ba.TextDangerEmphasis(), ba.Border(), ba.BorderDangerSubtle(), ba.RoundedSize(3), ba.P(2))(
		text.Text("Sync failed"),
	))
	// Output:
	// <span class="text-bg-primary px-2 rounded-pill">New</span>
	// <div class="bg-danger-subtle text-danger-emphasis border border-danger-subtle rounded-3 p-2">Sync failed</div>
}

// Typography helpers.
func ExampleFS() {
	core.Dump(text.P(ba.FS(4), ba.FW("medium"), ba.LH("sm"), ba.TextTruncate())(text.Text("Quarterly planning notes")))
	// Output: <p class="fs-4 fw-medium lh-sm text-truncate">Quarterly planning notes</p>
}

// Stacks lay out items with a gap, without flex boilerplate.
func ExampleHStack() {
	core.Dump(div.Div(ba.HStack(), ba.Gap(3))(
		div.Div(ba.BorderWidth(2), ba.Border())(text.Text("First")),
		div.Div(ba.VR())(),
		div.Div()(text.Text("Second")),
	))
	// Output: <div class="hstack gap-3"><div class="border-2 border">First</div><div class="vr"></div><div>Second</div></div>
}

// A header that sticks to the top of the viewport.
func ExampleStickyTop() {
	core.Dump(div.Div(ba.StickyTop(), ba.BgBodyTertiary(), ba.ShadowSM())(text.Text("Header")))
	core.Dump(div.Div(ba.StickyTopAt(ba.LG))(text.Text("Sidebar")))
	// Output:
	// <div class="sticky-top bg-body-tertiary shadow-sm">Header</div>
	// <div class="sticky-lg-top">Sidebar</div>
}

// Links in a theme color, and a link that inherits its color.
func ExampleLinkBodyEmphasis() {
	core.Dump(text.A(text.Href("/projects/7"), ba.LinkBodyEmphasis(), ba.TextDecorationNone())(text.Text("Website redesign")))
	core.Dump(text.A(text.Href("/help"), ba.LinkSecondary())(text.Text("Help")))
	// Output:
	// <a href="/projects/7" class="link-body-emphasis text-decoration-none">Website redesign</a>
	// <a href="/help" class="link-secondary">Help</a>
}

// A Bootstrap tooltip needs Bootstrap JavaScript and initialization code. The
// title attribute alone is the no-JavaScript alternative.
func ExampleTooltip() {
	core.Dump(button.Button(button.Type(button.TypeButton), ba.Tooltip(attr.Title("Copy link")))(text.Text("Copy")))
	core.Dump(button.Button(button.Type(button.TypeButton), attr.Title("Copy link"))(text.Text("Copy")))
	// Output:
	// <button type="button" data-bs-toggle="tooltip" title="Copy link">Copy</button>
	// <button type="button" title="Copy link">Copy</button>
}

// The breakpoint infixes.
func ExampleBreakpoint() {
	for _, bp := range []ba.Breakpoint{ba.SM, ba.MD, ba.LG, ba.XL, ba.XXL, 0} {
		fmt.Println(bp)
	}
	// Output:
	// sm
	// md
	// lg
	// xl
	// xxl
	// Breakpoint(0)
}
