package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// Trigger sets the hx-trigger attribute to specify what triggers the request.
func Trigger(v string) attr.Node { return attr.Name("hx-trigger").Value(v) }
