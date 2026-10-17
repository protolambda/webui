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
// [Layout], such as a bs.Dialog without a title. An invalid id also matches
// bs.ErrInvalidID, as it does in bs.
package page
