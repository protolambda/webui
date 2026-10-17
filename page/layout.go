package page

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/hx2"
)

// DefaultSkipLabel is the text of the skip link of a [Layout] without one.
const DefaultSkipLabel = "Skip to main content"

// Layout is the standard body of a page: a skip link, the navbar, an
// optional sidebar, the main landmark with the page header, the alerts
// region and the content, a footer, and the dialogs region.
//
// Without a sidebar it renders:
//
//	<div class="d-flex flex-column min-vh-100">
//	  <a class="visually-hidden-focusable p-2" href="#main">Skip to main content</a>
//	  Navbar
//	  <main id="main" class="flex-grow-1">
//	    <div class="container py-3">              container-fluid when Fluid
//	      Header
//	      <div id="alerts" aria-live="polite">Alerts</div>
//	      Main
//	    </div>
//	  </main>
//	  <footer class="border-top py-3"><div class="container">Footer</div></footer>
//	</div>
//	<div id="dialogs">Dialogs</div>
//
// With a sidebar, the navbar is followed by a two-column grid that stacks
// on screens narrower than Bootstrap's md breakpoint (sidebar first):
//
//	<div class="container-fluid flex-grow-1 d-flex flex-column">
//	  <div class="row flex-grow-1 align-content-start align-content-md-stretch">
//	    <div class="col-md-3 col-xl-2 py-3 position-relative">
//	      Sidebar
//	      <div class="vr d-none d-md-block position-absolute top-0 end-0 h-100"></div>
//	    </div>
//	    <main id="main" class="col-md-9 col-xl-10 py-3">Header, alerts, Main</main>
//	  </div>
//	</div>
//
// The layout needs no script. The column wrapper keeps the footer at the
// bottom of short pages. From md up, the columns fill the height down to
// the footer and a vertical rule (an empty, decorative .vr) separates them
// over that full height; stacked, the columns keep their content height
// and draw no rule.
//
// The alerts region ([AlertsID]) and the dialogs region ([DialogsID]) are
// always rendered, also when empty, because handlers and fragments target
// them: the web package swaps htmx error messages into the alerts region,
// and a fragment may append a dialog to the dialogs region. Dialogs are
// rendered after all other content and outside every form: forms do not
// nest, and a dialog often holds a form of its own. Render one Layout per
// page; its ids are fixed.
//
// The Header comes before the alerts, so that the page's h1 is the first
// thing in the main landmark and the alerts follow it.
type Layout struct {
	// Navbar is the page's top navigation, usually a [Navbar]. Nil leaves
	// it out.
	Navbar elem.Node
	// Sidebar is the content of the sidebar column, usually a [Sidebar].
	// Nil means a single-column layout.
	Sidebar elem.Node
	// Header is the page header, usually a [Header]. Nil leaves it out.
	Header elem.Node
	// Alerts are the alerts shown with the page, such as the notice of a
	// completed action, inside the alerts region.
	Alerts []elem.Node
	// Main is the page content.
	Main elem.Node
	// Footer is the content of the page footer; the footer element and its
	// container are provided. Nil leaves out the footer.
	Footer elem.Node
	// Dialogs are the dialogs of the page (e.g. bs.Dialog), in the dialogs
	// region.
	Dialogs []elem.Node
	// Fluid makes the containers full width (container-fluid) instead of
	// Bootstrap's responsive fixed widths. With a Sidebar the layout is
	// always full width.
	Fluid bool
	// SkipLabel is the text of the skip link. Empty means
	// [DefaultSkipLabel].
	SkipLabel string
	// NoSkipLink leaves out the skip link, for pages without navigation
	// before the main content.
	NoSkipLink bool
}

var _ elem.Node = Layout{}

// Eval builds the layout. It fails only when one of the given nodes fails.
func (l Layout) Eval(ctx context.Context) (elem.Obj, error) {
	container := "container"
	if l.Fluid || l.Sidebar != nil {
		container = "container-fluid"
	}
	alerts := AlertsRegion(l.Alerts...)
	content := []elem.Node{optional(l.Header), alerts, optional(l.Main)}

	column := make([]elem.Node, 0, 4)
	if !l.NoSkipLink {
		label := l.SkipLabel
		if label == "" {
			label = DefaultSkipLabel
		}
		column = append(column, text.A(attr.Class("visually-hidden-focusable p-2"), text.Href("#"+MainID))(text.Text(label)))
	}
	column = append(column, optional(l.Navbar))
	if l.Sidebar == nil {
		column = append(column, section.Main(attr.ID(MainID), attr.Class("flex-grow-1"))(
			div.Div(attr.Class(container+" py-3"))(content...),
		))
	} else {
		// The row fills the wrapper down to the footer. Stacked (below md)
		// its lines keep their content height; side by side the columns
		// stretch, so that the rule between them reaches the footer.
		// Bootstrap has no responsive border utilities, so the rule is a
		// .vr at the end of the sidebar column, shown from md up only:
		// stacked columns draw no line at the edge of the screen.
		column = append(column, div.Div(attr.Class("container-fluid flex-grow-1 d-flex flex-column"))(
			div.Div(attr.Class("row flex-grow-1 align-content-start align-content-md-stretch"))(
				div.Div(attr.Class("col-md-3 col-xl-2 py-3 position-relative"))(
					l.Sidebar,
					div.Div(attr.Class("vr d-none d-md-block position-absolute top-0 end-0 h-100"))(),
				),
				section.Main(attr.ID(MainID), attr.Class("col-md-9 col-xl-10 py-3"))(content...),
			),
		))
	}
	if l.Footer != nil {
		column = append(column, section.Footer(attr.Class("border-top py-3"))(
			div.Div(attr.Class(container))(l.Footer),
		))
	}
	return elem.Bundle{
		div.Div(attr.Class("d-flex flex-column min-vh-100"))(column...),
		DialogsRegion(l.Dialogs...),
	}.Eval(ctx)
}

// AlertsRegion renders the alerts region with the given alerts:
// <div id="alerts" aria-live="polite">. [Layout] renders it; AlertsRegion
// is for custom layouts, which should keep its id and live region.
//
// The alerts are rendered as they are: alerts that come with the page keep
// their roles (an [ErrorAlert] its role="alert", which screen readers may
// announce as the page loads, while a polite region does not announce the
// content it loads with). Alerts that htmx swaps into the region later go
// through [AlertsOOB] or [AlertsContent], and the region announces them.
func AlertsRegion(alerts ...elem.Node) elem.Node {
	return div.Div(attr.ID(AlertsID), aria.Live("polite"))(alerts...)
}

// DialogsRegion renders the dialogs region with the given dialogs:
// <div id="dialogs">. [Layout] renders it; DialogsRegion is for custom
// layouts, which should render it outside every form.
func DialogsRegion(dialogs ...elem.Node) elem.Node {
	return div.Div(attr.ID(DialogsID))(dialogs...)
}

// AlertsOOB renders alerts for an htmx response that swaps them into the
// alerts region out of band, next to its main content:
// <div id="alerts" hx-swap-oob="innerHTML">. Only the content is swapped,
// so the region itself stays in the page and screen readers announce the
// new alerts (a live region that is replaced as a whole is not announced).
// The alerts are rendered as [AlertsContent], so that the region is their
// only live region: an [ErrorAlert] or [Notice] in them has no role. No
// alerts empty the region.
//
// An out-of-band swap does not scroll: add [ShowAlerts] to the hx-swap of
// the element that makes the request, so that the notice is also seen.
func AlertsOOB(alerts ...elem.Node) elem.Node {
	return div.Div(attr.ID(AlertsID), hx2.SwapOOB(hx2.SwapInnerHTML))(AlertsContent(alerts...))
}

// ShowAlerts is the hx-swap modifier show:#alerts:top: once the swap is
// done, htmx scrolls the alerts region ([AlertsID]) to the top of the
// window. An out-of-band swap cannot scroll by itself (htmx 2 reads
// modifiers in hx-swap-oob as a selector, and drops the swap), so a
// request whose answer carries a notice out of band ([AlertsOOB]) scrolls
// with the modifier on the hx-swap of the element that makes it:
//
//	hx2.Swap(hx2.SwapOuterHTML + " " + page.ShowAlerts)
//
// or on an HX-Reswap header of the answer (hx2.SetReswap), which replaces
// the whole hx-swap of that element, its swap style included. Every
// answer to the request then scrolls, also one without a notice, and also
// when the region is in view already: use it on requests whose answers
// report their outcome in the alerts region. A page whose navbar stays in
// place at the top (sticky-top or fixed-top) should give the region a
// scroll-margin-top of the navbar's height in its stylesheet, or the
// navbar covers the notice. Error answers of the web package scroll the
// region by themselves.
const ShowAlerts = "show:#" + AlertsID + ":top"

// DialogsOOB renders dialogs for an htmx response that swaps them into the
// dialogs region out of band: <div id="dialogs" hx-swap-oob="innerHTML">.
// The dialogs in the region are replaced, which closes an open one, such
// as the dialog whose form made the request. Pass every dialog that the
// page still needs, since their openers name them by id; no dialogs empty
// the region.
//
// An htmx answer does not close a dialog by itself: an answer that swaps
// other parts of the page leaves the dialog open and modal over the new
// content. Besides re-rendering the region, an answer closes one dialog by
// replacing it out of band, rendered again with Attrs:
// hx2.SwapOOB("true") (a bs.Dialog never renders open), or by removing it
// with an element of the dialog's id and hx2.SwapOOB(hx2.SwapDelete). A
// redirect (web.Redirect) closes it with the full page load.
//
// A dialog that the browser closes gives the focus back to its opener, but
// one that is replaced or removed does not, and htmx's restoring of the
// focus by id finds the replaced dialog's controls hidden: the focus falls
// to the body, and a keyboard user starts over at the top of the page.
// Give the main content of the answer an element with [FocusTarget], such
// as the heading of the updated list, which htmx focuses after the swap.
func DialogsOOB(dialogs ...elem.Node) elem.Node {
	return div.Div(attr.ID(DialogsID), hx2.SwapOOB(hx2.SwapInnerHTML))(dialogs...)
}

// FocusTarget marks the element that takes the focus when an htmx swap
// inserts it: tabindex="-1" autofocus. htmx focuses the first element
// with autofocus in each element that it swaps in, once the swap settles;
// tabindex="-1" lets an element that is not a control, such as a heading,
// take the focus, without adding it to the order of the Tab key. A screen
// reader then reads the element, and the next Tab press continues from it.
// htmx scrolls after it focuses, so a show modifier, such as
// [ShowAlerts], decides where the window ends.
//
// Use it in htmx answers that remove the element that had the focus, such
// as the button of a dialog that the answer closes (see [DialogsOOB]) or
// of a list item that it deletes, and that have no control with the same
// id to give the focus to. Render it in the answer only: in a full page,
// the browser focuses an autofocus element as the page loads, and the
// skip link and the navbar are passed over. Mark one element per answer,
// and not one inside a dialog, whose closed controls cannot take the
// focus.
func FocusTarget() attr.Node {
	return attr.Bundle{attr.Tabindex("-1"), attr.Autofocus()}
}

// optional returns n, or a no-op for nil.
func optional(n elem.Node) elem.Node {
	if n == nil {
		return elem.Noop()
	}
	return n
}
