// Package page renders the HTML document shell and the standard layout of a
// webui application: the document head with its assets and htmx
// configuration, the page regions (navbar, optional sidebar, main content,
// footer), and the typed components that sit in them (navbar, page header,
// sidebar navigation, alerts).
//
// The components are chord nodes: struct values whose Eval method builds
// the markup when the page renders, and a few functions for alerts and
// regions. The package writes no styles and no scripts of its own. Markup
// uses Bootstrap 5.3 classes only, and works without Bootstrap's script:
// the navbar wraps instead of collapsing behind a toggler, and the sidebar
// layout is Bootstrap's grid, which stacks the columns on narrow screens.
//
// # Document
//
// [Document] renders the whole page: the doctype, the html element with its
// language and Bootstrap color mode, and a head in a fixed order:
//
//	<meta charset="utf-8">
//	<meta name="viewport" ...>
//	<title>...</title>
//	<meta name="color-scheme" ...>         matches the color mode
//	<link rel="icon" href="data:,">        no /favicon.ico request by default
//	<meta name="htmx-config" ...>          Document.HTMX; the zero value is secure
//	stylesheets of every asset set, in order
//	scripts of every asset set, deferred, in order
//	extra head nodes
//
// Assets are [asset.Set] values (usually the theme package's set, then the
// application's own), so every stylesheet and script is same-origin and
// carries an integrity value. Nothing is inlined, which keeps the page
// compatible with a strict Content-Security-Policy.
//
// # Layout and regions
//
// [Layout] is the standard body. Its regions have fixed ids, exported as
// constants, which fragments and handlers may target:
//
//   - [MainID] ("main"): the main landmark; the skip link points at it.
//   - [AlertsID] ("alerts"): a polite live region at the top of the main
//     content. Alerts placed here at render time show with the page; the web
//     package retargets htmx error responses into it, so that a failed
//     request shows its message without client-side script.
//   - [DialogsID] ("dialogs"): the dialogs of the page, after all other
//     content and outside every form (forms do not nest, and a dialog often
//     holds a form of its own).
//   - [HeaderID] ("page-header"): the page [Header], holding the page's one
//     h1.
//   - [NavbarID] and [SidebarID]: the default ids of the [Navbar] and the
//     [Sidebar], for out-of-band updates of their badges.
//
// # Alerts and announcements
//
// The alerts region is a polite live region (aria-live="polite") that
// stays in the page: screen readers announce what is added to it, once.
// [ErrorAlert] (role="alert") and [Notice] (role="status") are live regions
// of their own, which announce themselves. To have exactly one of the two
// announce each message, as with the toasts of a bs.ToastContainer:
//
//   - Alerts that come with the page ([Layout.Alerts], [AlertsRegion])
//     keep their roles. The region does not announce the content it loads
//     with; an error alert may be announced as the page loads.
//   - Alerts that htmx swaps into the region while the page is open go
//     through [AlertsOOB] (an out-of-band swap) or [AlertsContent] (an
//     answer that targets the region), whose [ErrorAlert] and [Notice]
//     values render without a role. The region announces them, politely,
//     errors too.
//   - Outside the region, such as a form's error summary that a swap
//     replaces with the form, an alert announces itself through its role.
//
// Swap content into the region (innerHTML), never the region itself: a
// live region that is inserted together with its content is generally not
// announced. [ShowAlerts] scrolls the region into view after an htmx swap
// that leaves content in the page; [ShowWindowTop] scrolls to the top of
// the page after one that removes its target, such as a deleted list item.
//
// # Dialogs and focus
//
// Dialogs (bs.Dialog) open and close without script, through the buttons
// that name them. An htmx answer does not close an open dialog: the answer
// to a form in the dialog swaps other parts of the page and leaves the
// dialog open, modal over the new content. Re-render the dialogs region
// out of band ([DialogsOOB]) to close it, and give the main content of the
// answer a [FocusTarget], since the focus is lost with the removed dialog:
//
//	elem.Bundle{
//		taskList(tasks), // the list, its heading with page.FocusTarget()
//		page.DialogsOOB(dialogs...), // the dialogs still needed
//		page.AlertsOOB(page.Notice("", text.Text("Task deleted."))),
//	}
//
// The example of [DialogsOOB] shows the whole exchange.
//
// # Accessibility
//
// Components require accessible names and fail the render with [ErrNoName]
// without one: a document without a title, a navigation item without a
// label. Navigation landmarks carry an aria-label that can be set per
// component (also the breadcrumb's, [Header.CrumbsLabel]), for pages in
// other languages. The link of the current page is marked with
// aria-current="page" as well as the active style ([NavItem.Current]); the
// link of the section that holds it, such as a navbar area whose sidebar
// lists the current page, with aria-current="true" and the same style
// ([NavItem.CurrentSection]). Decorative icons are hidden from assistive
// technology.
//
// # Errors
//
// Invalid options (an unknown color mode or variant, a malformed language
// tag, an id that is not a token, a URL that is not allowed in its place)
// fail the render with [ErrInvalid]. Error messages name the component and
// the option, never its value.
//
// [ErrInvalid] and [ErrNoName] are the bs package's sentinels
// (bs.ErrInvalidOption and bs.ErrNoName), so one errors.Is check covers the
// page components and the bs components rendered in a [Document] or
// [Layout], such as a bs.Dialog without a title. An invalid id, of a page
// or a bs component, matches both [ErrInvalid] and bs.ErrInvalidID:
//
//	if errors.Is(err, page.ErrInvalid) {
//		// an option or an id that the code got wrong
//	}
package page
