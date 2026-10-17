package ba

import (
	"fmt"

	"github.com/protolambda/chord/core/attr"
)

// Container returns a "container" class attribute.
func Container() attr.Node { return rawClass("container") }

// ContainerFluid returns a "container-fluid" class attribute.
func ContainerFluid() attr.Node { return rawClass("container-fluid") }

// ContainerSM returns a "container-sm" class attribute.
func ContainerSM() attr.Node { return rawClass("container-sm") }

// ContainerMD returns a "container-md" class attribute.
func ContainerMD() attr.Node { return rawClass("container-md") }

// ContainerLG returns a "container-lg" class attribute.
func ContainerLG() attr.Node { return rawClass("container-lg") }

// ContainerXL returns a "container-xl" class attribute.
func ContainerXL() attr.Node { return rawClass("container-xl") }

// ContainerXXL returns a "container-xxl" class attribute.
func ContainerXXL() attr.Node { return rawClass("container-xxl") }

// Row returns a "row" class attribute.
func Row() attr.Node { return rawClass("row") }

// Col returns a "col" class attribute.
func Col() attr.Node { return rawClass("col") }

// Col1 returns a "col-1" class attribute.
func Col1() attr.Node { return rawClass("col-1") }

// Col2 returns a "col-2" class attribute.
func Col2() attr.Node { return rawClass("col-2") }

// Col3 returns a "col-3" class attribute.
func Col3() attr.Node { return rawClass("col-3") }

// Col4 returns a "col-4" class attribute.
func Col4() attr.Node { return rawClass("col-4") }

// Col5 returns a "col-5" class attribute.
func Col5() attr.Node { return rawClass("col-5") }

// Col6 returns a "col-6" class attribute.
func Col6() attr.Node { return rawClass("col-6") }

// Col7 returns a "col-7" class attribute.
func Col7() attr.Node { return rawClass("col-7") }

// Col8 returns a "col-8" class attribute.
func Col8() attr.Node { return rawClass("col-8") }

// Col9 returns a "col-9" class attribute.
func Col9() attr.Node { return rawClass("col-9") }

// Col10 returns a "col-10" class attribute.
func Col10() attr.Node { return rawClass("col-10") }

// Col11 returns a "col-11" class attribute.
func Col11() attr.Node { return rawClass("col-11") }

// Col12 returns a "col-12" class attribute.
func Col12() attr.Node { return rawClass("col-12") }

// ColSM returns a "col-sm-{n}" class attribute.
func ColSM(n int) attr.Node { return rawClass(fmt.Sprintf("col-sm-%d", n)) }

// ColMD returns a "col-md-{n}" class attribute.
func ColMD(n int) attr.Node { return rawClass(fmt.Sprintf("col-md-%d", n)) }

// ColLG returns a "col-lg-{n}" class attribute.
func ColLG(n int) attr.Node { return rawClass(fmt.Sprintf("col-lg-%d", n)) }

// ColXL returns a "col-xl-{n}" class attribute.
func ColXL(n int) attr.Node { return rawClass(fmt.Sprintf("col-xl-%d", n)) }

// ColXXL returns a "col-xxl-{n}" class attribute.
func ColXXL(n int) attr.Node { return rawClass(fmt.Sprintf("col-xxl-%d", n)) }

// ColAuto returns a "col-auto" class attribute.
func ColAuto() attr.Node { return rawClass("col-auto") }

// Offset returns an "offset-{n}" class attribute.
func Offset(n int) attr.Node { return rawClass(fmt.Sprintf("offset-%d", n)) }

// OffsetSM returns an "offset-sm-{n}" class attribute.
func OffsetSM(n int) attr.Node { return rawClass(fmt.Sprintf("offset-sm-%d", n)) }

// OffsetMD returns an "offset-md-{n}" class attribute.
func OffsetMD(n int) attr.Node { return rawClass(fmt.Sprintf("offset-md-%d", n)) }

// OffsetLG returns an "offset-lg-{n}" class attribute.
func OffsetLG(n int) attr.Node { return rawClass(fmt.Sprintf("offset-lg-%d", n)) }
