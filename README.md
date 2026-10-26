# webui

A Go toolkit for server-rendered web interfaces, with typed components, a compact dark theme, and htmx interactions.

Build views with [chord](https://github.com/protolambda/chord), style them with Bootstrap 5.3, and serve them through
ordinary `net/http` handlers. Links and forms work as HTML; htmx adds partial updates, and server-sent events keep
live regions current.

- **Views in Go.** Compose pages, navigation, forms, tables, and dialogs from typed elements.
- **Interaction without application JavaScript.** Use native dialogs and disclosure controls, htmx swaps, and event streams.
- **Security helpers.** Local assets with integrity checks, strict CSP, origin checks, CSRF protection, cookie sessions,
  and bounded request handling.
- **Tests at the HTTP boundary.** Check rendered pages, form submissions, cookies, headers, and streams with `webuitest`.

[Go reference](https://pkg.go.dev/github.com/protolambda/webui) · [Application example](./example_test.go) · [Gallery](./internal/gallery)

## Get started

Requires Go 1.27 or later.

```sh
go get github.com/protolambda/webui@latest
```

To explore the components and their behavior, run the gallery from a checkout:

```sh
go run ./internal/gallery
```

Open [127.0.0.1:8080](http://127.0.0.1:8080). The gallery includes forms, dialogs, live updates, and a demo login;
its [handlers](./internal/gallery/app) are examples you can adapt.

## Build a page

A document supplies the assets and page metadata. A layout supplies navigation, a header, and the main content.
The default theme is dark, dense, and high contrast; assets are embedded and served by your application.

```go
var assets = theme.MustAssets(theme.Options{})

func home(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, page.Document{
		Title:  "Tasks",
		Assets: []*asset.Set{assets},
		Body: page.Layout{
			Navbar: page.Navbar{
				Brand: page.Brand{Label: "My app", Href: "/"},
			},
			Header: page.Header{Title: text.Text("Tasks")},
			Main:   text.P()(text.Text("Your work starts here.")),
		},
	})
}
```

Register the assets and handler with a standard router. `web.Errors` turns returned errors into error pages or
htmx alerts, including the 404 and 405 answers for requests that no route matches.

```go
errs := web.Errors{Assets: []*asset.Set{assets}}
mux := http.NewServeMux()
mux.Handle(assets.Pattern(), assets)
mux.Handle("GET /{$}", errs.Handle(home))
handler := errs.Mux(mux)
```

The [application example](./example_test.go) shows the complete handler setup, including the recommended
middleware order and a working form. Use [`mw`](./mw/doc.go) for request policies and
[`server`](./server/doc.go) for server defaults and graceful shutdown.

## Forms and interaction

Forms keep their ordinary HTML action and method. Add htmx attributes to choose which request to make and how
to update the page:

```go
func taskForm(token string) elem.Node {
	return session.PostForm("/tasks", token,
		attr.ID("task-form"),
		hx2.Post("/tasks"),
		hx2.Swap(hx2.SwapOuterHTML),
	)(
		bs.Field{ID: "task-title", Name: "title", Label: text.Text("New task")},
		bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Add task")),
	)
}
```

[`session.Forms`](./session/forms.go) supplies CSRF tokens and reads submissions within bounds, checking the token
before the handler uses the values. `web.Choose` returns a fragment or a full page as appropriate, and
`web.Redirect` handles navigation after a successful submission. See the [form examples](./session/forms_example_test.go)
for the complete flow.

For live views, [`sse`](./sse/doc.go) streams rendered fragments to htmx regions and handles update coalescing,
keep-alives, and stream lifetimes. Session and login helpers live in [`session`](./session/doc.go); your application
supplies credential verification and access rules.

## Find your way around

| What you need | Packages |
|---|---|
| Page structure and components | [`page`](./page), [`bs`](./bs), [`ba`](./ba), [`bi`](./bi) |
| Theme and embedded assets | [`theme`](./theme), [`asset`](./asset) |
| Forms, sessions, and CSRF | [`form`](./form), [`session`](./session) |
| Handlers, middleware, and serving | [`web`](./web), [`mw`](./mw), [`server`](./server) |
| Partial updates and live regions | [`hx2`](./hx2), [`sse`](./sse) |
| Displaying untrusted names and identifiers | [`visible`](./visible) |
| Testing pages and request flows | [`webuitest`](./webuitest) |

The package documentation covers contracts and configuration; executable `Example` tests show how the APIs fit
together. For example:

```sh
go doc github.com/protolambda/webui/session
go doc github.com/protolambda/webui/web Errors
go doc github.com/protolambda/webui/sse Loop
```

## Test your application

[`webuitest`](./webuitest/doc.go) provides handler-level clients and a test-server client with TLS and cookies.
It can submit the forms rendered by your pages, send htmx requests, read event streams, and check page structure,
accessibility rules, CSP, and CSRF wiring.

Start with the [application test](./example_test.go), then see the
[testing examples](./webuitest/example_test.go) and [gallery tests](./internal/gallery/app/app_test.go).

Checks in a real browser that fail on console messages need to allow the ones that webui causes by design: every
4xx or 5xx answer logs the browser's "Failed to load resource" error, an error answer to an htmx request adds
htmx's "Response Status Error Code", and dialogs add two Chromium messages. The "Browser console" section of
`go doc github.com/protolambda/webui/web Errors` lists them.

## Browser support

webui targets current evergreen browsers. Native dialogs use invoker commands with a Popover API fallback;
the theme uses `:has()`. htmx enhances the server-rendered pages, while ordinary links and forms remain usable
without script.

## License

[MIT](./LICENSE). Embedded Bootstrap, Bootstrap Icons, htmx, and extension files retain their own
[licenses](./theme/files).
