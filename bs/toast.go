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
// [ToastContainer] that is already in the page.
//
// It renders div.toast[.show] holding .toast-header (the Header and a close
// button, only when Header is set) and .toast-body. Attrs are added to the
// div.toast.
//
// # Announcements
//
// One live region announces a toast, never two nested ones, which a
// screen reader may read twice:
//
//   - Inside a [ToastContainer], among its children or deeper, the toast
//     is not a live region of its own: it renders no role, aria-live or
//     aria-atomic, and Polite has no effect.
//     The container, a live region that is already in the page, announces
//     the toasts added to it. This is the way to show toasts from htmx
//     responses (see ToastContainer).
//   - Elsewhere, the toast is its own live region, with Bootstrap's
//     attributes: role="alert" aria-live="assertive" aria-atomic="true",
//     or role="status" aria-live="polite" aria-atomic="true" when Polite.
//     That suits a toast that is in the page before it shows, such as one
//     that Bootstrap's script shows. A toast that a swap inserts outside a
//     container is announced only as far as screen readers announce a live
//     region that arrives with its content: generally for role="alert",
//     often not for role="status".
//
// Attrs may set or replace these attributes either way.
type Toast struct {
	Header elem.Node // .toast-header content; nil leaves out the header and its close button
	Body   elem.Node // .toast-body content; nil leaves out the body
	Shown  bool      // add the show class: visible without Bootstrap's script
	// Polite announces the toast politely (role="status",
	// aria-live="polite"), for a toast that does not need immediate
	// attention. Inside a ToastContainer it has no effect: the container's
	// aria-live decides.
	Polite bool
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
	own := []attr.Node{rawClass(class)}
	if !inToastContainer(ctx) {
		role, live := "alert", "assertive"
		if t.Polite {
			role, live = "status", "polite"
		}
		own = append(own,
			fallbackRaw("role", role),
			fallbackRaw("aria-live", live),
			fallbackRaw("aria-atomic", "true"),
		)
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
// which stacks the toasts in it. It is the live region of its toasts
// (aria-live="polite", which attrs may replace, e.g. with
// aria-live="assertive" for a container of urgent toasts): screen readers
// announce each toast added to it, once. The [Toast] values rendered inside
// it are therefore no live regions of their own (see Toast). Position it
// among attrs, e.g. with the classes "position-fixed bottom-0 end-0 p-3".
//
// To show toasts from htmx responses, render the container with the page,
// empty, with an id. Then add each toast with an out-of-band swap that
// inserts into it: a ToastContainer with the same id and
// hx-swap-oob="beforeend" around the toast. htmx appends the toast to the
// container in the page, an existing live region, which announces it. Do
// not replace the container itself (an outerHTML or hx-swap-oob="true"
// swap): a live region that is inserted together with its content is
// generally not announced.
func ToastContainer(attrs ...attr.Node) elem.Scope {
	container := div.Div(compose(attrs, rawClass("toast-container"), fallbackRaw("aria-live", "polite")))
	return func(children ...elem.Node) elem.Node {
		return toastRegion{node: container(children...)}
	}
}

// toastContainerKey is the context key that marks the subtree of a
// [ToastContainer] during a render.
type toastContainerKey struct{}

// inToastContainer reports whether ctx is that of the subtree of a
// [ToastContainer].
func inToastContainer(ctx context.Context) bool {
	in, _ := ctx.Value(toastContainerKey{}).(bool)
	return in
}

// toastRegion renders the container node with a context that marks its
// subtree, so that the toasts in it leave the announcing to the container.
type toastRegion struct {
	node elem.Node
}

func (r toastRegion) Eval(ctx context.Context) (elem.Obj, error) {
	return r.node.Eval(context.WithValue(ctx, toastContainerKey{}, true))
}
