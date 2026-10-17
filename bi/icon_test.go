package bi_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/bi"
)

func render(t devtest.T, n elem.Node) string {
	t.Helper()
	var out strings.Builder
	t.Must(be.NoError(core.Render(context.Background(), n, &out)))
	return out.String()
}

func renderErr(n elem.Node) error {
	var out strings.Builder
	return core.Render(context.Background(), n, &out)
}

func TestIconDecorative(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(`<i class="bi bi-alarm" aria-hidden="true"></i>`, render(t, bi.Alarm)))
	t.Must(be.Equal(`<i class="bi bi-alarm" aria-hidden="true"></i>`, render(t, bi.Alarm.With())))
	t.Must(be.Equal(`<i class="bi bi-123" aria-hidden="true"></i>`, render(t, bi.Num123)))

	page := ct.View(bi.Alarm)
	t.Must(page.Find(ct.Tag("i"), ct.Attr("aria-hidden", "true")).Count(1))
	t.Must(page.Find(ct.Role("img")).None())
}

func TestForgedIconValueIsEscaped(gt *testing.T) {
	t := devtest.ParallelT(gt)
	forged := bi.Icon(`alarm" onclick="attack()`)
	t.Must(be.Equal(`<i class="bi bi-alarm&#34; onclick=&#34;attack()" aria-hidden="true"></i>`, render(t, forged)))
	t.Must(be.Equal(`<i class="bi bi-alarm&#34; onclick=&#34;attack() fs-4" aria-hidden="true"></i>`,
		render(t, forged.With(attr.Class("fs-4")))))
	t.Must(be.Equal(`<i class="bi bi-alarm&#34; onclick=&#34;attack()" role="img" aria-label="x"></i>`,
		render(t, forged.Label("x"))))
	t.Must(ct.View(forged).Find(ct.HasAttr("onclick")).None())
}

func TestIconNamesOutsideTheSafeSetAreEscaped(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, want := range map[bi.Icon]string{
		"":         `<i class="bi bi-" aria-hidden="true"></i>`,
		"Alarm":    `<i class="bi bi-Alarm" aria-hidden="true"></i>`,
		"a&b":      `<i class="bi bi-a&amp;b" aria-hidden="true"></i>`,
		"a<b>":     `<i class="bi bi-a&lt;b&gt;" aria-hidden="true"></i>`,
		"x'y":      `<i class="bi bi-x&#39;y" aria-hidden="true"></i>`,
		"é":        "<i class=\"bi bi-é\" aria-hidden=\"true\"></i>",
		"custom-1": `<i class="bi bi-custom-1" aria-hidden="true"></i>`,
	} {
		t.Mustf(be.Equal(want, render(t, name)), "icon %q", string(name))
	}
}

func TestIconWith(gt *testing.T) {
	t := devtest.ParallelT(gt)
	got := render(t, bi.Trash.With(attr.Class("fs-4"), attr.Class("text-danger me-1"), attr.Title("Removed")))
	t.Must(be.Equal(`<i class="bi bi-trash fs-4 text-danger me-1" aria-hidden="true" title="Removed"></i>`, got))
	t.Must(be.Equal(1, strings.Count(got, "class=")))

	// An escaped extra class merges into the raw icon class safely.
	got = render(t, bi.Trash.With(attr.Class(`x" onclick="y`)))
	t.Must(be.Equal(`<i class="bi bi-trash x&#34; onclick=&#34;y" aria-hidden="true"></i>`, got))
}

func TestIconWithDuplicateAttribute(gt *testing.T) {
	t := devtest.ParallelT(gt)
	err := renderErr(bi.Trash.With(aria.Hidden("false")))
	t.Must(be.ErrorContains(err, `duplicate attribute "aria-hidden"`))
	err = renderErr(bi.Trash.Label("Delete", aria.Label("Other")))
	t.Must(be.ErrorContains(err, `duplicate attribute "aria-label"`))
	err = renderErr(bi.Trash.With(nil))
	t.Must(be.ErrorIs(err, attr.ErrNilNode))
}

func TestIconLabel(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(`<i class="bi bi-check-circle" role="img" aria-label="Done"></i>`,
		render(t, bi.CheckCircle.Label("Done"))))
	t.Must(be.Equal(`<i class="bi bi-check-circle text-success" role="img" aria-label="Done &amp; &lt;checked&gt; &#34;ok&#34;"></i>`,
		render(t, bi.CheckCircle.Label(`Done & <checked> "ok"`, attr.Class("text-success")))))

	page := ct.View(bi.CheckCircle.Label("Done"))
	t.Must(page.Find(ct.Role("img", ct.Named("Done"))).Count(1))
	t.Must(page.Find(ct.HasAttr("aria-hidden")).None())
}

func TestIconLabelEmpty(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, label := range []string{"", " ", "\t\n", "  "} {
		err := renderErr(bi.Alarm.Label(label))
		t.Mustf(be.ErrorIs(err, bi.ErrEmptyLabel), "label %q", label)
	}
}

func TestIconOnlyButtonPattern(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(button.Button(button.Type(button.TypeButton), aria.Label("Delete note"))(bi.Trash))
	t.Must(page.Find(ct.Role("button", ct.Named("Delete note"))).Count(1))
}

func TestLookup(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, want := range map[string]bi.Icon{
		"alarm":                    bi.Alarm,
		"alarm-fill":               bi.AlarmFill,
		"123":                      bi.Num123,
		"1-circle-fill":            bi.Num1CircleFill,
		"x-lg":                     bi.XLg,
		"globe-europe-africa-fill": bi.GlobeEuropeAfricaFill,
		"zoom-out":                 bi.ZoomOut,
	} {
		got, ok := bi.Lookup(name)
		t.Mustf(be.True(ok), "Lookup(%q)", name)
		t.Must(be.Equal(want, got))
	}
	for _, name := range []string{"", "Alarm", "bi-alarm", "alarm ", " alarm", "alarm\x00", `alarm" x`, "zzzz", "0", "alarm-fil"} {
		got, ok := bi.Lookup(name)
		t.Mustf(be.False(ok), "Lookup(%q)", name)
		t.Must(be.Equal(bi.Icon(""), got))
	}
}

func TestAllAndLookupAgree(gt *testing.T) {
	t := devtest.ParallelT(gt)
	n := 0
	for ic := range bi.All() {
		got, ok := bi.Lookup(string(ic))
		t.Mustf(be.True(ok), "Lookup(%q)", string(ic))
		t.Must(be.Equal(ic, got))
		n++
	}
	t.Must(be.Equal(2078, n))

	// Early break stops the iterator, and the iterator is restartable.
	var first []bi.Icon
	for ic := range bi.All() {
		first = append(first, ic)
		if len(first) == 2 {
			break
		}
	}
	t.Must(be.DeepEqual([]bi.Icon{bi.Num123, bi.AlarmFill}, first))
}

func TestConcurrentUse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	node := bi.Alarm.With(attr.Class("fs-4"))
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() {
			var out strings.Builder
			errs[i] = core.Render(context.Background(), node, &out)
			if errs[i] == nil && out.String() != `<i class="bi bi-alarm fs-4" aria-hidden="true"></i>` {
				errs[i] = errUnexpected(out.String())
			}
			_, _ = bi.Lookup("alarm")
		})
	}
	wg.Wait()
	for _, err := range errs {
		t.Must(be.NoError(err))
	}
}

type errUnexpected string

func (e errUnexpected) Error() string { return "unexpected output: " + string(e) }
