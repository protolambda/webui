# webui

A minimal, security-conscious toolkit for server-rendered web user interfaces in Go.

Pages are typed [chord](https://github.com/protolambda/chord) element trees, styled by Bootstrap 5.3 CSS and a
dark, dense, high-contrast theme, and made interactive by htmx 2 and its server-sent events extension.

- **No application JavaScript, no Bootstrap JavaScript.** Dialogs open as native modal `<dialog>` elements through
  invoker commands; accordions and dropdowns are `<details>`; htmx swaps server-rendered fragments. Every form also
  works as a plain HTML form.
- **Strict content security policy.** Bootstrap, Bootstrap Icons, htmx and the theme are embedded and served from the
  application's own origin with subresource integrity, so scripts, styles and fonts load only from `'self'` (images
  also from `data:` URLs, which Bootstrap's CSS uses for form controls and close buttons): no inline scripts, no
  inline styles, no `eval`. See `mw.StrictCSP` for the full policy.
- **Typed, accessible components.** Navbars, dialogs, cards, form fields and tables render Bootstrap's documented
  markup with ARIA wiring by default, and invalid options or ids fail the render instead of producing broken pages.
- **Secure HTTP defaults.** Security headers, host checks against DNS rebinding, a same-origin policy for
  state-changing requests, CSRF tokens, bounded form reading, deadlines, and hardened server defaults. HSTS is
  opt-in: `mw.HSTSFor` sends it for https sites, and it can stay off when a TLS proxy sends it already.
- **Testable.** `webuitest` checks pages, fragments, responses and event streams with
  [chord's ct](https://github.com/protolambda/chord) queries and [mustbe](https://github.com/protolambda/mustbe)
  assertions.

## Install

```bash
go get github.com/protolambda/webui@latest
```

Requires Go 1.27.

## Quick start

Adapted from [`example_test.go`](./example_test.go): a page with a form, behind the recommended middleware stack.

```go
var assets = theme.MustAssets(theme.Options{}) // embedded Bootstrap, icons, htmx, theme

func layout(title string, main elem.Node) elem.Node {
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
		},
	}
}

type account struct{ Name string } // the value of a signed-in visitor's session

func newHandler(site mw.Site, logger *slog.Logger) (http.Handler, error) {
	store, err := session.NewStore(session.Options[account]{})
	if err != nil {
		return nil, err
	}
	preauth, err := session.NewPreAuth(session.PreAuthOptions{})
	if err != nil {
		return nil, err
	}
	forms := session.Forms[account]{PreAuth: preauth} // CSRF tokens, with and without a session
	errs := web.Errors{Assets: []*asset.Set{assets}}  // error pages, and alerts for htmx requests

	mux := http.NewServeMux()
	mux.Handle(assets.Pattern(), assets)
	mux.Handle("GET /{$}", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		token, err := forms.PageToken(w, r)
		if err != nil {
			return err
		}
		return web.Render(w, r, http.StatusOK, layout("Today", taskForm(token, "", "")))
	}))
	mux.Handle("POST /tasks", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		f, err := forms.Read(w, r) // bounded, and the CSRF token checked
		if err != nil {
			return err // 400, 403, 413 or 415, through errs
		}
		defer f.Zero()
		title := strings.TrimSpace(f.String("title"))
		if title == "" {
			form := taskForm(f.String(session.CSRFField), title, "Give the task a title.")
			return web.Choose(w, r, http.StatusUnprocessableEntity, form, layout("Today", form))
		}
		// ... save the task
		web.Redirect(w, r, "/")
		return nil
	}))
	mux.Handle("/", errs.NotFound())

	return mw.Chain(mux,
		mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux}),
		errs.Recover,
		errs.Intercept,
		mw.BodyDeadline(mw.DefaultBodyTimeout),
		mw.WriteDeadline(mw.DefaultWriteTimeout),
		mw.SecurityHeaders(mw.Headers{HSTS: mw.HSTSFor(site)}),
		mw.HostCheck(site),
		mw.SameOrigin(mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)}),
		mw.MaxBody(1<<20),
		store.Middleware,
	), nil
}

// taskForm works as a plain form, and htmx swaps it in place.
func taskForm(token, title, problem string) elem.Node {
	var msg elem.Node
	if problem != "" {
		msg = text.Text(problem)
	}
	return session.PostForm("/tasks", token, attr.ID("task-form"), hx2.Post("/tasks"), hx2.Swap(hx2.SwapOuterHTML))(
		bs.Field{ID: "task-title", Name: "title", Label: text.Text("New task"), Value: title, Error: msg},
		bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Add")),
	)
}

func main() {
	logger := slog.Default()
	// The origin that browsers use, served by a TLS proxy in front of the listener.
	handler, err := newHandler(mw.MustParseSite("https://tasks.example.com"), logger)
	if err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, server.New(handler, server.Options{Logger: logger}), ln); err != nil {
		log.Fatal(err)
	}
}
```

The order of the middleware matters:

- `errs.Recover` and `errs.Intercept` sit right inside the request log, so that a panic becomes an error page and
  a plain-text error that other code writes (`http.Error`, a refusal of the host check) reaches htmx as an alert:
  htmx inserts every answer that it swaps as HTML.
- The host check and the same-origin policy refuse before any handler runs; `errs.Answer` turns the same-origin
  refusal into the error page, or an alert for htmx.
- The session middleware comes last, so that refused requests never count as use of a session. Without it,
  `session.FromContext` finds nothing and every visitor looks signed out.

The [gallery](./internal/gallery) is a complete demo of every package: `go run ./internal/gallery`, then open
http://127.0.0.1:8080.

## Forms and sessions

Every POST form carries a CSRF token: `session.PostForm` renders the form with the token's hidden field first. The
token comes from `session.Forms`: the session's token for a signed-in visitor, or a stateless pre-authentication
token (`session.PreAuth`, a signed cookie and a form token) for anyone else. `Forms.Read` reads the form within
bounds and checks the token before the handler sees any value; return its error to the error policy, which answers
with 400, 403, 413 or 415. A fragment that updates part of a page takes its token from `Forms.FragmentToken`, so that
the page's other forms stay valid.

A login checks its form with `PreAuth.Verify`, starts the session with `Store.New` (a new session ID), and clears
the pre-authentication cookie. Pages behind `errs.RequireAuth` send visitors without a session to the login page,
and back. Inside `newHandler`:

```go
mux.Handle("POST /login", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
	f, err := form.Read(w, r, form.Limits{})
	if err != nil {
		return err
	}
	defer f.Zero()
	next := web.SafeNext(f.String(web.NextParam), "/")
	if err := preauth.Verify(r, f.String(session.CSRFField)); err != nil {
		// The form expired: show it again with a fresh token, the name and next, never the password.
		return showLogin(w, r, http.StatusForbidden, f.String("name"), next, "The form expired. Log in again.")
	}
	password := f.Take("password") // owned by the handler now: cleared after the check
	name, ok := checkLogin(f.String("name"), password)
	clear(password)
	if !ok {
		return showLogin(w, r, http.StatusUnprocessableEntity, name, next, "The name or password is wrong.")
	}
	if _, err := store.New(w, r, account{Name: name}); err != nil {
		return err
	}
	preauth.ClearCookie(w)
	web.Redirect(w, r, next)
	return nil
}))

signedIn := errs.RequireAuth("/login", func(r *http.Request) bool {
	_, ok := session.FromContext[account](r.Context())
	return ok
})
mux.Handle("GET /settings", signedIn(errs.Handle(settings)))
```

`showLogin` renders the login form with a token from `preauth.Issue`. The session package's examples cover the rest:
logout (`Store.EndSession`), rotation, expiry and fragments.

Text that other users chose is escaped against markup by chord, but can still mislead: a right-to-left override
reverses what follows it, and invisible characters make two names look the same. Show names and titles with
`visible.Text`, and identifiers with `visible.Code`.

## Packages

| Package | Purpose |
|---|---|
| [`asset`](./asset) | Embedded files as an immutable set: integrity values, content-versioned URLs, immutable caching, gzip, and the `<link>`/`<script>` tags that load them. |
| [`theme`](./theme) | The embedded browser stack (Bootstrap 5.3.8 CSS, Bootstrap Icons 1.13.1, htmx 2.0.10, htmx SSE 2.2.4; opt-in Bootstrap JS and the htmx WebSocket extension) and the webui theme stylesheet. |
| [`page`](./page) | The HTML document and the page layout: navbar, sidebar, header, alerts and dialogs regions. |
| [`bs`](./bs) | Bootstrap 5.3 components, including script-free dialogs, accordions, dropdowns, toasts, form fields and data tables. |
| [`ba`](./ba) | Bootstrap 5.3 class utilities, validated when rendering. |
| [`bi`](./bi) | Bootstrap Icons, decorative by default or labelled. |
| [`hx2`](./hx2) | htmx 2 attributes, the htmx configuration (secure by default), and request and response headers. |
| [`hx1`](./hx1) | htmx 1 attributes, for legacy pages. |
| [`sse`](./sse) | Server-sent event streams of rendered fragments for htmx SSE swaps, with a coalescing update loop. |
| [`web`](./web) | Handlers that return errors, rendering, htmx-aware redirects and error answers, auth guards, notices. |
| [`mw`](./mw) | Middleware: security headers and a typed CSP builder, host check, same-origin policy, deadlines, body limits, request logging. |
| [`session`](./session) | Cookie sessions with hashed tokens, lifetimes, rotation and CSRF tokens; stateless tokens for anonymous forms; forms that carry and check them. |
| [`form`](./form) | Bounded form reading into owned, zeroable bytes. |
| [`visible`](./visible) | Untrusted text that shows what it is: escapes direction controls, invisible and collapsing characters, with a marker. |
| [`server`](./server) | Hardened `http.Server` defaults, a graceful run loop, per-client connection limits. |
| [`webuitest`](./webuitest) | Test helpers: page rules, page and fragment contracts, browser-like requests and form submission (also as htmx sends it), response assertions, SSE streams, crawling, cross-site probes. |

The `bs`, `ba`, `bi`, `hx1` and `hx2` packages started in chord. Their APIs stay familiar, and moving code over is
mostly a change of import path, with these exceptions:

- `bs.Modal.Size` and `bs.Offcanvas.Placement` are typed (`bs.ModalSize`, `bs.OffcanvasPlacement`), so a string
  variable needs a conversion.
- Invalid ids, options and utility values (`bs`, `ba`, `hx2`) fail the render instead of producing broken markup.
- Components render additional ARIA attributes and different heading levels.

## Testing pages

webuitest sends requests as a browser on `webuitest.BaseURL` (https://app.example.test) sends them, so the handler
under test is built for that site:

```go
func TestToday(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite(webuitest.BaseURL)
	handler, err := newHandler(site, slog.New(slog.DiscardHandler))
	t.Must(be.NoError(err))

	b := webuitest.NewBrowser(t, handler, webuitest.BrowserOptions{}) // keeps cookies
	res := b.Get(t, "/")
	t.Must(res.Status(http.StatusOK))
	t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(mw.Headers{HSTS: mw.HSTSFor(site)})))
	page := res.HTML()
	t.Must(webuitest.PageContract(page, webuitest.DefaultShell(assets), webuitest.Rules()...))
	t.Must(page.Find(ct.Role("heading", ct.Named("Today"))))

	// Submit the form as the browser does, with its CSRF token.
	res = b.Submit(t, page, page.Find(ct.ID("task-form")), nil, url.Values{"title": {" "}})
	t.Must(res.Status(http.StatusUnprocessableEntity))
	t.Must(res.HTML().Find(ct.ID("task-title-error")).Texts("Give the task a title."))
}
```

## Browser support

webui targets current evergreen browsers. Dialogs use invoker commands (`commandfor`/`command`) and fall back to the
Popover API; the theme uses `:has()`. Pages render and work without script; htmx and its SSE extension add partial
updates.

## License

MIT, see [`LICENSE`](./LICENSE). The embedded third-party files keep their own licenses, next to them in
[`theme/files`](./theme/files): Bootstrap and Bootstrap Icons (MIT), htmx and its extensions (BSD Zero Clause).
