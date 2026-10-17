package theme_test

import (
	"fmt"
	"net/http"
	"strings"

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
