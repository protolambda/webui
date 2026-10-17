package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/web"
)

var errView = errors.New("view failed")

// failing is a view that fails after some content rendered.
func failing() elem.Node {
	return elem.Bundle{
		text.P()(text.Text("first part")),
		elem.Fn(func(context.Context) (elem.Node, error) { return nil, errView }),
	}
}

func TestRender(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rec := httptest.NewRecorder()
	err := web.Render(rec, request(t, nil, http.MethodGet, "/", nil), http.StatusCreated, text.P()(text.Text("a < b")))
	t.Must(be.NoError(err))
	t.Must(be.Equal(http.StatusCreated, rec.Code))
	t.Must(be.Equal(web.ContentType, rec.Header().Get("Content-Type")))
	t.Must(be.Equal("<p>a &lt; b</p>", rec.Body.String()))
	t.Must(be.Equal("", rec.Header().Get("Vary")))
}

func TestRenderFailureWritesNothing(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	err := web.Render(w, request(t, nil, http.MethodGet, "/", nil), http.StatusOK, failing())
	t.Must(be.ErrorIs(err, errView))
	t.Must(be.Equal(0, w.status))
	t.Must(be.Equal("", w.body.String()))
	t.Must(be.Equal(0, len(w.header)))
}

func TestRenderWriteError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	w.writeErr = errors.New("connection reset")
	err := web.Render(w, request(t, nil, http.MethodGet, "/", nil), http.StatusOK, text.Text("x"))
	t.Must(be.ErrorIs(err, w.writeErr))
	t.Must(be.Equal(http.StatusOK, w.status))
}

func TestRenderUsesRequestContext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	type key struct{}
	r := request(t, nil, http.MethodGet, "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), key{}, "from the request"))
	rec := httptest.NewRecorder()
	err := web.Render(rec, r, http.StatusOK, elem.Fn(func(ctx context.Context) (elem.Node, error) {
		return text.Text(ctx.Value(key{}).(string)), nil
	}))
	t.Must(be.NoError(err))
	t.Must(be.Equal("from the request", rec.Body.String()))
}

func TestChoose(gt *testing.T) {
	t := devtest.ParallelT(gt)
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		err := web.Choose(w, r, http.StatusOK, text.P()(text.Text("fragment")), text.P()(text.Text("full")))
		if err != nil {
			panic(err)
		}
	}
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"plain", nil, "full"},
		{"htmx", map[string]string{hx2.HeaderRequest: "true"}, "fragment"},
		// htmx swaps a boosted answer into the whole body.
		{"htmx boosted", map[string]string{hx2.HeaderRequest: "true", hx2.HeaderBoosted: "true"}, "full"},
		{"htmx boosted not true", map[string]string{hx2.HeaderRequest: "true", hx2.HeaderBoosted: "1"}, "fragment"},
		{"boosted without htmx", map[string]string{hx2.HeaderBoosted: "true"}, "full"},
		{"htmx history restore", map[string]string{hx2.HeaderRequest: "true", hx2.HeaderHistoryRestoreRequest: "true"}, "full"},
		{"history restore without htmx", map[string]string{hx2.HeaderHistoryRestoreRequest: "true"}, "full"},
		{"not true", map[string]string{hx2.HeaderRequest: "1"}, "full"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			r := request(t, nil, http.MethodGet, "/", nil)
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
			t.Must(be.Equal(c.want == "fragment", web.WantsFragment(r)))
			res := cthttp.Serve(http.HandlerFunc(handler), r)
			t.Must(res.Status(http.StatusOK))
			t.Must(res.Body(ct.Exact("<p>" + c.want + "</p>")))
			t.Must(be.DeepEqual([]string{"Accept-Encoding", "HX-Request", "HX-Boosted"}, res.Headers().Values("Vary")))
		})
	}
}

// TestChooseVaryOnce checks that Choose keeps the Vary values that name
// its headers already, in any letter case, or "*".
func TestChooseVaryOnce(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		vary []string
		want []string
	}{
		{"both", []string{"hx-request, hx-boosted"}, []string{"hx-request, hx-boosted"}},
		{"star", []string{"*"}, []string{"*"}},
		{"boosted", []string{"Accept-Encoding,HX-Boosted"}, []string{"Accept-Encoding,HX-Boosted", "HX-Request"}},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		rec.Header()["Vary"] = c.vary
		err := web.Choose(rec, request(t, nil, http.MethodGet, "/", nil), http.StatusOK, text.Text("f"), text.Text("p"))
		t.Must(be.NoError(err))
		t.Mustf(be.DeepEqual(c.want, rec.Header().Values("Vary")), "case %s", c.name)
	}
}

func TestIsPrefetch(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name     string
		method   string
		headers  map[string]string
		prefetch bool
	}{
		{"plain get", http.MethodGet, nil, false},
		{"plain head", http.MethodHead, nil, false},
		{"plain post", http.MethodPost, nil, false},
		{"sec-purpose prefetch", http.MethodGet, map[string]string{"Sec-Purpose": "prefetch"}, true},
		{"sec-purpose prerender", http.MethodGet, map[string]string{"Sec-Purpose": "prefetch;prerender"}, true},
		{"sec-purpose other", http.MethodGet, map[string]string{"Sec-Purpose": "anything"}, true},
		{"purpose", http.MethodGet, map[string]string{"Purpose": "prefetch"}, true},
		{"purpose upper", http.MethodGet, map[string]string{"Purpose": "Prefetch"}, true},
		{"x-purpose preview", http.MethodGet, map[string]string{"X-Purpose": "preview"}, true},
		{"x-moz", http.MethodGet, map[string]string{"X-Moz": "prefetch"}, true},
		{"purpose other", http.MethodGet, map[string]string{"Purpose": "navigate"}, false},
	}
	for _, c := range cases {
		r := request(t, nil, c.method, "/confirm", nil)
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		t.Mustf(be.Equal(c.prefetch, web.IsPrefetch(r)), "IsPrefetch %s", c.name)
		t.Mustf(be.Equal(c.method == http.MethodGet && !c.prefetch, web.MayAct(r)), "MayAct %s", c.name)
	}
}
