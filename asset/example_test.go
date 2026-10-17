package asset_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing/fstest"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/html/meta"

	"github.com/protolambda/webui/asset"
)

func exampleSet() *asset.Set {
	// Usually an embed.FS: //go:embed app.css app.js
	files := fstest.MapFS{
		"app.css": {Data: []byte("main { padding: 1rem; }\n")},
		"app.js":  {Data: []byte("\"use strict\";\n")},
	}
	return asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     files,
		Files:  []asset.File{{Name: "app.css"}, {Name: "app.js"}},
	}))
}

func ExampleSet_Stylesheets() {
	set := exampleSet()
	head := meta.Head()(set.Stylesheets(), set.Scripts())
	if err := core.Render(context.Background(), head, os.Stdout, core.WithIndent()); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <head>
	//   <link rel="stylesheet" href="/assets/app/584b5f123217279a/app.css" integrity="sha384-KvyGCvFPE4H1pQYnn3wkiBzN/uByDyCl8P44/BC/Oz+BqLNWSSxkdBdKM8Ln+mqW" crossorigin="anonymous"/>
	//   <script src="/assets/app/584b5f123217279a/app.js" defer integrity="sha384-xkfRlW1o/12WPjFaNyRGWMTu+oSgZhrVjoOW5J3gH5+BR9iBauzFwFUOCOKARbI6" crossorigin="anonymous"></script>
	// </head>
}

func ExampleSet_ServeHTTP() {
	set := exampleSet()
	mux := http.NewServeMux()
	mux.Handle(set.Pattern(), set)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, set.URL("app.css"), nil))
	fmt.Println(rec.Code, rec.Header().Get("Content-Type"))
	fmt.Println(rec.Header().Get("Cache-Control"))
	fmt.Print(rec.Body.String())
	// Output:
	// 200 text/css; charset=utf-8
	// public, max-age=31536000, immutable
	// main { padding: 1rem; }
}

func ExampleSet_Files() {
	for info := range exampleSet().Files() {
		fmt.Println(info.Kind, info.Name, info.Size, info.ContentType)
	}
	// Output:
	// stylesheet app.css 24 text/css; charset=utf-8
	// script app.js 14 text/javascript; charset=utf-8
}
