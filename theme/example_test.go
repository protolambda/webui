package theme_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing/fstest"

	"github.com/protolambda/chord/core"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/theme"
)

func ExampleAssets() {
	assets := theme.MustAssets(theme.Options{WithoutIcons: true})

	mux := http.NewServeMux()
	mux.Handle(assets.Pattern(), assets)

	for info := range assets.Files() {
		fmt.Println(info.Kind, info.Name)
	}
	// Output:
	// stylesheet bootstrap-5.3.8.min.css
	// stylesheet webui.css
	// script htmx-2.0.10.min.js
	// script htmx-ext-sse-2.2.4.min.js
}

func ExampleOptions() {
	assets := theme.MustAssets(theme.Options{Prefix: "/static/ui/", WithoutSSE: true, BootstrapJS: true})
	for info := range assets.Files() {
		// URLs carry the set's version: /static/ui/<version>/<name>.
		fmt.Println(info.Kind, strings.TrimPrefix(info.URL, assets.Prefix()+assets.Version()))
	}
	// Output:
	// stylesheet /bootstrap-5.3.8.min.css
	// stylesheet /bootstrap-icons-1.13.1.min.css
	// font /fonts/bootstrap-icons.woff2
	// stylesheet /webui.css
	// script /htmx-2.0.10.min.js
	// script /bootstrap-5.3.8.bundle.min.js
}

// A page tunes the theme with a stylesheet of its own that overrides the
// theme's --webui- tokens (see the package documentation). Its set comes
// after the theme's, so that the stylesheet is linked after webui.css, as
// page.Document links the stylesheets of its assets in order.
func ExampleAssets_tuning() {
	ui := theme.MustAssets(theme.Options{WithoutIcons: true})
	// Usually an embed.FS.
	files := fstest.MapFS{
		"tuning.css": {Data: []byte(`[data-bs-theme="dark"] { --webui-focus-color: var(--bs-warning); }` + "\n")},
	}
	app := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     files,
		Files:  []asset.File{{Name: "tuning.css"}},
	}))

	for _, set := range []*asset.Set{ui, app} {
		for info := range set.Files() {
			if info.Kind == asset.KindStylesheet {
				fmt.Println(info.Name)
			}
		}
	}
	if err := core.Render(context.Background(), app.Stylesheets(), os.Stdout); err != nil {
		fmt.Println(err)
	}
	// Output:
	// bootstrap-5.3.8.min.css
	// webui.css
	// tuning.css
	// <link rel="stylesheet" href="/assets/app/fff63c2b5ee5b400/tuning.css" integrity="sha384-9JtPJFqEUPfd+YAzURPzB7siupmPygEfrY7h8ceo2mWE7ApQUOWNhNFJmRwCLiJU" crossorigin="anonymous"/>
}
