package bs_test

import (
	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	selectel "github.com/protolambda/chord/html/form/select"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/ba"
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

func ExampleBtnGroup() {
	core.Dump(bs.BtnGroup()(
		bs.BtnPrimary()(text.Text("Left")),
		bs.BtnPrimary()(text.Text("Right")),
	))
	// Output: <div class="btn-group"><button class="btn btn-primary" type="button">Left</button><button class="btn btn-primary" type="button">Right</button></div>
}

// Badges

func ExampleBadgePrimary() {
	core.Dump(bs.BadgePrimary()(text.Text("New")))
	// Output: <span class="badge text-bg-primary">New</span>
}

func ExampleBadgeSuccess() {
	core.Dump(bs.BadgeSuccess()(text.Text("Completed")))
	// Output: <span class="badge text-bg-success">Completed</span>
}

// Alerts

func ExampleAlertDanger() {
	core.Dump(bs.AlertDanger()(text.Text("Error occurred!")))
	// Output: <div class="alert alert-danger">Error occurred!</div>
}

func ExampleAlertSuccess() {
	core.Dump(bs.AlertSuccess()(text.Text("Operation completed.")))
	// Output: <div class="alert alert-success">Operation completed.</div>
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
	// Output: <div class="col-md-4"></div>
}

// Spacing (using ba package)

func ExampleMT() {
	core.Dump(div.Div(ba.MT(3))(text.Text("Margin top 3")))
	// Output: <div class="mt-3">Margin top 3</div>
}

func ExamplePX() {
	core.Dump(div.Div(ba.PX(4))(text.Text("Padding X 4")))
	// Output: <div class="px-4">Padding X 4</div>
}

// Flex (using ba package)

func ExampleDFlex() {
	core.Dump(div.Div(ba.DFlex(), ba.JustifyContentBetween(), ba.AlignItemsCenter()))
	// Output: <div class="d-flex justify-content-between align-items-center"></div>
}

func ExampleFlexColumn() {
	core.Dump(div.Div(ba.DFlex(), ba.FlexColumn()))
	// Output: <div class="d-flex flex-column"></div>
}

func ExampleGap() {
	core.Dump(div.Div(ba.DFlex(), ba.Gap(3)))
	// Output: <div class="d-flex gap-3"></div>
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

// Nav

func ExampleNav() {
	core.Dump(bs.Nav()(
		bs.NavItem()(bs.NavLink(text.Href("#"))(text.Text("Home"))),
		bs.NavItem()(bs.NavLink(text.Href("#"))(text.Text("About"))),
	))
	// Output: <ul class="nav"><li class="nav-item"><a class="nav-link" href="#">Home</a></li><li class="nav-item"><a class="nav-link" href="#">About</a></li></ul>
}

func ExampleNavbar() {
	core.Dump(bs.Navbar(ba.BgDark(), attr.Data("bs-theme", "dark"))(
		bs.NavbarBrand(text.Href("#"))(text.Text("Brand")),
	))
	// Output: <nav class="navbar bg-dark" data-bs-theme="dark"><a class="navbar-brand" href="#">Brand</a></nav>
}

// Modal trigger

func ExampleModalTrigger() {
	core.Dump(bs.ModalTrigger("myModal")(text.Text("Open Modal")))
	// Output: <button type="button" data-bs-toggle="modal" data-bs-target="#myModal">Open Modal</button>
}

// Offcanvas trigger

func ExampleOffcanvasTrigger() {
	core.Dump(bs.OffcanvasTrigger("myOffcanvas")(text.Text("Open Menu")))
	// Output: <button type="button" data-bs-toggle="offcanvas" data-bs-target="#myOffcanvas">Open Menu</button>
}

// Combined utilities

func Example_combined() {
	core.Dump(bs.Container(ba.MT(4), ba.P(3), ba.BgLight(), ba.Rounded())(
		text.Text("Styled container"),
	))
	// Output: <div class="container mt-4 p-3 bg-light rounded">Styled container</div>
}

// Complex layout

func Example_layout() {
	core.Dump(bs.Container()(
		bs.Row()(
			bs.ColMD(4, ba.MB(3))(bs.Card{Body: text.Text("Card 1")}),
			bs.ColMD(4, ba.MB(3))(bs.Card{Body: text.Text("Card 2")}),
			bs.ColMD(4, ba.MB(3))(bs.Card{Body: text.Text("Card 3")}),
		),
	))
	// Output: <div class="container"><div class="row"><div class="col-md-4 mb-3"><div class="card"><div class="card-body">Card 1</div></div></div><div class="col-md-4 mb-3"><div class="card"><div class="card-body">Card 2</div></div></div><div class="col-md-4 mb-3"><div class="card"><div class="card-body">Card 3</div></div></div></div></div>
}

// Form elements

func Example_form() {
	core.Dump(div.Div(attr.Class("mb-3"))(
		bs.FormLabel(label.For("email"))(text.Text("Email")),
		input.Input(ba.FormControl(), attr.ID("email"), input.Type("email")),
	))
	// Output: <div class="mb-3"><label class="form-label" for="email">Email</label><input class="form-control" id="email" type="email"/></div>
}

func ExampleFormSelect() {
	core.Dump(selectel.Select(ba.FormSelect())(
		selectel.Option()(text.Text("Option 1")),
		selectel.Option()(text.Text("Option 2")),
	))
	// Output: <select class="form-select"><option>Option 1</option><option>Option 2</option></select>
}

func ExampleFormCheck() {
	core.Dump(bs.FormCheck()(
		input.Input(ba.FormCheckInput(), attr.ID("check1"), input.Type("checkbox")),
		bs.FormCheckLabel(label.For("check1"))(text.Text("Check me")),
	))
	// Output: <div class="form-check"><input class="form-check-input" id="check1" type="checkbox"/><label class="form-check-label" for="check1">Check me</label></div>
}
