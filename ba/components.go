package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// Tooltip creates the data-bs-toggle="tooltip" attribute of a Bootstrap
// tooltip trigger, followed by opts (e.g. attr.Title for the text and
// attr.Data("bs-placement", "top")).
//
// The attribute does nothing by itself. A tooltip needs both the Bootstrap
// JavaScript bundle, which the theme package only includes when it is opted
// in (theme.Options.BootstrapJS), and application JavaScript that creates
// each tooltip (new bootstrap.Tooltip(element)), which webui does not
// provide. Under a strict content security policy that script must be a
// same-origin file, not an inline script.
//
// Without JavaScript, the title attribute alone (attr.Title) gives a native
// tooltip, and the HTML popover attribute (attr.Popover with a popovertarget
// button) shows richer content on click.
func Tooltip(opts ...attr.Node) attr.Node {
	return attr.Cons(attr.Name("data-bs-toggle").Raw("tooltip"), opts...)
}

// Popover creates the data-bs-toggle="popover" attribute of a Bootstrap
// popover trigger, followed by opts (e.g. attr.Title and
// attr.Data("bs-content", "...")).
//
// Like [Tooltip], it needs the opt-in Bootstrap JavaScript bundle and
// application JavaScript that creates each popover
// (new bootstrap.Popover(element)), which webui does not provide; otherwise
// it is inert. It is unrelated to the HTML popover attribute (attr.Popover),
// which needs no JavaScript: a button with popovertarget toggles an element
// with popover="auto", and is the recommended alternative.
func Popover(opts ...attr.Node) attr.Node {
	return attr.Cons(attr.Name("data-bs-toggle").Raw("popover"), opts...)
}
