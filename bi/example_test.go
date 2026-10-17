package bi_test

import (
	"fmt"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
)

// An icon is a node: by default it is decorative and hidden from assistive
// technology, next to text that carries the meaning.
func ExampleIcon() {
	core.Dump(text.P()(bi.Alarm, text.Text(" It's time")))
	// Output: <p><i class="bi bi-alarm" aria-hidden="true"></i> It&#39;s time</p>
}

// A name that is not an icon name fails the render: it can neither break
// out of the class attribute nor add classes, and the error does not repeat
// it. Check a run-time name with Lookup instead of converting it.
func ExampleIcon_forged() {
	core.Dump(bi.Icon(`x" onclick="alert(1)`))
	core.Dump(bi.Icon("x position-fixed top-0 start-0 w-100 h-100"))
	// Output:
	// ERROR: render: at i[0]: attribute: bi: invalid icon name
	// ERROR: render: at i[0]: attribute: bi: invalid icon name
}

// An icon-only button gets its accessible name from the button; the icon
// stays decorative.
func ExampleIcon_iconOnlyButton() {
	core.Dump(button.Button(button.Type(button.TypeButton), aria.Label("Delete note"))(bi.Trash))
	// Output: <button type="button" aria-label="Delete note"><i class="bi bi-trash" aria-hidden="true"></i></button>
}

// With adds classes and attributes; the classes share one class attribute.
func ExampleIcon_With() {
	core.Dump(bi.ExclamationTriangleFill.With(attr.Class("fs-4 text-danger me-1")))
	// Output: <i class="bi bi-exclamation-triangle-fill fs-4 text-danger me-1" aria-hidden="true"></i>
}

// Label makes an icon that means something by itself an image with a name.
func ExampleIcon_Label() {
	core.Dump(bi.CheckCircleFill.Label("Completed", attr.Class("text-success")))
	// Output: <i class="bi bi-check-circle-fill text-success" role="img" aria-label="Completed"></i>
}

// A blank label fails the render instead of producing an unnamed image.
func ExampleIcon_Label_empty() {
	core.Dump(bi.CheckCircleFill.Label(" "))
	// Output: ERROR: render: at [0]: bi: empty icon label
}

// Lookup checks an icon name that is only known at run time.
func ExampleLookup() {
	for _, name := range []string{"folder2-open", "Folder", "no-such-icon"} {
		ic, ok := bi.Lookup(name)
		fmt.Printf("%q %v %q\n", name, ok, ic)
	}
	// Output:
	// "folder2-open" true "folder2-open"
	// "Folder" false ""
	// "no-such-icon" false ""
}

// All iterates over every icon of the release, in stylesheet order.
func ExampleAll() {
	n := 0
	for ic := range bi.All() {
		if n < 3 {
			fmt.Println(ic)
		}
		n++
	}
	fmt.Println(n, "icons in Bootstrap Icons", bi.Version)
	// Output:
	// 123
	// alarm-fill
	// alarm
	// 2078 icons in Bootstrap Icons 1.13.1
}
