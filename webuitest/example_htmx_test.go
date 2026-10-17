package webuitest_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	webform "github.com/protolambda/webui/form"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// SubmitHTMX sends what htmx sends for an element of a page: here the
// hx-delete of a form whose action is the fallback without script, with
// the form's fields in the body (webui's htmx configuration) and htmx's
// headers. Submit with HTMX would send the fallback with htmx's headers, a
// request that no browser makes, and fails.
func ExampleClient_SubmitHTMX() {
	t := webuitest.PrintT{}
	const csrf = "csrf-token"
	set := theme.MustAssets(theme.Options{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /notes", func(w http.ResponseWriter, r *http.Request) {
		_ = web.Render(w, r, http.StatusOK, page.Document{
			Title:  "Notes",
			Assets: []*asset.Set{set},
			Body: page.Layout{Main: list.UL(attr.ID("notes"))(list.LI()(
				text.Text("Groceries "),
				form.Form(attr.ID("delete-1"), form.Method(form.MethodPost), form.Action("/notes/1/delete"),
					hx2.Delete("/notes/1"), hx2.Target("#notes"))(
					session.CSRFInput(csrf),
					bs.BtnDanger(button.Type(button.TypeSubmit))(text.Text("Delete")),
				),
			))},
		})
	})
	mux.HandleFunc("DELETE /notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		// net/http's ParseForm ignores DELETE bodies; the form package
		// reads them.
		v, err := webform.Read(w, r, webform.Limits{})
		if err != nil || v.String(session.CSRFField) != csrf {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h := hx2.ParseRequestHeaders(r.Header)
		fmt.Println(r.Method, r.URL.Path, "target:", h.Target, "trigger:", h.Trigger, "page:", h.CurrentURL)
		_ = web.Render(w, r, http.StatusOK, list.UL(attr.ID("notes"))())
	})
	mux.HandleFunc("POST /notes/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		web.Redirect(w, r, "/notes")
	})
	c := &webuitest.Client{Handler: mux, CSRF: csrf}
	p := c.Page(t, c.Get(t, "/notes"), http.StatusOK)
	del := p.Find(ct.ID("delete-1"))
	res := c.SubmitHTMX(t, p, del, nil, nil, webuitest.PageURL("/notes"))
	t.Must(c.Fragment(t, res, http.StatusOK, "div").Find(ct.ID("notes")).Count(1))

	fmt.Println(c.Submit(t, p, del, nil, nil, webuitest.HTMX()) == nil)
	t.Must(webuitest.Location(c.Submit(t, p, del, nil, nil), http.StatusSeeOther, ct.Exact("/notes")))
	// Output:
	// DELETE /notes/1 target: notes trigger: delete-1 page: https://app.example.test/notes
	// webuitest: cannot compute form data: htmx sends DELETE "/notes/1" for the hx-delete of the form, not the form's POST "/notes/1/delete"; send it with SubmitHTMX
	// true
}

// HTMXData computes the request that htmx sends for an element without
// sending it: the method and URL of its request attribute, the fields of
// its form with hx-vals and hx-params applied, and the target. A fragment
// runs under webui's htmx configuration, which sends the parameters of
// DELETE requests in the body.
func ExampleHTMXData() {
	view := list.LI(attr.ID("task-7"))(
		form.Form(form.Method(form.MethodPost), form.Action("/tasks/7/archive"))(
			session.CSRFInput("csrf-token"),
			input.Input(input.Type(input.Hidden), input.Name("version"), input.Value("3")),
			input.Input(input.Type(input.Hidden), input.Name("draft"), input.Value("x")),
			button.Button(button.Type(button.TypeButton), attr.ID("archive-7"), hx2.Delete("/tasks/7"),
				hx2.Target("closest li"), hx2.Vals(`{"reason": "done"}`), hx2.Params("not draft"))(text.Text("Archive")),
		),
	)
	frag := ct.View(view)
	r, err := webuitest.HTMXData(context.Background(), frag, frag.Find(ct.ID("archive-7")), nil, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(r.Method, r.Path, "in the URL:", r.URLParams, "target:", r.Target, "trigger:", r.Trigger)
	for _, f := range r.Fields {
		fmt.Printf("%s=%s\n", f.Name, f.Value)
	}
	// Output:
	// DELETE /tasks/7 in the URL: false target: task-7 trigger: archive-7
	// csrf=csrf-token
	// version=3
	// reason=done
}

// A fragment runs under the htmx configuration of the page that it is
// swapped into. Under webui's, the zero hx2.Config, an hx-delete sends the
// CSRF field of its form in the body; WithHTMXConfig checks the fragments
// of pages with DeleteURLParams, where the same markup puts the token in
// the URL.
func ExampleWithHTMXConfig() {
	t := webuitest.PrintT{}
	view := form.Form(form.Method(form.MethodPost), form.Action("/notes/1/delete"))(
		session.CSRFInput("csrf-token"),
		bs.BtnDanger(button.Type(button.TypeButton), hx2.Delete("/notes/1"))(text.Text("Delete")),
	)
	t.Must(webuitest.FragmentContract(ct.View(view), webuitest.Rules(webuitest.WithCSRF("csrf-token"))...))
	fmt.Println("valid under webui's configuration")
	leaky := webuitest.WithHTMXConfig(hx2.Config{DeleteURLParams: true})
	t.Must(webuitest.FragmentContract(ct.View(view), webuitest.Rules(webuitest.WithCSRF("csrf-token"), leaky)...))
	// Output:
	// valid under webui's configuration
	// assertion failed: document rule violated in chord view:
	//   noTokenLeaks(token): hx-delete at form[0]/button[1] sends the "csrf" field in the URL
}

// Stream serves an event stream through the client: Prepare attaches the
// session, as for the client's other requests.
func ExampleClient_Stream() {
	t := webuitest.PrintT{}
	sess := session.NewDetached("account-1", "csrf-token")
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[string](r.Context())
		if !ok {
			http.Error(w, "sign in", http.StatusUnauthorized)
			return
		}
		account, _ := s.Data()
		st := sse.New(w)
		_ = st.Send(r.Context(), "account", text.Strong()(text.Text(account)))
		_ = st.End(r.Context(), text.P()(text.Text("Signed out.")))
	})
	c := &webuitest.Client{
		Handler: h,
		Prepare: func(r *http.Request) *http.Request {
			return r.WithContext(session.WithSession(r.Context(), sess))
		},
	}
	st := c.Stream(t, "/live")
	fmt.Println(st.Expect(t, "account").Data)
	fmt.Println(st.Expect(t, sse.EventEnd).Data)
	st.ExpectEnd(t)
	// Output:
	// <strong>account-1</strong>
	// <p>Signed out.</p>
}
