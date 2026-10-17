package sse_test

import (
	"context"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/sse"
)

func renderString(ctx context.Context, n attr.Node) (string, error) {
	var b strings.Builder
	err := core.Render(ctx, div.Div(n)(), &b)
	return b.String(), err
}

func TestRegion(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := ct.View(section.Section(attr.ID("tasks-live"), aria.Label("Tasks"), sse.Region("/tasks/live?project=7&view=all"))(
		div.Div(attr.ID("tasks"), sse.Swap("tasks"), aria.Live("polite"))(text.Text("No tasks yet.")),
	))
	region := page.Find(ct.ID("tasks-live"))
	t.Must(region.Matches(
		ct.Attr("hx-ext", "sse"),
		ct.Attr("sse-connect", "/tasks/live?project=7&view=all"),
		ct.Attr("sse-swap", sse.EventEnd),
		ct.Attr("sse-close", sse.EventEnd),
	))
	t.Must(region.Find(ct.Attr("sse-swap", "tasks"), ct.Attr("aria-live", "polite")))

	got, err := renderString(t.Context(), sse.Connect("/live"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div hx-ext="sse" sse-connect="/live" sse-close="end"></div>`, got))
}

func TestRegionRejectsURLs(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, url := range []string{
		"", "live", "//other.example/live", "/\\other.example", "https://other.example/live",
		"javascript:alert(1)", "/\t/other.example", "/\n/other.example", "/a\rb", "/a b", "/a\\b",
		" /live", "/a\x00", "/a\x7f",
	} {
		for _, n := range []attr.Node{sse.Region(url), sse.Connect(url)} {
			_, err := renderString(t.Context(), n)
			t.Must(be.Annotate(be.ErrorIs(err, sse.ErrURL), "url %q", url))
		}
	}
	// The error does not repeat the URL, which may carry a token.
	_, err := renderString(t.Context(), sse.Region("/live?token=7f3a9c\n"))
	t.Must(be.ErrorIs(err, sse.ErrURL))
	t.Must(be.False(strings.Contains(err.Error(), "7f3a9c")))

	for _, url := range []string{"/", "/live", "/a/b?c=d&e=%20#f", "/日本"} {
		_, err := renderString(t.Context(), sse.Region(url))
		t.Must(be.Annotate(be.NoError(err), "url %q", url))
	}
}

func TestSwap(gt *testing.T) {
	t := devtest.ParallelT(gt)
	got, err := renderString(t.Context(), sse.Swap("a"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div sse-swap="a"></div>`, got))
	got, err = renderString(t.Context(), sse.Swap("a", "b", sse.EventEnd))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div sse-swap="a,b,end"></div>`, got))

	for _, events := range [][]string{{""}, {"a,b"}, {"open"}, {"a", "b c"}, {"a", ""}, {"a", "error"}} {
		_, err := renderString(t.Context(), sse.Swap(events[0], events[1:]...))
		t.Must(be.Annotate(be.ErrorIs(err, sse.ErrEventName), "events %q", events))
	}
}
