package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Target sets the hx-target attribute to specify where to swap response content.
func Target(v string) attr.Node { return attr.Name("hx-target").Value(v) }

// Swap sets the hx-swap attribute to control how response content is swapped.
func Swap(v string) attr.Node { return attr.Name("hx-swap").Value(v) }

// SwapOOB sets the hx-swap-oob attribute for out-of-band swaps.
func SwapOOB(v string) attr.Node { return attr.Name("hx-swap-oob").Value(v) }

// Select sets the hx-select attribute to select a subset of response content.
func Select(v string) attr.Node { return attr.Name("hx-select").Value(v) }

// SelectOOB sets the hx-select-oob attribute for out-of-band selection.
func SelectOOB(v string) attr.Node { return attr.Name("hx-select-oob").Value(v) }
