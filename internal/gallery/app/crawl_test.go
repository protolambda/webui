package app_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/protolambda/devtest"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/webuitest"
)

// pages are the pages of the gallery that every visitor reaches from the
// start page.
var pages = []string{
	"/", "/components", "/components/controls", "/components/content", "/components/feedback",
	"/forms", "/dialogs", "/disclosure", "/htmx", "/live", "/bootstrap-js",
}

// withPolicies adds to o the security headers that every page and asset
// of the gallery carries at an https site, and that no asset sets a
// cookie.
func withPolicies(o webuitest.CrawlOptions) webuitest.CrawlOptions {
	page, asset := webuitest.PolicyFor(httpsHeaders), webuitest.AssetPolicyFor(httpsHeaders)
	o.PagePolicy = func(*url.URL) *webuitest.Policy { return &page }
	o.AssetPolicy = func(*url.URL) *webuitest.Policy { return &asset }
	o.NoAssetCookies = true
	return o
}

// TestCrawl follows every link of the gallery, signed out and signed in,
// and checks every page against the contract of its URL, every target
// against the routes, the security headers of every page and asset, every
// asset against its integrity value, and every hx-get fragment against
// the fragment contract.
func TestCrawl(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("signed out", func(t devtest.T) {
		g := newGallery(t, app.Options{})
		report := webuitest.Crawl(t, g.anonymous(), withPolicies(webuitest.CrawlOptions{
			Routes:           webuitest.MuxRoutes(g.Mux()),
			Contract:         contract(""),
			ContractFor:      contractFor(""),
			FragmentContract: fragmentContract(""),
			Assets:           true,
			Fragments:        true,
		}))
		t.Must(report)
		t.Must(report.Covers(append(pages, "/login?next=%2Fnotes", "/components/content?page=3")...))
		t.Must(report.Uses(http.MethodPost, "/forms", "/dialogs/rename", "/dialogs/reset", "/htmx/tasks", "/htmx/fail", "/htmx/plain-error", "/login"))
		t.Must(report.Uses(http.MethodGet, "/live/clock", "/notes/live", "/htmx/poll", "/htmx/search", "/components/toast"))
	})
	t.Run("signed in", func(t devtest.T) {
		g := newGallery(t, app.Options{})
		c := g.signedIn(newAccount("Ada"))
		report := webuitest.Crawl(t, c, withPolicies(webuitest.CrawlOptions{
			Routes:           webuitest.MuxRoutes(g.Mux()),
			Contract:         contract(testCSRF),
			ContractFor:      contractFor(testCSRF),
			FragmentContract: fragmentContract(testCSRF),
			Assets:           true,
			Fragments:        true,
		}))
		t.Must(report)
		t.Must(report.Covers(append(pages, "/notes")...))
		t.Must(report.Uses(http.MethodPost, "/notes", "/logout", "/htmx/tasks/1/delete"))
	})
}
