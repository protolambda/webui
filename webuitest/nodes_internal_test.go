package webuitest

import (
	"testing"

	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

func TestElementsByTag(gt *testing.T) {
	t := devtest.ParallelT(gt)
	doc, err := cthtml.PageString(`<!DOCTYPE html><html><body><div><p>one</p><span><p>two</p></span></div></body></html>`).Load(t.Context())
	t.Must(be.NoError(err))
	var texts []string
	for e := range elementsByTag(doc.Root(), "P") {
		texts = append(texts, rawText(e))
	}
	t.Must(be.DeepEqual([]string{"one", "two"}, texts))
	for range elementsByTag(nil, "p") {
		t.Fatal("nil has no elements")
	}
}
