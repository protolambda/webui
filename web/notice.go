package web

import (
	"fmt"
	"net/http"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/page"
)

// NoticeParam is the query parameter that carries the code of a [Notice].
const NoticeParam = "notice"

// MaxNoticeCode is the longest notice code, in bytes.
const MaxNoticeCode = 32

// Notice is a fixed message that reports the outcome of an action on the
// page the action redirects to, such as "Task saved.".
type Notice struct {
	// Message is the text of the notice. It is fixed by the application and
	// never contains request data.
	Message string
	// Variant is the color of the notice. Empty means bs.VariantSuccess.
	Variant bs.Variant
}

// Alert renders the notice as an alert with role="status" ([page.Notice]).
func (n Notice) Alert() elem.Node {
	return page.Notice(n.Variant, text.Text(n.Message))
}

// Notices is a registry of notices by code, for Post/Redirect/Get: after an
// action, the handler redirects to a page with the code of its outcome in
// the [NoticeParam] query parameter ([Notices.Target], [Notices.Redirect]),
// and the page shows the notice of that code ([Notices.Alert]).
//
// Only the short code travels in the URL, and only registered messages are
// ever shown: a crafted link cannot put its own text on the page, and an
// unknown code shows nothing. The notice survives a reload, and needs no
// session state.
//
// Codes are 1 to [MaxNoticeCode] bytes of lower-case ASCII letters, digits,
// '-' and '_'. Build the registry once, e.g. as a package variable; it is
// safe for concurrent use as long as it is not modified.
type Notices map[string]Notice

// Target returns target with the notice parameter set to code (replacing
// an earlier one), as [WithParam] does. Pass target as an absolute path,
// as for [Redirect]: a relative one, such as "#new", gives a relative
// result ("?notice=saved#new"), which plain and htmx requests resolve
// against different URLs. It panics if code is not a valid code of the
// registry: codes are constants of the application, so an unknown one is
// a programming error.
func (n Notices) Target(target, code string) string {
	if _, ok := n[code]; !ok || !validCode(code) {
		panic(fmt.Sprintf("web: unknown notice code %q", code))
	}
	return WithParam(target, NoticeParam, code)
}

// Redirect redirects to target, an absolute path, with the notice code,
// with [Redirect]. It panics like [Notices.Target].
func (n Notices) Redirect(w http.ResponseWriter, r *http.Request, target, code string) {
	Redirect(w, r, n.Target(target, code))
}

// Of returns the notice whose code the request's URL carries, and whether
// there is one. A missing, malformed or unknown code gives false.
func (n Notices) Of(r *http.Request) (Notice, bool) {
	code := r.URL.Query().Get(NoticeParam)
	if !validCode(code) {
		return Notice{}, false
	}
	notice, ok := n[code]
	return notice, ok
}

// Alert renders the notice of the request ([Notices.Of]) as an alert, or
// nothing when it has none: put it among the alerts of the page's layout.
func (n Notices) Alert(r *http.Request) elem.Node {
	if notice, ok := n.Of(r); ok {
		return notice.Alert()
	}
	return elem.Noop()
}

// validCode reports whether code has the shape of a notice code.
func validCode(code string) bool {
	if code == "" || len(code) > MaxNoticeCode {
		return false
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}
