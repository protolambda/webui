package hx2_test

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
)

// renderAttr renders a div with the attribute.
func renderAttr(t devtest.T, a attr.Node) (string, error) {
	t.Helper()
	return renderNode(t.Context(), div.Div(a))
}

// parsedAttr renders a div with the attribute, parses the HTML, and returns
// the value of the named attribute as a browser reads it.
func parsedAttr(t devtest.T, a attr.Node, name string) string {
	t.Helper()
	out, err := renderAttr(t, a)
	t.Must(be.NoError(err))
	v, ok, err := cthtml.FragmentString("body", out).Find(ct.Tag("div")).Attr(t.Context(), name)
	t.Must(be.NoError(err))
	t.Must(be.True(ok))
	return v
}

func TestHistory(gt *testing.T) {
	t := devtest.ParallelT(gt)
	out, err := renderAttr(t, hx2.History(false))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div hx-history="false"></div>`, out))
	out, err = renderAttr(t, hx2.History(true))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div hx-history="true"></div>`, out))
}

func TestValsJSON(gt *testing.T) {
	t := devtest.ParallelT(gt)
	type item struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Draft bool   `json:"draft,omitempty"`
	}
	t.Run("struct", func(t devtest.T) {
		got := parsedAttr(t, hx2.ValsJSON(item{ID: 7, Name: `a "quoted" <name> & 'more'`}), "hx-vals")
		var back item
		t.Must(be.NoError(json.Unmarshal([]byte(got), &back)))
		t.Must(be.Equal(item{ID: 7, Name: `a "quoted" <name> & 'more'`}, back))
	})
	t.Run("map in sorted order", func(t devtest.T) {
		out, err := renderAttr(t, hx2.ValsJSON(map[string]any{"op": "move", "b": 2, "a": []int{1}}))
		t.Must(be.NoError(err))
		t.Must(be.Equal(`<div hx-vals="{&#34;a&#34;:[1],&#34;b&#34;:2,&#34;op&#34;:&#34;move&#34;}"></div>`, out))
	})
	t.Run("pointer and empty object", func(t devtest.T) {
		t.Must(be.Equal(`{"id":1,"name":""}`, parsedAttr(t, hx2.ValsJSON(&item{ID: 1}), "hx-vals")))
		t.Must(be.Equal(`{}`, parsedAttr(t, hx2.ValsJSON(map[string]int{}), "hx-vals")))
		t.Must(be.Equal(`{}`, parsedAttr(t, hx2.ValsJSON(struct{}{}), "hx-vals")))
	})
	t.Run("encoded on the call", func(t devtest.T) {
		m := map[string]string{"k": "before"}
		a := hx2.ValsJSON(m)
		m["k"] = "after"
		t.Must(be.Equal(`{"k":"before"}`, parsedAttr(t, a, "hx-vals")))
	})
	t.Run("not an object", func(t devtest.T) {
		var nilMap map[string]any
		var nilItem *item
		for _, v := range []any{nil, nilMap, nilItem, []int{1}, "js:alert(1)", 3, true, json.RawMessage(`[1]`)} {
			out, err := renderAttr(t, hx2.ValsJSON(v))
			t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidValue), "value %#v", v))
			t.Must(be.Equal("", out))
		}
	})
	t.Run("not encodable", func(t devtest.T) {
		for _, v := range []any{
			map[string]any{"c": make(chan int)},
			map[string]any{"f": func() {}},
			map[string]float64{"x": math.NaN()},
			json.RawMessage(`{"broken"`),
		} {
			out, err := renderAttr(t, hx2.ValsJSON(v))
			t.Must(be.ErrorIs(err, hx2.ErrInvalidValue))
			t.Must(be.Equal("", out))
		}
		_, err := renderAttr(t, hx2.ValsJSON(map[string]any{"c": make(chan int)}))
		var typeErr *json.UnsupportedTypeError
		t.Must(be.True(errors.As(err, &typeErr)))
	})
}

// prototypeNames are the properties of JavaScript's Object.prototype. htmx
// merges hx-vals and hx-headers into a plain object, so it drops members of
// these names, and throws on "hasOwnProperty".
var prototypeNames = []string{
	"__proto__", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__",
	"constructor", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable",
	"toLocaleString", "toString", "valueOf",
}

func TestValsJSONPrototypeNames(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("map", func(t devtest.T) {
		for _, name := range prototypeNames {
			out, err := renderAttr(t, hx2.ValsJSON(map[string]string{"a": "1", name: "x"}))
			t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidValue), "name %q", name))
			t.Must(be.Equal("", out))
		}
	})
	t.Run("struct tag", func(t devtest.T) {
		type kind struct {
			Constructor string `json:"constructor"`
		}
		_, err := renderAttr(t, hx2.ValsJSON(kind{Constructor: "x"}))
		t.Must(be.ErrorIs(err, hx2.ErrInvalidValue))
	})
	t.Run("escaped name", func(t devtest.T) {
		// htmx reads the decoded name.
		_, err := renderAttr(t, hx2.ValsJSON(json.RawMessage(`{"\u0063onstructor":1}`)))
		t.Must(be.ErrorIs(err, hx2.ErrInvalidValue))
	})
	t.Run("allowed", func(t devtest.T) {
		// Names are compared exactly, and only top-level members are merged:
		// htmx sends a nested object as its JSON text.
		for _, v := range []any{
			map[string]string{"Constructor": "x", "to_string": "y", "proto": "z"},
			map[string]any{"a": map[string]int{"constructor": 1, "hasOwnProperty": 2}},
			map[string]any{"a": []string{"toString"}},
			map[string]string{"name": "constructor"},
		} {
			_, err := renderAttr(t, hx2.ValsJSON(v))
			t.Must(be.Annotate(be.NoError(err), "value %#v", v))
		}
	})
}

// encoding/json writes nil slices, maps, pointers and interfaces as null,
// which htmx sends as the text "null"; omitempty leaves the member out.
func TestValsJSONNullMembers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	type filter struct {
		Tags  []string `json:"tags"`
		Owner *string  `json:"owner"`
		Kinds []string `json:"kinds,omitempty"`
		Empty []string `json:"empty"`
	}
	got := parsedAttr(t, hx2.ValsJSON(filter{Empty: []string{}}), "hx-vals")
	t.Must(be.Equal(`{"tags":null,"owner":null,"empty":[]}`, got))
}

func TestHeadersJSON(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("sorted", func(t devtest.T) {
		out, err := renderAttr(t, hx2.HeadersJSON(map[string]string{"X-Item-Version": "3", "X-A": `"q" <b>`}))
		t.Must(be.NoError(err))
		t.Must(be.Equal(`<div hx-headers="{&#34;X-A&#34;:&#34;\&#34;q\&#34; \u003cb\u003e&#34;,&#34;X-Item-Version&#34;:&#34;3&#34;}"></div>`, out))
		got := parsedAttr(t, hx2.HeadersJSON(map[string]string{"X-A": `"q" <b>`}), "hx-headers")
		var back map[string]string
		t.Must(be.NoError(json.Unmarshal([]byte(got), &back)))
		t.Must(be.Equal(`"q" <b>`, back["X-A"]))
	})
	t.Run("empty", func(t devtest.T) {
		t.Must(be.Equal(`{}`, parsedAttr(t, hx2.HeadersJSON(nil), "hx-headers")))
		t.Must(be.Equal(`{}`, parsedAttr(t, hx2.HeadersJSON(map[string]string{}), "hx-headers")))
	})
	t.Run("token characters", func(t devtest.T) {
		name := "!#$%&'*+-.^_`|~09azAZ"
		got := parsedAttr(t, hx2.HeadersJSON(map[string]string{name: "v"}), "hx-headers")
		var back map[string]string
		t.Must(be.NoError(json.Unmarshal([]byte(got), &back)))
		t.Must(be.Equal("v", back[name]))
	})
	t.Run("encoded on the call", func(t devtest.T) {
		h := map[string]string{"X-K": "before"}
		a := hx2.HeadersJSON(h)
		h["X-K"] = "after"
		h["bad name"] = "x"
		t.Must(be.Equal(`{"X-K":"before"}`, parsedAttr(t, a, "hx-headers")))
	})
	t.Run("prototype names", func(t devtest.T) {
		// All are valid tokens, so only the prototype check refuses them.
		for _, name := range prototypeNames {
			out, err := renderAttr(t, hx2.HeadersJSON(map[string]string{"X-Ok": "1", name: "v"}))
			t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidValue), "name %q", name))
			t.Must(be.Equal("", out))
		}
		// Names are compared exactly: header names are case-insensitive, but
		// JavaScript property names are not.
		got := parsedAttr(t, hx2.HeadersJSON(map[string]string{"Constructor": "v", "ToString": "w"}), "hx-headers")
		t.Must(be.Equal(`{"Constructor":"v","ToString":"w"}`, got))
	})
	t.Run("invalid names", func(t devtest.T) {
		for _, name := range []string{"", "X Y", "X:Y", "X\nY", "X\x00", "naïve", "X(Y)", `"X"`, "X,Y", "X/Y", "X@Y", "{X}"} {
			out, err := renderAttr(t, hx2.HeadersJSON(map[string]string{"X-Ok": "1", name: "v"}))
			t.Must(be.Annotate(be.ErrorIs(err, hx2.ErrInvalidValue), "name %q", name))
			t.Must(be.Equal("", out))
		}
	})
}

func TestOnEventNames(gt *testing.T) {
	t := devtest.ParallelT(gt)
	out, err := renderAttr(t, hx2.On("htmx:after-swap", "x()"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div hx-on:htmx:after-swap="x()"></div>`, out))
	out, err = renderAttr(t, hx2.On(":after-swap", "x()"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div hx-on::after-swap="x()"></div>`, out))

	// The handler is HTML-escaped (which keeps the attribute intact; it does
	// not make the JavaScript safe).
	t.Must(be.Equal(`a("b") && c('d')`, parsedAttr(t, hx2.On("click", `a("b") && c('d')`), "hx-on:click")))

	// An event that cannot be part of an attribute name fails the render.
	for _, event := range []string{"click x", `click"`, "click>", "click=", "a/b", "a\tb"} {
		out, err := renderAttr(t, hx2.On(event, "x()"))
		t.Must(be.Annotate(be.ErrorIs(err, attr.ErrInvalidName), "event %q", event))
		t.Must(be.Equal("", out))
	}
}

// The swap styles are the names that htmx 2 switches on.
func TestSwapStyles(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.DeepEqual(
		[]string{"innerHTML", "outerHTML", "textContent", "beforebegin", "afterbegin", "beforeend", "afterend", "delete", "none"},
		[]string{hx2.SwapInnerHTML, hx2.SwapOuterHTML, hx2.SwapTextContent, hx2.SwapBeforeBegin,
			hx2.SwapAfterBegin, hx2.SwapBeforeEnd, hx2.SwapAfterEnd, hx2.SwapDelete, hx2.SwapNone}))
	out, err := renderAttr(t, hx2.Swap(hx2.SwapOuterHTML+" show:top"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(`<div hx-swap="outerHTML show:top"></div>`, out))
}

// The plain attribute helpers write their attribute with the escaped value.
func TestAttributeHelpers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		node attr.Node
		want string
	}{
		{hx2.Get(`/a?b=1&c="2"`), `hx-get="/a?b=1&amp;c=&#34;2&#34;"`},
		{hx2.Post("/p"), `hx-post="/p"`},
		{hx2.Put("/p"), `hx-put="/p"`},
		{hx2.Patch("/p"), `hx-patch="/p"`},
		{hx2.Delete("/p"), `hx-delete="/p"`},
		{hx2.Boost("true"), `hx-boost="true"`},
		{hx2.Preserve(), `hx-preserve`},
		{hx2.Confirm("Delete 'x'?"), `hx-confirm="Delete &#39;x&#39;?"`},
		{hx2.Prompt("Name"), `hx-prompt="Name"`},
		{hx2.Disable(), `hx-disable`},
		{hx2.DisabledElt("find button"), `hx-disabled-elt="find button"`},
		{hx2.Disinherit("*"), `hx-disinherit="*"`},
		{hx2.Encoding("multipart/form-data"), `hx-encoding="multipart/form-data"`},
		{hx2.Ext("sse"), `hx-ext="sse"`},
		{hx2.PushURL("false"), `hx-push-url="false"`},
		{hx2.ReplaceURL("/r"), `hx-replace-url="/r"`},
		{hx2.HistoryElt(), `hx-history-elt`},
		{hx2.Indicator("#spin"), `hx-indicator="#spin"`},
		{hx2.Include("closest form"), `hx-include="closest form"`},
		{hx2.Params("not secret"), `hx-params="not secret"`},
		{hx2.Vals(`{"a":1}`), `hx-vals="{&#34;a&#34;:1}"`},
		{hx2.Headers(`{"X-A":"1"}`), `hx-headers="{&#34;X-A&#34;:&#34;1&#34;}"`},
		{hx2.Request(`{"timeout":100}`), `hx-request="{&#34;timeout&#34;:100}"`},
		{hx2.Sync("this:replace"), `hx-sync="this:replace"`},
		{hx2.Target("#t"), `hx-target="#t"`},
		{hx2.Swap(hx2.SwapNone), `hx-swap="none"`},
		{hx2.SwapOOB("true"), `hx-swap-oob="true"`},
		{hx2.Select("#s"), `hx-select="#s"`},
		{hx2.SelectOOB("#a,#b"), `hx-select-oob="#a,#b"`},
		{hx2.Trigger("every 2s"), `hx-trigger="every 2s"`},
		{hx2.Validate("true"), `hx-validate="true"`},
		{hx2.Inherit("hx-target"), `hx-inherit="hx-target"`},
		{hx2.SSEConnect("/events"), `sse-connect="/events"`},
		{hx2.SSESwap("panel"), `sse-swap="panel"`},
		{hx2.SSEClose("done"), `sse-close="done"`},
		{hx2.WSConnect("/ws"), `ws-connect="/ws"`},
		{hx2.WSSend(), `ws-send`},
	}
	for _, c := range cases {
		out, err := renderAttr(t, c.node)
		t.Must(be.NoError(err))
		t.Must(be.Equal("<div "+c.want+"></div>", out))
	}
}

func TestAddVary(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		have []string
		want []string
	}{
		{"none", nil, []string{"HX-Request"}},
		{"other", []string{"Accept-Encoding"}, []string{"Accept-Encoding", "HX-Request"}},
		{"other list", []string{"Accept-Encoding, Origin"}, []string{"Accept-Encoding, Origin", "HX-Request"}},
		{"present", []string{"HX-Request"}, []string{"HX-Request"}},
		{"present lower case", []string{"hx-request"}, []string{"hx-request"}},
		{"present in list", []string{"Origin,  HX-Request ,Accept"}, []string{"Origin,  HX-Request ,Accept"}},
		{"present in second line", []string{"Origin", "HX-Request"}, []string{"Origin", "HX-Request"}},
		{"star", []string{"*"}, []string{"*"}},
		{"similar names", []string{"HX-Request-Id, XHX-Request"}, []string{"HX-Request-Id, XHX-Request", "HX-Request"}},
		{"empty value", []string{""}, []string{"", "HX-Request"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			h := http.Header{}
			h.Set("Content-Type", "text/html")
			for _, v := range c.have {
				h.Add("Vary", v)
			}
			hx2.AddVary(h)
			t.Must(be.DeepEqual(c.want, h.Values("Vary")))
			// Again: no change.
			hx2.AddVary(h)
			t.Must(be.DeepEqual(c.want, h.Values("Vary")))
			t.Must(be.Equal("text/html", h.Get("Content-Type")))
		})
	}
	// The header is the canonical one.
	h := http.Header{}
	hx2.AddVary(h)
	t.Must(be.Equal(1, len(h)))
	t.Must(be.Equal("HX-Request", strings.Join(h["Vary"], ",")))
}
