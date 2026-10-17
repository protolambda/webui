package asset_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing/fstest"

	"github.com/protolambda/chord/core"

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

func ExampleNew() {
	// A third-party file, pinned with the integrity value its project
	// publishes. Usually an embed.FS: //go:embed vendor
	files := fstest.MapFS{"vendor/lib.js": {Data: []byte("\"use strict\";\n")}}
	opts := asset.Options{
		Prefix: "/assets/vendor/",
		FS:     files,
		Files: []asset.File{{
			Name:      "vendor/lib.js",
			Integrity: "sha384-xkfRlW1o/12WPjFaNyRGWMTu+oSgZhrVjoOW5J3gH5+BR9iBauzFwFUOCOKARbI6",
		}},
	}
	set, err := asset.New(opts)
	fmt.Println(set.URL("vendor/lib.js"), err)

	// A changed copy no longer builds.
	files["vendor/lib.js"] = &fstest.MapFile{Data: []byte("alert(1);\n")}
	_, err = asset.New(opts)
	fmt.Println(errors.Is(err, asset.ErrIntegrity))
	// Output:
	// /assets/vendor/e729cc9b59986722/vendor/lib.js <nil>
	// true
}

func ExampleSet_Stylesheets() {
	set := exampleSet()
	// Both usually go in the page head: meta.Head()(set.Stylesheets(), set.Scripts()).
	core.Dump(set.Stylesheets())
	core.Dump(set.Scripts())
	// Output:
	// <link rel="stylesheet" href="/assets/app/584b5f123217279a/app.css" integrity="sha384-KvyGCvFPE4H1pQYnn3wkiBzN/uByDyCl8P44/BC/Oz+BqLNWSSxkdBdKM8Ln+mqW" crossorigin="anonymous"/>
	// <script src="/assets/app/584b5f123217279a/app.js" defer integrity="sha384-xkfRlW1o/12WPjFaNyRGWMTu+oSgZhrVjoOW5J3gH5+BR9iBauzFwFUOCOKARbI6" crossorigin="anonymous"></script>
}

func ExampleSet_Preload() {
	files := fstest.MapFS{
		"app.css":         {Data: []byte("@font-face { font-family: App; src: url(fonts/app.woff2); }\n")},
		"fonts/app.woff2": {Data: []byte("wOF2")},
		"logo.svg":        {Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"/>")},
	}
	set := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     files,
		Files:  []asset.File{{Name: "app.css"}, {Name: "fonts/app.woff2"}, {Name: "logo.svg"}},
	}))
	// Fonts are fetched in CORS mode, so the preload is too.
	core.Dump(set.Preload("fonts/app.woff2"))
	// Images from CSS url() and <img> are not, so their preload has no
	// crossorigin (nor integrity, which those fetches do not check).
	core.Dump(set.Preload("logo.svg"))
	// Output:
	// <link rel="preload" href="/assets/app/c95168ba712313a2/fonts/app.woff2" as="font" type="font/woff2" integrity="sha384-S3fIiVERLVzV27O4ScbL7BjI3j+V5ESj0MvZIGneB6cGwCyZ67elbQHGFlBCEAR9" crossorigin="anonymous"/>
	// <link rel="preload" href="/assets/app/c95168ba712313a2/logo.svg" as="image" type="image/svg+xml"/>
}

func ExampleFile_unlisted() {
	files := fstest.MapFS{
		"app.js":    {Data: []byte("new Worker(new URL(\"worker.js\", document.currentScript.src));\n")},
		"worker.js": {Data: []byte("self.onmessage = (e) => self.postMessage(e.data);\n")},
	}
	set := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     files,
		Files: []asset.File{
			{Name: "app.js"},
			// Served, but not loaded by every page as a script.
			{Name: "worker.js", Unlisted: true},
		},
	}))
	core.Dump(set.Scripts())
	fmt.Println(set.URL("worker.js"))
	// Output:
	// <script src="/assets/app/e52bbd9516607556/app.js" defer integrity="sha384-DilBub59HIlguUPrP5pxNczjDPFPT8LE3oIKVuUOi7a0somRjOotv0daOAVOwmea" crossorigin="anonymous"></script>
	// /assets/app/e52bbd9516607556/worker.js
}

func ExampleKind_module() {
	files := fstest.MapFS{
		"main.mjs":  {Data: []byte("import { greet } from \"./greet.mjs\";\ngreet();\n")},
		"greet.mjs": {Data: []byte("export function greet() {}\n")},
	}
	set := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     files,
		Files: []asset.File{
			// The entry point; greet.mjs is only imported, so it keeps the
			// zero kind and is served without a tag.
			{Name: "main.mjs", Kind: asset.KindModule},
			{Name: "greet.mjs"},
		},
	}))
	core.Dump(set.Scripts())
	core.Dump(set.Preload("main.mjs"))
	// Output:
	// <script type="module" src="/assets/app/c4b1ba832d198b03/main.mjs" integrity="sha384-T6AbHcbbYOcE+VAuVIGsmPaNndq0Bb8N9Q+Qq9oJ/+a7XPBwOh7/D1fEJy8Vu6rz" crossorigin="anonymous"></script>
	// <link rel="modulepreload" href="/assets/app/c4b1ba832d198b03/main.mjs" integrity="sha384-T6AbHcbbYOcE+VAuVIGsmPaNndq0Bb8N9Q+Qq9oJ/+a7XPBwOh7/D1fEJy8Vu6rz" crossorigin="anonymous"/>
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
