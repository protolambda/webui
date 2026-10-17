package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/web"
)

var notices = web.Notices{
	"saved":   {Message: "Task saved."},
	"deleted": {Message: "Task deleted.", Variant: bs.VariantInfo},
	"partial": {Message: "Task saved; the reminder could not be set.", Variant: bs.VariantWarning},
	"Bad":     {Message: "never valid"},
}

func TestNoticesTarget(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal("/tasks?notice=saved", notices.Target("/tasks", "saved")))
	t.Must(be.Equal("/tasks?page=2&notice=deleted#list", notices.Target("/tasks?notice=saved&page=2#list", "deleted")))
	t.Must(be.Panic(func() { notices.Target("/tasks", "unknown") }))
	t.Must(be.Panic(func() { notices.Target("/tasks", "Bad") }))
	t.Must(be.Panic(func() { notices.Target("/tasks", "") }))
}

func TestNoticesOf(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		query string
		want  string
	}{
		{"notice=saved", "Task saved."},
		{"page=2&notice=deleted", "Task deleted."},
		{"notice=saved&notice=deleted", "Task saved."},
		{"", ""},
		{"notice=", ""},
		{"notice=unknown", ""},
		{"notice=Bad", ""},
		{"notice=" + url.QueryEscape("<script>alert(1)</script>"), ""},
		{"notice=" + strings.Repeat("a", web.MaxNoticeCode+1), ""},
		{"notice=saved%00", ""},
	}
	for _, c := range cases {
		r := request(t, nil, http.MethodGet, "/tasks?"+c.query, nil)
		n, ok := notices.Of(r)
		t.Mustf(be.Equal(c.want != "", ok), "query %q", c.query)
		t.Mustf(be.Equal(c.want, n.Message), "query %q", c.query)
	}
}

func TestNoticesAlert(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := ct.View(notices.Alert(request(t, nil, http.MethodGet, "/tasks?notice=partial", nil)))
	t.Must(view.Find(ct.Role("status")).Matches(ct.Class("alert-warning"), ct.InnerText("Task saved; the reminder could not be set.")))
	t.Must(ct.View(notices.Alert(request(t, nil, http.MethodGet, "/tasks?notice=%3Cb%3E", nil))).Find(ct.Role("status")).None())
	t.Must(ct.View(notices["saved"].Alert()).Find(ct.Role("status")).Matches(ct.Class("alert-success")))
}

func TestNoticesRedirect(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notices.Redirect(w, r, "/tasks", "saved")
	})
	res := cthttp.Serve(h, request(t, nil, http.MethodPost, "/tasks", nil))
	t.Must(res.Status(http.StatusSeeOther))
	t.Must(res.Header("Location", ct.Exact("/tasks?notice=saved")))
	res = cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), "https://app.example/tasks"))
	t.Must(res.Header("HX-Redirect", ct.Exact("/tasks?notice=saved")))
}
