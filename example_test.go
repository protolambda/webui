package webui_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// The quick start: an application with one page, behind the full
// middleware stack, served once through webuitest.
//
// A main function would serve the same handler with
//
//	ln, err := net.Listen("tcp", "127.0.0.1:8080")
//	...
//	err = server.Run(ctx, server.New(handler, server.Options{Logger: logger}), ln)
//
// configured with the origin that browsers use, such as
// mw.MustParseSite("https://tasks.example.com").
func Example() {
	// The embedded browser stack and theme, served from this origin with
	// integrity values.
	assets := theme.MustAssets(theme.Options{})

	// layout renders a page in the standard layout of the application.
	layout := func(title string, main elem.Node) elem.Node {
		return page.Document{
			Title:  title + " · Tasks",
			Assets: []*asset.Set{assets},
			Body: page.Layout{
				Navbar: page.Navbar{
					Brand: page.Brand{Label: "Tasks", Href: "/"},
					Items: []page.NavItem{{Label: "Today", Href: "/", Current: true}},
				},
				Header: page.Header{Title: text.Text(title)},
				Main:   main,
				Footer: text.Small()(text.Text("Tasks, an example.")),
			},
		}
	}

	// The error policy turns the errors handlers return into error pages
	// (or alerts in the page, for htmx requests).
	errs := web.Errors{Assets: []*asset.Set{assets}}

	mux := http.NewServeMux()
	mux.Handle(assets.Pattern(), assets)
	mux.Handle("GET /{$}", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Render(w, r, http.StatusOK, layout("Today", text.P()(text.Text("Nothing to do."))))
	}))
	mux.Handle("/", errs.NotFound())

	site := mw.MustParseSite(webuitest.BaseURL)
	logger := slog.New(slog.DiscardHandler) // the application's logger
	handler := mw.Chain(mux,
		mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux}),
		errs.Recover,
		mw.BodyDeadline(mw.DefaultBodyTimeout),
		mw.WriteDeadline(mw.DefaultWriteTimeout),
		mw.SecurityHeaders(mw.Headers{}),
		mw.HostCheck(site),
		mw.SameOrigin(mw.OriginPolicy{}),
		mw.MaxBody(1<<20),
	)

	// One request, as a browser sends it, and what came back.
	t := webuitest.PrintT{}
	res := webuitest.Serve(t, handler, webuitest.Get(t, "/"))
	fmt.Println(res.StatusCode())
	fmt.Println(res.Headers().Get("Content-Type"))
	csp, err := mw.ParseCSP(res.Headers().Get("Content-Security-Policy"))
	if err != nil {
		fmt.Println(err)
		return
	}
	scripts, _ := csp.Values(mw.ScriptSrc)
	fmt.Println("script-src", strings.Join(scripts, " "))

	// The page has the document head of its assets, passes every webui
	// rule (no inline script or style, integrity on every asset, named
	// controls, ...) and has the regions of the layout.
	rules := append(webuitest.Rules(), webuitest.LayoutRegions())
	t.Must(webuitest.PageContract(res.HTML(), webuitest.DefaultShell(assets), rules...))
	t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
	fmt.Println("page contract passes")

	// The assets are cached for good: their URLs change with their
	// content.
	res = webuitest.Serve(t, handler, webuitest.Get(t, assets.URL(theme.HTMXJS)))
	t.Must(webuitest.SecurityHeaders(res, webuitest.AssetPolicy()))
	fmt.Println(res.Headers().Get("Cache-Control"))

	// Unknown paths get the error page, with the same headers.
	res = webuitest.Serve(t, handler, webuitest.Get(t, "/nowhere"))
	fmt.Println(res.StatusCode())
	// Output:
	// 200
	// text/html; charset=utf-8
	// script-src 'self'
	// page contract passes
	// public, max-age=31536000, immutable
	// 404
}
