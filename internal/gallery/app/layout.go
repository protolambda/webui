package app

import (
	"net/http"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/ba"
	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	webform "github.com/protolambda/webui/form"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
)

// siteName names the gallery in the navbar and in document titles.
const siteName = "webui gallery"

// titleSuffix ends the document title of every page but the start page,
// whose title is siteName alone.
const titleSuffix = " · " + siteName

// formLimits bound every form of the gallery: its fields are short texts.
var formLimits = webform.Limits{Body: 32 << 10, Field: 4 << 10, Fields: 32}

// notices are the outcomes that actions report after their redirect.
var notices = web.Notices{
	"task-saved":   {Message: "Task saved."},
	"task-added":   {Message: "Task added."},
	"task-deleted": {Message: "Task deleted.", Variant: bs.VariantInfo},
	"renamed":      {Message: "Project renamed."},
	"reset":        {Message: "Project name reset.", Variant: bs.VariantInfo},
	"note-added":   {Message: "Note added."},
	"signed-in":    {Message: "Signed in."},
	"signed-out":   {Message: "Signed out.", Variant: bs.VariantInfo},
}

// gallerySection is a part of the gallery in the navbar and on the start
// page.
type gallerySection struct {
	Path, Label, Blurb string
	Icon               bi.Icon
}

var gallerySections = []gallerySection{
	{"/components", "Components", "Buttons, badges, alerts, cards, lists, tables, pagination, spinners, progress, icons and toasts.", bi.Grid3x3Gap},
	{"/forms", "Forms", "Fields with server-side validation, CSRF tokens and Post/Redirect/Get, with and without htmx.", bi.UiChecks},
	{"/dialogs", "Dialogs", "Modal dialogs without script, one of them holding a form.", bi.WindowStack},
	{"/disclosure", "Disclosure", "Accordions and dropdown menus built on the details element.", bi.ChevronExpand},
	{"/htmx", "htmx", "Fragment and out-of-band swaps, error answers, a confirmed delete, search and polling.", bi.ArrowRepeat},
	{"/live", "Live", "Server-sent event streams that end with an end event.", bi.Broadcast},
	{"/notes", "Notes", "Private notes behind a login: sessions and an authentication guard.", bi.Lock},
	{"/bootstrap-js", "Bootstrap JS", "The opt-in Bootstrap script and the components that need it.", bi.CodeSquare},
}

// view is the content of a gallery page, which page renders into the
// gallery's document and layout.
type view struct {
	Title string
	Icon  bi.Icon
	// Parents are the breadcrumb steps between the start page and this
	// page, such as the overview of a section.
	Parents []page.Crumb
	Actions []elem.Node
	Sidebar elem.Node
	Alerts  []elem.Node
	Main    elem.Node
	Dialogs []elem.Node
	// BootstrapJS loads the asset set with Bootstrap's script.
	BootstrapJS bool
}

// page renders v as a full page for r: the document with the gallery's
// assets, and the layout with the navbar, a header with the breadcrumb,
// the notice of the request among the alerts, and the footer.
func (a *App) page(r *http.Request, v view) elem.Node {
	set := a.assets
	if v.BootstrapJS {
		set = a.jsAssets
	}
	var crumbs []page.Crumb
	if r.URL.Path != "/" {
		crumbs = append(append([]page.Crumb{{Label: "Gallery", Href: "/"}}, v.Parents...), page.Crumb{Label: v.Title})
	}
	title := v.Title + titleSuffix
	if v.Title == siteName {
		title = siteName
	}
	return page.Document{
		Title:  title,
		Assets: []*asset.Set{set},
		Body: page.Layout{
			Navbar:  a.navbar(r),
			Sidebar: v.Sidebar,
			Header:  page.Header{Title: text.Text(v.Title), Icon: v.Icon, Crumbs: crumbs, Actions: v.Actions},
			Alerts:  append([]elem.Node{notices.Alert(r)}, v.Alerts...),
			Main:    v.Main,
			Footer:  footer(),
			Dialogs: v.Dialogs,
		},
	}
}

// errorPage is the Document hook of the gallery's error policy: the error
// in the gallery's layout.
func (a *App) errorPage(r *http.Request, v web.ErrorView) elem.Node {
	l := v.Layout()
	l.Navbar = a.navbar(r)
	l.Footer = footer()
	return page.Document{Title: v.Title + titleSuffix, Assets: []*asset.Set{a.assets}, Body: l}
}

// navbar renders the navbar for r: the sections, and at the end the login
// link or the logout form. The link of the current page is Current; on a
// page inside a section (such as /components/content), the section's link
// is CurrentSection.
func (a *App) navbar(r *http.Request) elem.Node {
	items := make([]page.NavItem, 0, len(gallerySections))
	for _, s := range gallerySections {
		items = append(items, page.NavItem{
			Label: s.Label, Href: s.Path, Icon: s.Icon,
			Current:        r.URL.Path == s.Path,
			CurrentSection: strings.HasPrefix(r.URL.Path, s.Path+"/"),
		})
	}
	return page.Navbar{
		Label: "Gallery sections",
		Brand: page.Brand{Label: siteName, Href: "/", Icon: bi.Grid, Current: r.URL.Path == "/"},
		Items: items,
		End:   a.navbarEnd(r),
	}
}

func (a *App) navbarEnd(r *http.Request) []elem.Node {
	s, ok := session.FromContext[Account](r.Context())
	if !ok {
		return []elem.Node{
			text.A(text.Href("/login"), attr.Class("btn btn-outline-primary"), ba.BtnSM())(bi.BoxArrowInRight, text.Text(" Log in")),
		}
	}
	acc, _ := s.Data()
	return []elem.Node{
		text.Span(attr.Class("navbar-text small"))(text.Text("Signed in as "), text.Strong()(text.Text(acc.Name))),
		postForm("/logout", s.CSRF())(
			bs.BtnOutlineSecondary(button.Type(button.TypeSubmit), ba.BtnSM())(bi.BoxArrowRight, text.Text(" Log out")),
		),
	}
}

func footer() elem.Node {
	return text.P(attr.Class("small text-body-secondary mb-0"))(
		text.Text("webui gallery. Every page works without application JavaScript; the demo data lives in memory and resets when the server restarts."),
	)
}

// postForm renders a POST form to action that carries the CSRF token, with
// more attributes and the children after the token.
func postForm(action, token string, attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		base := []attr.Node{form.Method(form.MethodPost), form.Action(action)}
		return form.Form(append(base, attrs...)...)(append([]elem.Node{session.CSRFInput(token)}, children...)...)
	}
}

// submit renders a submit button of a variant.
func submit(v bs.Variant, label string, attrs ...attr.Node) elem.Node {
	return bs.BtnOf(v, append([]attr.Node{button.Type(button.TypeSubmit)}, attrs...)...)(text.Text(label))
}

// link renders a link.
func link(href, label string, attrs ...attr.Node) elem.Node {
	return text.A(append([]attr.Node{text.Href(href)}, attrs...)...)(text.Text(label))
}

// demo renders a section of a demo page, named by its h2.
func demo(id, title string, content ...elem.Node) elem.Node {
	heading := id + "-title"
	return section.Section(attr.ID(id), aria.Labelledby(heading), attr.Class("mb-5"))(
		append([]elem.Node{section.H2(attr.ID(heading), attr.Class("h5 border-bottom pb-2 mb-3"))(text.Text(title))}, content...)...,
	)
}

// para renders a paragraph of text.
func para(s string, attrs ...attr.Node) elem.Node {
	return text.P(attrs...)(text.Text(s))
}

// errText renders a validation message, or nil for none, for the Error
// fields of bs form components.
func errText(msg string) elem.Node {
	if msg == "" {
		return nil
	}
	return text.Text(msg)
}

// formToken returns the CSRF token for the forms of the whole page that
// answers r: the session's token when the visitor is signed in, and
// otherwise a pre-authentication token (session.PreAuth.Issue), whose
// cookie it sets or renews on w. Fragments use fragmentToken.
func (a *App) formToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if s, ok := session.FromContext[Account](r.Context()); ok {
		return s.CSRF(), nil
	}
	return a.preauth.Issue(w, r)
}

// fragmentToken returns the CSRF token for the forms of an htmx fragment
// that updates a page of r: the session's token when the visitor is
// signed in, and otherwise the token of the pre-authentication cookie that
// the page's forms carry (session.PreAuth.Token), which a fragment must not
// renew under them. Only when r has no valid cookie (none, or an expired
// one, which the page's forms fail with as well) does it issue a new
// cookie on w, and report renewed: the fragment must then renew every form
// of the page out of band, as the browser's new cookie refuses their
// tokens.
func (a *App) fragmentToken(w http.ResponseWriter, r *http.Request) (token string, renewed bool, err error) {
	if s, ok := session.FromContext[Account](r.Context()); ok {
		return s.CSRF(), false, nil
	}
	if token, ok := a.preauth.Token(r); ok {
		return token, false, nil
	}
	return a.preauth.IssueRenewed(w, r)
}

// choosePage answers r like web.Choose, with a fragment that holds no
// form, or the page that full renders for a form token. It asks for the
// token only for the page, so that a fragment, such as the answer to a
// poll, never sets a pre-authentication cookie.
func (a *App) choosePage(w http.ResponseWriter, r *http.Request, fragment elem.Node, full func(token string) elem.Node) error {
	if web.WantsFragment(r) {
		return web.Choose(w, r, http.StatusOK, fragment, nil)
	}
	token, err := a.formToken(w, r)
	if err != nil {
		return err
	}
	return web.Choose(w, r, http.StatusOK, fragment, full(token))
}

// readForm reads the form of r within formLimits and checks its CSRF token,
// as formToken issued it. The caller owns the values and zeroes them.
func (a *App) readForm(w http.ResponseWriter, r *http.Request) (*webform.Values, error) {
	f, err := webform.Read(w, r, formLimits)
	if err != nil {
		return nil, err
	}
	token := f.String(session.CSRFField)
	if s, ok := session.FromContext[Account](r.Context()); ok {
		err = s.CheckCSRF(token)
	} else {
		err = a.preauth.Verify(r, token)
	}
	if err != nil {
		f.Zero()
		return nil, err
	}
	return f, nil
}

// capitalize returns s with an upper-case first ASCII letter.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
