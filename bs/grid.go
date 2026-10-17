package bs

import (
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
)

// Container creates a Bootstrap container div element.
func Container(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container"), attrs...))
}

// ContainerFluid creates a Bootstrap container-fluid div element.
func ContainerFluid(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container-fluid"), attrs...))
}

// ContainerSM creates a Bootstrap container-sm div element.
func ContainerSM(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container-sm"), attrs...))
}

// ContainerMD creates a Bootstrap container-md div element.
func ContainerMD(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container-md"), attrs...))
}

// ContainerLG creates a Bootstrap container-lg div element.
func ContainerLG(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container-lg"), attrs...))
}

// ContainerXL creates a Bootstrap container-xl div element.
func ContainerXL(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container-xl"), attrs...))
}

// ContainerXXL creates a Bootstrap container-xxl div element.
func ContainerXXL(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("container-xxl"), attrs...))
}

// Row creates a Bootstrap row div element.
func Row(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("row"), attrs...))
}

// Col creates a Bootstrap col div element.
func Col(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col"), attrs...))
}

// Col1 creates a Bootstrap col-1 div element.
func Col1(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-1"), attrs...))
}

// Col2 creates a Bootstrap col-2 div element.
func Col2(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-2"), attrs...))
}

// Col3 creates a Bootstrap col-3 div element.
func Col3(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-3"), attrs...))
}

// Col4 creates a Bootstrap col-4 div element.
func Col4(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-4"), attrs...))
}

// Col5 creates a Bootstrap col-5 div element.
func Col5(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-5"), attrs...))
}

// Col6 creates a Bootstrap col-6 div element.
func Col6(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-6"), attrs...))
}

// Col7 creates a Bootstrap col-7 div element.
func Col7(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-7"), attrs...))
}

// Col8 creates a Bootstrap col-8 div element.
func Col8(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-8"), attrs...))
}

// Col9 creates a Bootstrap col-9 div element.
func Col9(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-9"), attrs...))
}

// Col10 creates a Bootstrap col-10 div element.
func Col10(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-10"), attrs...))
}

// Col11 creates a Bootstrap col-11 div element.
func Col11(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-11"), attrs...))
}

// Col12 creates a Bootstrap col-12 div element.
func Col12(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-12"), attrs...))
}

// ColSM creates a Bootstrap col-sm-{n} div element.
func ColSM(n int, attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass(fmt.Sprintf("col-sm-%d", n)), attrs...))
}

// ColMD creates a Bootstrap col-md-{n} div element.
func ColMD(n int, attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass(fmt.Sprintf("col-md-%d", n)), attrs...))
}

// ColLG creates a Bootstrap col-lg-{n} div element.
func ColLG(n int, attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass(fmt.Sprintf("col-lg-%d", n)), attrs...))
}

// ColXL creates a Bootstrap col-xl-{n} div element.
func ColXL(n int, attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass(fmt.Sprintf("col-xl-%d", n)), attrs...))
}

// ColXXL creates a Bootstrap col-xxl-{n} div element.
func ColXXL(n int, attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass(fmt.Sprintf("col-xxl-%d", n)), attrs...))
}

// ColAuto creates a Bootstrap col-auto div element.
func ColAuto(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("col-auto"), attrs...))
}
