package app_test

import (
	"net/http"
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

// TestCrawl follows every link of the gallery, signed out and signed in,
// and checks every page against the contract, every target against the
// routes, every asset against its integrity value, and every hx-get
// fragment against the fragment contract.
func TestCrawl(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("signed out", func(t devtest.T) {
		g := newGallery(t, app.Options{})
		report := webuitest.Crawl(t, g.anonymous(), webuitest.CrawlOptions{
			Routes:           webuitest.MuxRoutes(g.Mux()),
			Contract:         contract(""),
			FragmentContract: fragmentContract(""),
			Assets:           true,
			Fragments:        true,
		})
		t.Must(report)
		t.Must(report.Covers(append(pages, "/login?next=%2Fnotes", "/components/content?page=3")...))
		t.Must(report.Uses(http.MethodPost, "/forms", "/dialogs/rename", "/dialogs/reset", "/htmx/tasks", "/htmx/fail", "/login"))
		t.Must(report.Uses(http.MethodGet, "/live/clock", "/notes/live", "/htmx/poll", "/htmx/search", "/components/toast"))
	})
	t.Run("signed in", func(t devtest.T) {
		g := newGallery(t, app.Options{})
		c := g.signedIn(newAccount("Ada"))
		report := webuitest.Crawl(t, c, webuitest.CrawlOptions{
			Routes:           webuitest.MuxRoutes(g.Mux()),
			Contract:         contract(testCSRF),
			FragmentContract: fragmentContract(testCSRF),
			Assets:           true,
			Fragments:        true,
		})
		t.Must(report)
		t.Must(report.Covers(append(pages, "/notes")...))
		t.Must(report.Uses(http.MethodPost, "/notes", "/logout", "/htmx/tasks/1/delete"))
	})
}
