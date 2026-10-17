package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// FormControl creates a Bootstrap form-control class attribute.
func FormControl(opts ...attr.Node) attr.Node {
	return attr.Cons(rawClass("form-control"), opts...)
}

// FormSelect creates a Bootstrap form-select class attribute.
func FormSelect(opts ...attr.Node) attr.Node {
	return attr.Cons(rawClass("form-select"), opts...)
}

// FormCheckInput creates a Bootstrap form-check-input class attribute.
func FormCheckInput(opts ...attr.Node) attr.Node {
	return attr.Cons(rawClass("form-check-input"), opts...)
}
