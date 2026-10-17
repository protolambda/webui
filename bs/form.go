package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"
)

// FormGroup creates a Bootstrap form group (mb-3 div wrapper).
func FormGroup(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("mb-3"), attrs...))
}

// FormLabel creates a Bootstrap form label.
func FormLabel(attrs ...attr.Node) elem.Scope {
	return label.Label(attr.Cons(rawClass("form-label"), attrs...))
}

// FormCheck creates a Bootstrap form check wrapper.
func FormCheck(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("form-check"), attrs...))
}

// FormCheckLabel creates a Bootstrap form check label.
func FormCheckLabel(attrs ...attr.Node) elem.Scope {
	return label.Label(attr.Cons(rawClass("form-check-label"), attrs...))
}

// FormText creates a Bootstrap form text helper.
func FormText(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("form-text"), attrs...))
}

// InputGroup creates a Bootstrap input group.
func InputGroup(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("input-group"), attrs...))
}

// InputGroupText creates a Bootstrap input group text addon.
func InputGroupText(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("input-group-text"), attrs...))
}
