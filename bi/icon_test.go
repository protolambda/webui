package bi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"

	"github.com/protolambda/webui/bi"
)

func TestForgedIconValueIsEscaped(t *testing.T) {
	var out strings.Builder
	if err := core.Render(context.Background(), bi.Icon(`alarm" onclick="attack()`), &out); err != nil {
		t.Fatalf("render: %v", err)
	}

	if got, want := out.String(), `<i class="bi bi-alarm&#34; onclick=&#34;attack()"></i>`; got != want {
		t.Fatalf("rendered output mismatch:\n got: %s\nwant: %s", got, want)
	}
}
