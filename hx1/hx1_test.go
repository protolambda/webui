package hx1_test

import (
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx1"
)

// Every helper writes its attribute, with the value HTML-escaped.
func TestAttributeHelpers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		node attr.Node
		want string
	}{
		{hx1.Get(`/a?b=1&c="2"`), `hx-get="/a?b=1&amp;c=&#34;2&#34;"`},
		{hx1.Post("/p"), `hx-post="/p"`},
		{hx1.Put("/p"), `hx-put="/p"`},
		{hx1.Patch("/p"), `hx-patch="/p"`},
		{hx1.Delete("/p"), `hx-delete="/p"`},
		{hx1.Boost("true"), `hx-boost="true"`},
		{hx1.Preserve(), `hx-preserve`},
		{hx1.Confirm("Delete 'x'?"), `hx-confirm="Delete &#39;x&#39;?"`},
		{hx1.Prompt("Name"), `hx-prompt="Name"`},
		{hx1.Disable(), `hx-disable`},
		{hx1.DisabledElt("this"), `hx-disabled-elt="this"`},
		{hx1.Disinherit("*"), `hx-disinherit="*"`},
		{hx1.Encoding("multipart/form-data"), `hx-encoding="multipart/form-data"`},
		{hx1.Ext("json-enc"), `hx-ext="json-enc"`},
		{hx1.PushURL("true"), `hx-push-url="true"`},
		{hx1.ReplaceURL("/r"), `hx-replace-url="/r"`},
		{hx1.HistoryElt(), `hx-history-elt`},
		{hx1.Indicator("#spin"), `hx-indicator="#spin"`},
		{hx1.Include("closest form"), `hx-include="closest form"`},
		{hx1.Params("*"), `hx-params="*"`},
		{hx1.Vals(`{"a":1}`), `hx-vals="{&#34;a&#34;:1}"`},
		{hx1.Headers(`{"X-A":"1"}`), `hx-headers="{&#34;X-A&#34;:&#34;1&#34;}"`},
		{hx1.Request(`{"timeout":100}`), `hx-request="{&#34;timeout&#34;:100}"`},
		{hx1.Sync("this:drop"), `hx-sync="this:drop"`},
		{hx1.Target("#t"), `hx-target="#t"`},
		{hx1.Swap("outerHTML"), `hx-swap="outerHTML"`},
		{hx1.SwapOOB("true"), `hx-swap-oob="true"`},
		{hx1.Select("#s"), `hx-select="#s"`},
		{hx1.SelectOOB("#a"), `hx-select-oob="#a"`},
		{hx1.Trigger("load"), `hx-trigger="load"`},
		{hx1.Validate("true"), `hx-validate="true"`},
		{hx1.SSE("connect:/e"), `hx-sse="connect:/e"`},
		{hx1.WS("connect:/w"), `hx-ws="connect:/w"`},
	}
	for _, c := range cases {
		var b strings.Builder
		t.Must(be.NoError(core.Render(t.Context(), div.Div(c.node), &b)))
		t.Must(be.Equal("<div "+c.want+"></div>", b.String()))
	}
}
