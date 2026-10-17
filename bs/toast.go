package bs

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
)

// Toast represents a Bootstrap toast component.
type Toast struct {
	Header elem.Node // toast-header content
	Body   elem.Node // toast-body content
	Attrs  attr.Node
}

// Eval builds the toast structure with proper Bootstrap markup.
func (t Toast) Eval(ctx context.Context) (elem.Obj, error) {
	var attrs []attr.Node
	if t.Attrs != nil {
		attrs = append(attrs, t.Attrs)
	}
	attrs = append(attrs, rawClass("toast"))

	var children []elem.Node
	if t.Header != nil {
		header := div.Div(rawClass("toast-header"))(
			t.Header,
			closeButton("toast"),
		)
		children = append(children, header)
	}

	if t.Body != nil {
		children = append(children, div.Div(rawClass("toast-body"))(t.Body))
	}

	return div.Div(attrs...)(children...).Eval(ctx)
}
