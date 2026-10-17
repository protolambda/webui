package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// Indicator sets the hx-indicator attribute to specify loading indicator element.
func Indicator(v string) attr.Node { return attr.Name("hx-indicator").Value(v) }
