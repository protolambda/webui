package app_test

import (
	"net/http"
	"net/url"
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

// The two asset sets of the gallery, and their shells.
var (
	plainAssets = theme.MustAssets(theme.Options{})
	jsAssets    = theme.MustAssets(theme.Options{Prefix: app.BootstrapJSPrefix, BootstrapJS: true})
	plainShell  = webuitest.DefaultShell(plainAssets)
	jsShell     = webuitest.DefaultShell(jsAssets)
)

// contract is the page contract of the gallery: the default shell, every
// webui rule (with csrf, when not empty, as the token of every post form)
// and the regions of page.Layout. Pages may not use Bootstrap's script;
// the one that does has its own contract (contractFor).
func contract(csrf string) webuitest.Contract {
	return pageContract(plainShell, webuitest.WithCSRF(csrf))
}

// contractFor picks the contract of a crawled page by its URL: the
// Bootstrap JS page has the shell of the second asset set and may use
// Bootstrap's script; every other page has contract(csrf).
func contractFor(csrf string) func(u *url.URL) webuitest.Contract {
	js := pageContract(jsShell, webuitest.WithCSRF(csrf), webuitest.WithBootstrapJS())
	return func(u *url.URL) webuitest.Contract {
		if u.Path == "/bootstrap-js" {
			return js
		}
		return nil
	}
}

// pageContract is the contract of pages with shell, the webui rules with
// opts and the regions of page.Layout.
func pageContract(shell webuitest.Shell, opts ...webuitest.RuleOption) webuitest.Contract {
	return func(p *ct.Subject) assertion.Assertion {
		return webuitest.PageContract(p, shell, append(webuitest.Rules(opts...), webuitest.LayoutRegions())...)
	}
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
