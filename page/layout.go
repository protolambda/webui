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
//	<div class="container-fluid flex-grow-1"><div class="row">
//	  <div class="col-md-3 col-xl-2 border-end py-3">Sidebar</div>
//	  <main id="main" class="col-md-9 col-xl-10 py-3">Header, alerts, Main</main>
//	</div></div>
//
// The layout needs no script. The column wrapper keeps the footer at the
// bottom of short pages.
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
		column = append(column, div.Div(attr.Class("container-fluid flex-grow-1"))(
			div.Div(attr.Class("row"))(
				div.Div(attr.Class("col-md-3 col-xl-2 border-end py-3"))(l.Sidebar),
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
// No alerts empty the region.
func AlertsOOB(alerts ...elem.Node) elem.Node {
	return div.Div(attr.ID(AlertsID), hx2.SwapOOB(hx2.SwapInnerHTML))(alerts...)
}

// optional returns n, or a no-op for nil.
func optional(n elem.Node) elem.Node {
	if n == nil {
		return elem.Noop()
	}
	return n
}
