package webuitest_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/assertion"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/webuitest"
)

const crawlToken = "crawl-token"

// site is a small application for the crawler: pages with links, a form,
// htmx requests, an event stream and the theme's assets. extra adds
// content to the home page.
func site(extra ...elem.Node) *http.ServeMux {
	set := theme.MustAssets(theme.Options{})
	mux := http.NewServeMux()
	mux.Handle(set.Pattern(), set)
	page := func(title string, body ...elem.Node) elem.Node {
		main := section.Main(attr.ID("main"))(append([]elem.Node{section.H1()(text.Text(title))}, body...)...)
		return validDoc(set).with(func(d *docSpec) { d.body = []elem.Node{main} }).node()
	}
	link := func(href, label string) elem.Node { return text.A(text.Href(href))(text.Text(label)) }
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, page("Home", append([]elem.Node{
			link("/tasks", "Tasks"),
			link("/tasks#list", "Task list"),
			link("https://other.example/", "Elsewhere"),
			link("mailto:a@example.test", "Mail"),
			link("#main", "Top"),
			link("/old", "Old"),
		}, extra...)...))
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, page("Tasks",
			div.Div(attr.ID("list"), sse.Region("/tasks/live"))(),
			form.Form(form.Method(form.MethodPost), form.Action("/tasks"))(
				session.CSRFInput(crawlToken),
				button.Button(button.Type(button.TypeSubmit))(text.Text("Add")),
				button.Button(button.Type(button.TypeSubmit), button.Formaction("/tasks/clear"))(text.Text("Clear")),
			),
			button.Button(button.Type(button.TypeButton), hx2.Get("/tasks/count"), hx2.Target("#list"))(text.Text("Count")),
			link("/", "Home"),
		))
	})
	mux.HandleFunc("GET /old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tasks", http.StatusMovedPermanently)
	})
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("POST /tasks/clear", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /tasks/live", liveTasks)
	mux.HandleFunc("GET /tasks/count", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, text.P()(text.Text("2 tasks")))
	})
	mux.HandleFunc("GET /broken", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusInternalServerError)
	})
	return mux
}

func TestCrawl(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := site()
	c := &webuitest.Client{Handler: mux}
	contract := func(p *ct.Subject) assertion.Assertion {
		return webuitest.PageContract(p, webuitest.DefaultShell(), webuitest.Rules(webuitest.WithCSRF(crawlToken))...)
	}
	report := webuitest.Crawl(t, c, webuitest.CrawlOptions{
		Routes:    webuitest.MuxRoutes(mux),
		Contract:  contract,
		Assets:    true,
		Fragments: true,
	})
	t.Must(report)
	t.Must(be.DeepEqual([]string{"/", "/tasks"}, report.Pages))
	t.Must(report.Covers("/", "/tasks"))
	t.Must(report.Uses(http.MethodPost, "/tasks", "/tasks/clear"))
	t.Must(report.Uses(http.MethodGet, "/tasks/live", "/tasks/count"))
	kinds := map[webuitest.TargetKind]int{}
	for _, tg := range report.Targets {
		kinds[tg.Kind]++
	}
	t.Must(be.Equal(2, kinds[webuitest.TargetForm]))
	t.Must(be.Equal(1, kinds[webuitest.TargetStream]))
	t.Must(be.Equal(1, kinds[webuitest.TargetHTMX]))
	t.Must(be.Equal(1, kinds[webuitest.TargetRedirect]))
	t.Must(be.Equal(10, kinds[webuitest.TargetAsset]))
	t.Must(be.Equal("crawl(/)", report.String()))

	err := report.Covers("/notes").Check(t.Context())
	t.Must(be.ErrorContains(err, `the crawl did not reach "/notes"`))
	err = report.Uses(http.MethodDelete, "/tasks").Check(t.Context())
	t.Must(be.ErrorContains(err, `no crawled page has a DELETE target for "/tasks"`))
}

func TestCrawlProblems(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := site(
		text.A(text.Href("/missing"))(text.Text("Missing")),
		text.A(text.Href("/broken"))(text.Text("Broken")),
		text.A(text.Href("/tasks#nope"))(text.Text("No such part")),
		text.A(text.Href("/assets/webui/x/unknown.css"))(text.Text("Unknown asset")),
		form.Form(form.Method(form.MethodPost), form.Action("/nowhere"))(session.CSRFInput(crawlToken)),
		div.Div(hx2.Delete("/tasks/7"))(),
	)
	c := &webuitest.Client{Handler: mux}
	report := webuitest.Crawl(t, c, webuitest.CrawlOptions{
		Routes: webuitest.MuxRoutes(mux),
		Skip:   func(path string) bool { return strings.HasPrefix(path, "/assets/") },
	})
	err := report.Check(t.Context())
	t.Must(be.ErrorIs(err, webuitest.ErrCrawl))
	for _, want := range []string{
		"GET /missing (link on /) has no route",
		"GET /missing (from /) answered 404",
		"GET /broken (from /) answered 500",
		"POST /nowhere (form on /) has no route",
		"DELETE /tasks/7 (htmx on /) has no route",
		"link on / to /tasks#nope: the page has no element with that id",
	} {
		t.Must(be.ErrorContains(err, want))
	}
	t.Must(be.False(strings.Contains(err.Error(), "unknown.css")))
	t.Run("contract", func(t devtest.T) {
		c := &webuitest.Client{Handler: site()}
		report := webuitest.Crawl(t, c, webuitest.CrawlOptions{Start: []string{"/tasks"}})
		err := report.Check(t.Context())
		// The default contract does not know the page's CSRF token, but
		// requires the field; the page passes.
		t.Must(be.NoError(err))
		strict := webuitest.Crawl(t, c, webuitest.CrawlOptions{Start: []string{"/tasks"}, Contract: func(p *ct.Subject) assertion.Assertion {
			return webuitest.PageContract(p, webuitest.DefaultShell(), webuitest.Rules(webuitest.WithCSRF("other"))...)
		}})
		t.Must(be.ErrorContains(strict.Check(t.Context()), `page /tasks: document rule violated in GET /tasks html:`))
	})
	t.Run("bound", func(t devtest.T) {
		report := webuitest.Crawl(t, c, webuitest.CrawlOptions{MaxPages: 1})
		t.Must(be.ErrorContains(report.Check(t.Context()), "stopped after 1 pages"))
	})
	t.Run("start outside", func(t devtest.T) {
		report := webuitest.Crawl(t, c, webuitest.CrawlOptions{Start: []string{"https://other.example/"}})
		t.Must(be.ErrorContains(report.Check(t.Context()), `start page "https://other.example/" is not a path of the site`))
	})
}

func TestCrawlContractFor(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := site()
	c := &webuitest.Client{Handler: mux}
	fails := func(p *ct.Subject) assertion.Assertion { return be.True(false) }
	var pages, fragments []string
	report := webuitest.Crawl(t, c, webuitest.CrawlOptions{
		Contract: fails,
		ContractFor: func(u *url.URL) webuitest.Contract {
			pages = append(pages, u.String())
			defer func() { u.Path = "/changed" }() // The crawl keeps its own copy.
			if u.Path == "/tasks" {
				return fails
			}
			return func(p *ct.Subject) assertion.Assertion { return webuitest.PageContract(p, webuitest.DefaultShell()) }
		},
		Fragments:        true,
		FragmentContract: fails,
		FragmentContractFor: func(u *url.URL) webuitest.Contract {
			fragments = append(fragments, u.String())
			return func(f *ct.Subject) assertion.Assertion { return webuitest.FragmentContract(f) }
		},
	})
	// "/old" redirects to "/tasks", which ContractFor sees once, by the
	// URL of the page itself.
	t.Must(be.DeepEqual([]string{webuitest.BaseURL + "/", webuitest.BaseURL + "/tasks"}, pages))
	t.Must(be.DeepEqual([]string{webuitest.BaseURL + "/tasks/count"}, fragments))
	t.Must(be.DeepEqual([]string{"/", "/tasks"}, report.Pages))
	t.Must(be.Len(report.Problems, 1))
	t.Must(be.Substring(report.Problems[0], "page /tasks: "))

	t.Run("nil picks the default", func(t devtest.T) {
		var calls atomic.Int32
		none := func(*url.URL) webuitest.Contract { calls.Add(1); return nil }
		report := webuitest.Crawl(t, c, webuitest.CrawlOptions{
			Contract:            fails,
			ContractFor:         none,
			Fragments:           true,
			FragmentContract:    fails,
			FragmentContractFor: none,
		})
		t.Must(be.Equal(int32(3), calls.Load()))
		err := report.Check(t.Context())
		t.Must(be.ErrorContains(err, "page /: "))
		t.Must(be.ErrorContains(err, "page /tasks: "))
		t.Must(be.ErrorContains(err, "fragment /tasks/count (hx-get on /tasks): "))
		// Without Contract, nil is the default contract.
		report = webuitest.Crawl(t, c, webuitest.CrawlOptions{ContractFor: none, Fragments: true, FragmentContractFor: none})
		t.Must(report)
	})
	t.Run("query", func(t devtest.T) {
		var got []string
		mux := http.NewServeMux()
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			render(w, r, http.StatusOK, validDoc().with(func(d *docSpec) {
				d.body = []elem.Node{section.Main(attr.ID("main"))(text.A(text.Href("/?page=2#main"))(text.Text("Next")))}
			}).node())
		})
		webuitest.Crawl(t, &webuitest.Client{Handler: mux}, webuitest.CrawlOptions{ContractFor: func(u *url.URL) webuitest.Contract {
			got = append(got, u.Path+" "+u.RawQuery+" "+u.Fragment)
			return nil
		}})
		t.Must(be.DeepEqual([]string{"/  ", "/ page=2 "}, got))
	})
}

func TestCrawlRedirects(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("chain that mints new URLs", func(t devtest.T) {
		// A login page that itself requires a login wraps next= forever.
		var n atomic.Int32
		mux := http.NewServeMux()
		mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
			n.Add(1)
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		})
		report := webuitest.Crawl(t, &webuitest.Client{Handler: mux}, webuitest.CrawlOptions{Start: []string{"/login"}, MaxPages: 5})
		t.Must(be.ErrorContains(report.Check(t.Context()), "more than 20 redirects in a row"))
		t.Must(be.Equal(int32(21), n.Load()))
		t.Must(be.Len(report.Problems, 1))
		t.Must(be.Len(report.Pages, 0))
	})
	t.Run("loop", func(t devtest.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /a", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/b", http.StatusFound) })
		mux.HandleFunc("GET /b", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/a", http.StatusFound) })
		mux.HandleFunc("GET /self", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/self", http.StatusFound) })
		report := webuitest.Crawl(t, &webuitest.Client{Handler: mux}, webuitest.CrawlOptions{Start: []string{"/a", "/self"}})
		err := report.Check(t.Context())
		t.Must(be.ErrorContains(err, "GET /b (from /a): the redirects loop back to /a"))
		t.Must(be.ErrorContains(err, "GET /self (from the start): the redirects loop back to /self"))
	})
	t.Run("short chains are followed", func(t devtest.T) {
		mux := site()
		mux.HandleFunc("GET /older", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/old", http.StatusFound) })
		report := webuitest.Crawl(t, &webuitest.Client{Handler: mux}, webuitest.CrawlOptions{Start: []string{"/older"}})
		t.Must(report)
		t.Must(report.Covers("/tasks", "/"))
	})
}

func TestCrawlAssets(gt *testing.T) {
	t := devtest.ParallelT(gt)
	set := theme.MustAssets(theme.Options{})
	mux := http.NewServeMux()
	css := set.URL(theme.ThemeCSS)
	mux.HandleFunc("GET "+css, func(w http.ResponseWriter, r *http.Request) {
		// A changed file under the URL of the old one.
		w.Header().Set("Content-Type", "text/css")
		_, _ = io.WriteString(w, "p{}")
	})
	mux.Handle(set.Pattern(), set)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, validDoc(set).node())
	})
	report := webuitest.Crawl(t, &webuitest.Client{Handler: mux}, webuitest.CrawlOptions{Assets: true})
	err := report.Check(t.Context())
	t.Must(be.ErrorContains(err, "asset "+css+" (on /): no X-Content-Type-Options: nosniff; the served bytes do not match the integrity value of the page"))
	t.Must(be.Len(report.Problems, 1))
}

// routerFunc is the mw.Router of a function.
type routerFunc func(r *http.Request) (http.Handler, string)

func (f routerFunc) Handler(r *http.Request) (http.Handler, string) { return f(r) }

func TestMuxRoutes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks/{id}", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("/notes/", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("POST /tasks", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {})
	routes := webuitest.MuxRoutes(mux)
	t.Must(be.True(routes(http.MethodGet, "/tasks/7?x=1")))
	t.Must(be.True(routes(http.MethodHead, "/tasks/7")))
	t.Must(be.True(routes(http.MethodPost, "/tasks")))
	t.Must(be.True(routes(http.MethodGet, "/notes/a")))
	t.Must(be.True(routes(http.MethodGet, "/")))
	t.Must(be.False(routes(http.MethodGet, "/notes")))
	t.Must(be.False(routes(http.MethodGet, "/elsewhere")))
	t.Must(be.False(routes(http.MethodGet, "/tasks/../x")))
	t.Must(be.False(routes(http.MethodGet, "%zz")))
	only := http.NewServeMux()
	only.HandleFunc("GET /a", func(http.ResponseWriter, *http.Request) {})
	t.Must(be.False(webuitest.MuxRoutes(only)(http.MethodPost, "/a")))

	// A mux that an outer mux serves at its catch-all: the outer mux
	// routes all of its requests to "/", Mounted to the mux's patterns.
	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(http.ResponseWriter, *http.Request) {})
	outer.Handle("/", mux)
	t.Must(be.False(webuitest.MuxRoutes(outer)(http.MethodGet, "/tasks/7")))
	mounted := webuitest.MuxRoutes(mw.Mounted(outer, "/", mux))
	t.Must(be.True(mounted(http.MethodGet, "/tasks/7")))
	t.Must(be.True(mounted(http.MethodGet, "/healthz")))
	t.Must(be.True(mounted(http.MethodGet, "/")))
	t.Must(be.False(mounted(http.MethodGet, "/notes")))
	t.Must(be.False(mounted(http.MethodGet, "/elsewhere")))
	// Any router: here one that sends posts to another mux.
	byMethod := routerFunc(func(r *http.Request) (http.Handler, string) {
		if r.Method == http.MethodPost {
			return only.Handler(r)
		}
		return mux.Handler(r)
	})
	t.Must(be.True(webuitest.MuxRoutes(byMethod)(http.MethodGet, "/tasks/7")))
	t.Must(be.False(webuitest.MuxRoutes(byMethod)(http.MethodPost, "/tasks")))
	t.Must(be.Equal("redirect", webuitest.TargetRedirect.String()))
	t.Must(be.Equal("TargetKind(99)", webuitest.TargetKind(99).String()))
}
