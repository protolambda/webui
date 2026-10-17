package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
)

func rawClass(value string) attr.Node {
	return attr.Name("class").Raw(value)
}

// closeButton is the btn-close button that dismisses the enclosing
// component. The button has no text, so aria-label names it.
func closeButton(dismiss string) elem.Node {
	return button.Button(
		rawClass("btn-close"),
		button.Type(button.TypeButton),
		attr.Name("data-bs-dismiss").Raw(dismiss),
		attr.Name("aria-label").Raw("Close"),
	)()
}

// optional returns an optional component field as a child list: empty when
// the field is nil.
func optional(n elem.Node) []elem.Node {
	if n == nil {
		return nil
	}
	return []elem.Node{n}
}
