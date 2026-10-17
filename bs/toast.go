package bs

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
)

// Toast represents a Bootstrap toast, a short notification.
//
// Bootstrap hides a toast (.toast:not(.show)) until its script shows it.
// Set Shown to render a visible toast from the server, without script: for
// example appended by an htmx out-of-band swap (beforeend) into a
// [ToastContainer] that is already in the page. Screen readers announce
// content added to a live region that already exists; a toast inserted
// together with a new live region around it is generally not announced.
//
// It renders div.toast[.show] with role="alert" aria-live="assertive"
// aria-atomic="true" (or role="status" aria-live="polite" when Polite),
// holding .toast-header (the Header and a close button, only when Header is
// set) and .toast-body. Attrs are added to the div.toast and may replace
// the role, aria-live and aria-atomic defaults.
type Toast struct {
	Header elem.Node // .toast-header content; nil leaves out the header and its close button
	Body   elem.Node // .toast-body content; nil leaves out the body
	Shown  bool      // add the show class: visible without Bootstrap's script
	Polite bool      // announce politely (role="status", aria-live="polite"), for a toast that does not need immediate attention
	// Close replaces the header's close button. Nil renders Bootstrap's
	// close button, whose data-bs-dismiss="toast" needs Bootstrap's script
	// (the theme package's opt-in Bootstrap JS asset). Without that script,
	// pass a [CloseButton] with an htmx request that removes the toast, or
	// elem.Noop() for no close button.
	Close elem.Node
	Attrs attr.Node // additional attributes of the div.toast
}

// Eval builds the toast structure with Bootstrap markup.
func (t Toast) Eval(ctx context.Context) (elem.Obj, error) {
	class := "toast"
	if t.Shown {
		class += " show"
	}
	role, live := "alert", "assertive"
	if t.Polite {
		role, live = "status", "polite"
	}
	own := []attr.Node{
		rawClass(class),
		fallbackRaw("role", role),
		fallbackRaw("aria-live", live),
		fallbackRaw("aria-atomic", "true"),
	}

	var children []elem.Node
	if t.Header != nil {
		closeBtn := t.Close
		if closeBtn == nil {
			closeBtn = dismissButton("toast", "")
		}
		children = append(children, div.Div(rawClass("toast-header"))(t.Header, closeBtn))
	}
	if t.Body != nil {
		children = append(children, div.Div(rawClass("toast-body"))(t.Body))
	}

	return div.Div(compose(userAttrs(t.Attrs), own...))(children...).Eval(ctx)
}

// ToastContainer creates a Bootstrap toast container (div.toast-container),
// which stacks the toasts in it. It is a polite live region
// (aria-live="polite", which attrs may replace), so that screen readers
// announce the toasts added to it. Position it among attrs, e.g. with the
// classes "position-fixed bottom-0 end-0 p-3".
//
// To show toasts from htmx responses, render the container with the page,
// empty, with an id. Then add each toast with an out-of-band swap that
// inserts into it: a ToastContainer with the same id and
// hx-swap-oob="beforeend" around the toast. Do not replace the container
// itself (an outerHTML or hx-swap-oob="true" swap): a live region that is
// inserted together with its content is generally not announced.
func ToastContainer(attrs ...attr.Node) elem.Scope {
	return div.Div(compose(attrs, rawClass("toast-container"), fallbackRaw("aria-live", "polite")))
}
