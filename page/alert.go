package page

import (
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"

	"github.com/protolambda/webui/bs"
)

// ErrorAlert renders an error message, such as a refused action:
// <div class="alert alert-danger" role="alert">. Screen readers announce it
// as soon as it appears, also when an htmx swap inserts it. Never put
// submitted values or secrets in it.
func ErrorAlert(children ...elem.Node) elem.Node {
	return bs.AlertDanger()(children...)
}

// Notice renders the outcome of an action, such as "Saved.":
// <div class="alert alert-{v}" role="status">. A status is announced
// politely, without interrupting. An empty v means bs.VariantSuccess; an
// unknown variant fails the render with bs.ErrInvalidOption.
func Notice(v bs.Variant, children ...elem.Node) elem.Node {
	if v == "" {
		v = bs.VariantSuccess
	}
	return bs.AlertOf(v, aria.Role("status"))(children...)
}
