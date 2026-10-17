package page

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/div"

	"github.com/protolambda/webui/bs"
)

// ErrorAlert renders an error message, such as a refused action:
// <div class="alert alert-danger" role="alert">. With its role, the alert
// is a live region of its own, which screen readers announce at once,
// interrupting: in the page that it comes with (in [Layout.Alerts] or a
// form's error summary), or when a swap inserts it outside a live region.
//
// Inside [AlertsContent] and [AlertsOOB], content for a live region that
// is already in the page, it renders no role: <div class="alert
// alert-danger">. The region announces it, once (see Alerts and
// announcements in the package documentation).
//
// Never put submitted values or secrets in it.
func ErrorAlert(children ...elem.Node) elem.Node {
	return alert{component: "page.ErrorAlert", variant: bs.VariantDanger, role: "alert", children: children}
}

// Notice renders the outcome of an action, such as "Saved.":
// <div class="alert alert-{v}" role="status">. A status is announced
// politely, without interrupting. An empty v means bs.VariantSuccess; an
// unknown variant fails the render with [ErrInvalid] (which is
// bs.ErrInvalidOption).
//
// Inside [AlertsContent] and [AlertsOOB] it renders no role, like
// [ErrorAlert]: the live region it is swapped into announces it.
func Notice(v bs.Variant, children ...elem.Node) elem.Node {
	if v == "" {
		v = bs.VariantSuccess
	}
	return alert{component: "page.Notice", variant: v, role: "status", children: children}
}

// AlertsContent renders alerts for an htmx answer that is swapped into a
// live region already in the page: the content of the alerts region
// ([AlertsID]) for an answer whose hx-target or HX-Retarget names it,
// with the innerHTML swap style, or of another live region, such as one
// at the top of a dialog. It adds no element of its own.
//
// The live region announces the alerts that are swapped into it, so the
// [ErrorAlert] and [Notice] values among alerts, also deeper in them,
// render no role: an alert with a role of its own inside the region would
// be a second live region, which a screen reader may read twice. Other
// alerts keep their attributes: the Alert* constructors of the bs package
// render role="alert" anywhere. [AlertsOOB] wraps its alerts in
// AlertsContent.
func AlertsContent(alerts ...elem.Node) elem.Node {
	return liveContent(alerts)
}

// liveContentKey is the context key that marks the subtree of
// [AlertsContent] during a render.
type liveContentKey struct{}

// inLiveContent reports whether ctx is that of the subtree of
// [AlertsContent].
func inLiveContent(ctx context.Context) bool {
	in, _ := ctx.Value(liveContentKey{}).(bool)
	return in
}

// liveContent renders its nodes with a context that marks their subtree,
// so that the alerts in it leave the announcing to the live region they
// are swapped into.
type liveContent []elem.Node

func (c liveContent) Eval(ctx context.Context) (elem.Obj, error) {
	return elem.Bundle(c).Eval(context.WithValue(ctx, liveContentKey{}, true))
}

// alert is an [ErrorAlert] or a [Notice]: a Bootstrap alert of the variant
// with the role, which it leaves out inside [AlertsContent].
type alert struct {
	component string
	variant   bs.Variant
	role      string
	children  []elem.Node
}

func (a alert) Eval(ctx context.Context) (elem.Obj, error) {
	if !a.variant.Valid() {
		return elem.Obj{}, fail(a.component, ErrInvalid, "variant")
	}
	if inLiveContent(ctx) {
		return div.Div(attr.Class("alert alert-" + string(a.variant)))(a.children...).Eval(ctx)
	}
	return bs.AlertOf(a.variant, aria.Role(a.role))(a.children...).Eval(ctx)
}
