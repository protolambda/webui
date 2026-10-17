package ba

import (
	"fmt"

	"github.com/protolambda/chord/core/attr"
)

// FlexRow creates a Bootstrap flex-row class.
func FlexRow() attr.Node { return rawClass("flex-row") }

// FlexColumn creates a Bootstrap flex-column class.
func FlexColumn() attr.Node { return rawClass("flex-column") }

// FlexRowReverse creates a Bootstrap flex-row-reverse class.
func FlexRowReverse() attr.Node { return rawClass("flex-row-reverse") }

// FlexColumnReverse creates a Bootstrap flex-column-reverse class.
func FlexColumnReverse() attr.Node { return rawClass("flex-column-reverse") }

// FlexWrap creates a Bootstrap flex-wrap class.
func FlexWrap() attr.Node { return rawClass("flex-wrap") }

// FlexNowrap creates a Bootstrap flex-nowrap class.
func FlexNowrap() attr.Node { return rawClass("flex-nowrap") }

// FlexWrapReverse creates a Bootstrap flex-wrap-reverse class.
func FlexWrapReverse() attr.Node { return rawClass("flex-wrap-reverse") }

// JustifyContentStart creates a Bootstrap justify-content-start class.
func JustifyContentStart() attr.Node { return rawClass("justify-content-start") }

// JustifyContentEnd creates a Bootstrap justify-content-end class.
func JustifyContentEnd() attr.Node { return rawClass("justify-content-end") }

// JustifyContentCenter creates a Bootstrap justify-content-center class.
func JustifyContentCenter() attr.Node { return rawClass("justify-content-center") }

// JustifyContentBetween creates a Bootstrap justify-content-between class.
func JustifyContentBetween() attr.Node { return rawClass("justify-content-between") }

// JustifyContentAround creates a Bootstrap justify-content-around class.
func JustifyContentAround() attr.Node { return rawClass("justify-content-around") }

// JustifyContentEvenly creates a Bootstrap justify-content-evenly class.
func JustifyContentEvenly() attr.Node { return rawClass("justify-content-evenly") }

// AlignItemsStart creates a Bootstrap align-items-start class.
func AlignItemsStart() attr.Node { return rawClass("align-items-start") }

// AlignItemsEnd creates a Bootstrap align-items-end class.
func AlignItemsEnd() attr.Node { return rawClass("align-items-end") }

// AlignItemsCenter creates a Bootstrap align-items-center class.
func AlignItemsCenter() attr.Node { return rawClass("align-items-center") }

// AlignItemsBaseline creates a Bootstrap align-items-baseline class.
func AlignItemsBaseline() attr.Node { return rawClass("align-items-baseline") }

// AlignItemsStretch creates a Bootstrap align-items-stretch class.
func AlignItemsStretch() attr.Node { return rawClass("align-items-stretch") }

// AlignSelfStart creates a Bootstrap align-self-start class.
func AlignSelfStart() attr.Node { return rawClass("align-self-start") }

// AlignSelfEnd creates a Bootstrap align-self-end class.
func AlignSelfEnd() attr.Node { return rawClass("align-self-end") }

// AlignSelfCenter creates a Bootstrap align-self-center class.
func AlignSelfCenter() attr.Node { return rawClass("align-self-center") }

// AlignSelfBaseline creates a Bootstrap align-self-baseline class.
func AlignSelfBaseline() attr.Node { return rawClass("align-self-baseline") }

// AlignSelfStretch creates a Bootstrap align-self-stretch class.
func AlignSelfStretch() attr.Node { return rawClass("align-self-stretch") }

// FlexFill creates a Bootstrap flex-fill class.
func FlexFill() attr.Node { return rawClass("flex-fill") }

// FlexGrow0 creates a Bootstrap flex-grow-0 class.
func FlexGrow0() attr.Node { return rawClass("flex-grow-0") }

// FlexGrow1 creates a Bootstrap flex-grow-1 class.
func FlexGrow1() attr.Node { return rawClass("flex-grow-1") }

// FlexShrink0 creates a Bootstrap flex-shrink-0 class.
func FlexShrink0() attr.Node { return rawClass("flex-shrink-0") }

// FlexShrink1 creates a Bootstrap flex-shrink-1 class.
func FlexShrink1() attr.Node { return rawClass("flex-shrink-1") }

// Gap creates a Bootstrap gap-{n} class.
func Gap(n int) attr.Node { return rawClass(fmt.Sprintf("gap-%d", n)) }
