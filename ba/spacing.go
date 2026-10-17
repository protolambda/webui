package ba

import (
	"fmt"

	"github.com/protolambda/chord/core/attr"
)

// M creates a Bootstrap m-{n} (margin) class.
func M(n int) attr.Node { return rawClass(fmt.Sprintf("m-%d", n)) }

// MT creates a Bootstrap mt-{n} (margin-top) class.
func MT(n int) attr.Node { return rawClass(fmt.Sprintf("mt-%d", n)) }

// MB creates a Bootstrap mb-{n} (margin-bottom) class.
func MB(n int) attr.Node { return rawClass(fmt.Sprintf("mb-%d", n)) }

// MS creates a Bootstrap ms-{n} (margin-start) class.
func MS(n int) attr.Node { return rawClass(fmt.Sprintf("ms-%d", n)) }

// ME creates a Bootstrap me-{n} (margin-end) class.
func ME(n int) attr.Node { return rawClass(fmt.Sprintf("me-%d", n)) }

// MX creates a Bootstrap mx-{n} (margin x-axis) class.
func MX(n int) attr.Node { return rawClass(fmt.Sprintf("mx-%d", n)) }

// MY creates a Bootstrap my-{n} (margin y-axis) class.
func MY(n int) attr.Node { return rawClass(fmt.Sprintf("my-%d", n)) }

// P creates a Bootstrap p-{n} (padding) class.
func P(n int) attr.Node { return rawClass(fmt.Sprintf("p-%d", n)) }

// PT creates a Bootstrap pt-{n} (padding-top) class.
func PT(n int) attr.Node { return rawClass(fmt.Sprintf("pt-%d", n)) }

// PB creates a Bootstrap pb-{n} (padding-bottom) class.
func PB(n int) attr.Node { return rawClass(fmt.Sprintf("pb-%d", n)) }

// PS creates a Bootstrap ps-{n} (padding-start) class.
func PS(n int) attr.Node { return rawClass(fmt.Sprintf("ps-%d", n)) }

// PE creates a Bootstrap pe-{n} (padding-end) class.
func PE(n int) attr.Node { return rawClass(fmt.Sprintf("pe-%d", n)) }

// PX creates a Bootstrap px-{n} (padding x-axis) class.
func PX(n int) attr.Node { return rawClass(fmt.Sprintf("px-%d", n)) }

// PY creates a Bootstrap py-{n} (padding y-axis) class.
func PY(n int) attr.Node { return rawClass(fmt.Sprintf("py-%d", n)) }
