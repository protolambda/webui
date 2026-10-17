package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
)

// A form handler returns the errors of session.Forms.Read as they are: the
// policy answers a bad CSRF token with 403, a body over its limit with 413
// and a malformed form with 400, without echoing what was sent.
func ExampleErrors_form() {
	errs := web.Errors{}
	forms := session.Forms[string]{Limits: form.Limits{Body: 256}}
	h := errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		f, err := forms.Read(w, r)
		if err != nil {
			return err
		}
		defer f.Zero()
		if strings.TrimSpace(f.String("title")) == "" {
			return web.StatusError(http.StatusUnprocessableEntity, "Give the note a title.", nil)
		}
		web.Redirect(w, r, "/notes")
		return nil
	})

	sess := session.NewDetached("account", "csrf-token")
	post := func(body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(body))
		r.Header.Set("Content-Type", form.URLEncoded)
		r.Header.Set("HX-Request", "true")
		return r.WithContext(session.WithSession(r.Context(), sess))
	}
	ok := url.Values{session.CSRFField: {sess.CSRF()}, "title": {"Shopping"}}
	forged := url.Values{session.CSRFField: {"guess"}, "title": {"Shopping"}}
	empty := url.Values{session.CSRFField: {sess.CSRF()}, "title": {" "}}
	serve(h, post(ok.Encode()), "HX-Redirect")
	serve(h, post(forged.Encode()))
	serve(h, post(empty.Encode()))
	serve(h, post("title="+strings.Repeat("x", 300)))
	serve(h, post("title=%zz"))
	// Output:
	// 200
	// HX-Redirect: /notes
	// 403
	// <div class="alert alert-danger">The form expired or came from another site. Open the form again and retry.</div>
	// 422
	// <div class="alert alert-danger">Give the note a title.</div>
	// 413
	// <div class="alert alert-danger">The request is too large.</div>
	// 400
	// <div class="alert alert-danger">The form is not valid.</div>
}
