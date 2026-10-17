// Package bs provides Bootstrap 5.3 element components.
//
// This package contains Bootstrap components that create HTML elements.
// For Bootstrap attribute utilities (spacing, colors, flexbox, etc.),
// use the [ba] package.
//
// # Grid Elements
//
// Grid functions create div elements with Bootstrap grid classes:
//
//	bs.Container(
//	    bs.Row(
//	        bs.Col6(text.Text("Left")),
//	        bs.Col6(text.Text("Right")),
//	    ),
//	)
//
// # Components
//
// The package provides element constructors for Bootstrap components:
//
//   - Buttons: [Btn], [BtnPrimary], [BtnOutlinePrimary], [BtnGroup]
//     (type="button" unless a type is passed, e.g. button.Type(button.TypeSubmit))
//   - Badges: [Badge], [BadgePrimary], [BadgeSuccess]
//   - Alerts: [Alert], [AlertDanger], [AlertDismissible]
//   - Cards: [Card], [CardBody], [CardImgOverlay]
//   - Navigation: [Nav], [NavItem], [NavLink], [Navbar], [NavbarBrand]
//   - Lists: [ListGroup], [ListGroupItem]
//   - Tables: [Table], [TableStriped], [TableResponsive]
//   - Forms: [FormGroup], [FormLabel], [FormCheck], [FormCheckLabel], [FormText], [InputGroup], [InputGroupText]
//   - Modals: [Modal], [ModalTrigger]
//   - Dropdowns: [Dropdown], [DropdownItem], [DropdownDivider]
//   - Offcanvas: [Offcanvas], [OffcanvasTrigger]
//   - Toasts: [Toast]
//   - Accordions: [Accordion], [AccordionItem]
//   - Other: [Breadcrumb], [Carousel], [Pagination], [Progress], [Spinner], [Collapse]
//
// # Combining with Attributes
//
// Use attribute utilities from the ba package:
//
//	bs.Container(
//	    ba.MT(4), ba.P(3),
//	    bs.Row(
//	        bs.ColMD(6, ba.MB(3), bs.Card{Body: text.Text("Card 1")}),
//	        bs.ColMD(6, ba.MB(3), bs.Card{Body: text.Text("Card 2")}),
//	    ),
//	)
//
// Version: Bootstrap 5.3
// Reference: https://getbootstrap.com/docs/5.3/
//
// Note: This package generates HTML markup only. You must include the Bootstrap
// CSS and JavaScript files in your HTML document for proper styling and behavior.
package bs
