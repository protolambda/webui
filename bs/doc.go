// Package bs provides Bootstrap 5.3 components as chord element nodes.
//
// This package contains Bootstrap components that create HTML elements.
// For Bootstrap attribute utilities (spacing, colors, flexbox, etc.), use
// the [github.com/protolambda/webui/ba] package.
//
// The markup needs Bootstrap's CSS, and the components marked "no script"
// below also need the webui theme CSS (package theme), which styles native
// dialog and details elements as Bootstrap modals, accordions and
// dropdowns. Nothing in this package renders inline scripts, event handler
// attributes or style attributes, so the output works under a strict
// Content-Security-Policy.
//
// # Two call shapes
//
// Function components take attributes and return an [elem.Scope], which is
// then called with the children:
//
//	bs.Container()(
//		bs.Row()(
//			bs.Col6()(text.Text("Left")),
//			bs.Col6()(text.Text("Right")),
//		),
//	)
//
// Struct components have named fields, and an Attrs field for additional
// attributes of their outer element:
//
//	bs.Card{Header: text.Text("Notes"), Body: text.Text("No notes yet.")}
//
// Attributes combine with the ba utilities; classes merge:
//
//	bs.Container(ba.MT(4))(
//		bs.Row()(
//			bs.ColMD(6, ba.MB(3))(bs.Card{Body: text.Text("Card 1")}),
//			bs.ColMD(6, ba.MB(3))(bs.Card{Body: text.Text("Card 2")}),
//		),
//	)
//
// # Defaults and attributes
//
// Components add the attributes that Bootstrap's documentation asks for:
// type="button" on buttons (so that they do not submit a form by
// accident), role="alert" on alerts, role="status" and a visually hidden
// label on spinners, aria-current on active links, aria-labelledby on
// dialogs, and so on. A caller's attribute of the same name replaces such
// a default instead of failing the render as a duplicate, wherever it is
// among the attributes (bundles, sequences and lazy attr.Fn nodes
// included; these are evaluated once, during the render). Attributes that
// a component needs, such as the id of a struct component, are not
// defaults: setting them again fails the render with chord's duplicate
// attribute error. Classes always merge.
//
// # Validation
//
// Options are typed ([Variant], [ModalSize], [OffcanvasPlacement],
// [HeadingLevel], [InputMode]) and checked when the component renders: an
// unknown value fails the render with [ErrInvalidOption] instead of
// emitting a class that does not exist. Ids that components use in CSS
// selectors, id references or derived ids must be tokens (see [ValidID]),
// or the render fails with [ErrInvalidID], which also matches
// [ErrInvalidOption]. Error messages name the component and the problem,
// never the offending value.
//
// # Text from users
//
// chord escapes all text, so caller text never becomes markup, but text
// can still mislead a reader: a direction override reorders the text after
// it, and invisible characters make two names look the same. Render names
// and titles that other users chose with
// [github.com/protolambda/webui/visible.Text], and identifiers with
// [github.com/protolambda/webui/visible.Code]: they escape such characters
// and mark the result. Where only text fits, such as the Label of a
// [SelectOption] or an aria-label, use
// [github.com/protolambda/webui/visible.TextString] and
// [github.com/protolambda/webui/visible.CodeString], which keep the marker
// as a word. Look-alike letters, such as a Cyrillic letter in a
// Latin name, are visible letters that neither escapes; telling such names
// apart is an application policy (see the visible package).
//
// [Dialog], the form fields ([Field], [Textarea], [Select], [Checkbox],
// [Radio]) and [DetailsDropdown] fail with [ErrNoName] when they would have
// no accessible name: no Title, Label or Toggle, and no aria-label or
// aria-labelledby in their attributes. [Modal], [Offcanvas] and [Dropdown]
// accept a nil Title or Toggle, as in earlier versions, and do not check:
// their callers must then give the name themselves, with an aria-label in
// Attrs (ToggleAttrs for a Dropdown).
//
// # Forms
//
// The field components ([Field], [Textarea], [Select], [Checkbox] and
// [Radio]) render a label, the control, help text and a server-side
// validation error, tied together by ids (for, aria-describedby). To
// report errors, the server renders the form again with Error set and the
// submitted values (never a secret), for example as the answer to an htmx
// request that replaces the form in place.
//
// [Field], [Textarea] and [Select] sit in a div.mb-3, a block with a bottom
// margin, as fields stacked in a form do. In a row of controls, such as a
// filter bar or the cells of a table, give them another class with
// WrapperClass, such as "col-auto" or "mb-0" (see the row example of
// [Field]): a margin class in Wrapper cannot undo mb-3.
//
// Such a swap removes the element that has the keyboard focus, often the
// submit button. htmx then focuses the element of the new content that
// has the same id, if there is one; otherwise the focus falls back to the
// document body, and a keyboard or screen reader user starts over at the
// top of the page. The fields have ids already. Give the submit button, and
// any other control that sends the form, an id as well, the same in every
// rendering of the form and unique in the page:
//
//	bs.BtnPrimary(button.Type(button.TypeSubmit), attr.ID("note-save"))(text.Text("Save"))
//
// The form itself stays a plain HTML form: method="post" and an action
// beside hx-post to the same URL, and the CSRF field
// ([github.com/protolambda/webui/session.CSRFInput]). Without htmx (script
// blocked or not loaded yet) it posts like any form, and the server answers
// with the whole page; with htmx, only the form is swapped. A form with
// hx-post and no method falls back to a GET of the page's URL, with the
// fields, the CSRF token too, in the query string. The htmx form example of
// [BtnPrimary] shows the whole form.
//
// # Components
//
//   - Grid: [Container], [Row], [Col], [Col1] to [Col12], [ColSM], [ColMD],
//     [ColLG], [ColXL], [ColXXL], [ColAuto]
//   - Buttons: [Btn], [BtnOf], [BtnOutlineOf], [BtnPrimary] and the other
//     variants, [BtnGroup], [BtnToolbar], [CloseButton]
//   - Badges and alerts: [Badge], [BadgeOf], [Alert], [AlertOf], the
//     variant constructors, [AlertDismissible]
//   - Cards and lists: [Card], [CardBody], [CardImgOverlay], [ListGroup],
//     [ListGroupItem], [ListGroupItemActive]
//   - Navigation: [Nav], [NavTabs], [NavPills], [NavUnderline], [NavItem],
//     [NavLink], [NavLinkActive], [Navbar], [NavbarBrand], [NavbarNav],
//     [Breadcrumb], [BreadcrumbActive], [Pagination], [PageItemActive],
//     [PageLinkCurrent]
//   - Tables: [DataTable], [Table], [TableStriped], [TableResponsive]
//   - Forms: [Field] (with input groups), [Textarea], [Select], [Checkbox]
//     (and switches), [Radio], and the class wrappers [FormGroup],
//     [FormLabel], [FormCheck], [FormText], [InvalidFeedback], [InputGroup]
//   - Feedback: [Progress], [ProgressValue], [Spinner], [Toast] (with
//     Shown), [ToastContainer]
//   - No script: [Dialog] with [DialogOpen], [DialogClose] and
//     [DialogButton]; [DetailsAccordion]; [DetailsDropdown]
//
// # Components that need Bootstrap's script
//
// [Modal], [ModalTrigger], [Offcanvas], [OffcanvasTrigger], [Dropdown],
// [Accordion], [Collapse], [Carousel], [NavbarToggler],
// [NavbarTogglerFor], [NavbarCollapse], the close button of
// [AlertDismissible] and the default close button of [Toast] work only
// with Bootstrap's JavaScript, which the theme package ships as an opt-in
// asset (its BootstrapJS option). Without it they render but do nothing:
// a modal never opens, a dropdown never shows its menu. Prefer the no-JS
// components ([Dialog], [DetailsAccordion], [DetailsDropdown], a [Toast]
// with Shown, a navbar with navbar-expand) unless the application loads
// that script.
//
// Version: Bootstrap 5.3. Reference: https://getbootstrap.com/docs/5.3/
package bs
