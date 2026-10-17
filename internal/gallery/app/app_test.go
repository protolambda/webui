package app_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/assertion"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/webuitest"
)

// testCSRF is the CSRF token of the detached sessions of signed-in clients.
const testCSRF = "test-csrf-token-0123456789"

// gallery is a gallery under test, served at webuitest.BaseURL unless the
// options say otherwise, with its log captured.
type gallery struct {
	*app.App
	logs *webuitest.Logs
}

func newGallery(t devtest.T, opts app.Options) gallery {
	t.Helper()
	logs := webuitest.NewLogs()
	if opts.Sites == nil {
		opts.Sites = []mw.Site{mw.MustParseSite(webuitest.BaseURL)}
	}
	opts.Logger = logs.Logger()
	a, err := app.New(opts)
	t.Must(be.NoError(err))
	return gallery{App: a, logs: logs}
}

// anonymous returns a client of the gallery without a session. Its pages
// carry pre-authentication tokens, a new one per page since the client
// keeps no cookies, so its contract accepts any token.
func (g gallery) anonymous() *webuitest.Client {
	return &webuitest.Client{Handler: g, Contract: contract(""), FragmentContract: fragmentContract("")}
}

// signedIn returns a client whose requests carry s, a detached session
// with the CSRF token testCSRF, instead of a cookie.
func (g gallery) signedIn(s *session.Session[app.Account]) *webuitest.Client {
	return &webuitest.Client{
		Handler: g,
		Prepare: func(r *http.Request) *http.Request {
			return r.WithContext(session.WithSession(r.Context(), s))
		},
		CSRF:             testCSRF,
		Contract:         contract(testCSRF),
		FragmentContract: fragmentContract(testCSRF),
	}
}

// newAccount returns a detached session for signedIn.
func newAccount(name string) *session.Session[app.Account] {
	return session.NewDetached(app.Account{Name: name}, testCSRF)
}

// Shells of the two asset sets of the gallery.
var (
	plainShell = webuitest.DefaultShell(theme.MustAssets(theme.Options{}))
	jsShell    = webuitest.DefaultShell(theme.MustAssets(theme.Options{Prefix: app.BootstrapJSPrefix, BootstrapJS: true}))
)

// bootstrapJSTitle is the document title of the only page that loads
// Bootstrap's script.
const bootstrapJSTitle = "Bootstrap JS (opt-in) · webui gallery"

// contract is the page contract of the gallery: the shell of the page's
// asset set, every webui rule (with csrf, when not empty, as the token of
// every post form) and the regions of page.Layout. The Bootstrap JS page,
// recognized by its title, has the shell of the second asset set and may
// use Bootstrap's script; every other page has the default shell, without
// Bootstrap's script, and may not.
func contract(csrf string) webuitest.Contract {
	return func(p *ct.Subject) assertion.Assertion {
		return pageAssertion{page: p, csrf: csrf}
	}
}

type pageAssertion struct {
	page *ct.Subject
	csrf string
}

func (a pageAssertion) String() string { return "gallery page contract of " + a.page.String() }

func (a pageAssertion) Check(ctx context.Context) error {
	title, err := a.page.Find(ct.Tag("title")).Text(ctx)
	if err != nil {
		return fmt.Errorf("page title: %w", err)
	}
	shell, opts := plainShell, []webuitest.RuleOption{webuitest.WithCSRF(a.csrf)}
	if title == bootstrapJSTitle {
		shell, opts = jsShell, append(opts, webuitest.WithBootstrapJS())
	}
	return webuitest.PageContract(a.page, shell, append(webuitest.Rules(opts...), webuitest.LayoutRegions())...).Check(ctx)
}

// fragmentContract is the contract of the gallery's htmx fragments.
func fragmentContract(csrf string) webuitest.Contract {
	return func(f *ct.Subject) assertion.Assertion {
		return webuitest.FragmentContract(f, webuitest.Rules(webuitest.WithCSRF(csrf))...)
	}
}

func TestNewRefusesOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	https := mw.MustParseSite("https://gallery.example.com")
	loopback := mw.MustParseSite("http://127.0.0.1:8080")
	for name, opts := range map[string]app.Options{
		"no site":                   {},
		"zero site":                 {Sites: []mw.Site{{}}},
		"insecure cookies on https": {Sites: []mw.Site{loopback, https}, InsecureCookies: true},
		"negative lifetime":         {Sites: []mw.Site{https}, LiveLifetime: -time.Second},
	} {
		_, err := app.New(opts)
		t.Mustf(be.ErrorIs(err, app.ErrOptions), "%s", name)
	}
	_, err := app.New(app.Options{Sites: []mw.Site{loopback}, InsecureCookies: true})
	t.Must(be.NoError(err))
}
