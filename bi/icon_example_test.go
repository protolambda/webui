package bi

import (
	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/html/text"
)

func ExampleIcon() {
	core.Dump(text.P()(text.Text("It's time "), Alarm, text.Strong()(text.Text("BEEP BEEP"))))
	// Output: <p>It&#39;s time <i class="bi bi-alarm"></i><strong>BEEP BEEP</strong></p>
}
