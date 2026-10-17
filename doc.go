// Package webui is a minimal, security-conscious toolkit for server-rendered
// web user interfaces in Go.
//
// Pages are typed [github.com/protolambda/chord] element trees, styled by
// Bootstrap 5.3 CSS and a dark, dense, high-contrast theme, and made
// interactive by htmx 2 and its server-sent events extension. Pages need no
// application JavaScript and no Bootstrap JavaScript: dialogs, accordions and
// dropdowns are native HTML, and htmx swaps server-rendered fragments. Every
// asset is embedded and served from the application's own origin with
// subresource integrity, so the content security policy
// ([github.com/protolambda/webui/mw.StrictCSP]) loads scripts, styles and
// fonts only from 'self', and images only from 'self' and data: URLs, which
// Bootstrap's CSS uses for the icons of form controls and close buttons.
//
// This package is the starting index; applications import the packages
// below it.
//
// # Package tree
//
// Paths are relative to github.com/protolambda/webui.
//
//	webui/
//	├── asset/      # embedded files as an immutable set: integrity, versioned URLs, serving, <link>/<script> tags
//	├── theme/      # the embedded browser stack (Bootstrap, Bootstrap Icons, htmx, htmx SSE) and the webui theme
//	├── page/       # the HTML document and the page layout: navbar, sidebar, header, alerts and dialogs regions
//	├── bs/         # Bootstrap 5.3 components, including script-free dialogs, accordions, dropdowns and form fields
//	├── ba/         # Bootstrap 5.3 class utilities
//	├── bi/         # Bootstrap Icons
//	├── hx2/        # htmx 2 attributes, the htmx configuration, and request and response headers
//	├── hx1/        # htmx 1 attributes, for legacy pages
//	├── sse/        # server-sent event streams of rendered fragments, for htmx SSE swaps
//	├── web/        # handlers that return errors, rendering, htmx-aware redirects and error answers
//	├── mw/         # middleware: security headers and CSP, host check, same-origin policy, deadlines, logging
//	├── session/    # cookie sessions, CSRF tokens, and the forms that carry and check them
//	├── form/       # bounded form reading into owned, zeroable bytes
//	├── server/     # hardened http.Server defaults and a graceful run loop
//	├── visible/    # untrusted text: escapes direction controls, invisible and collapsing characters, with a marker
//	└── webuitest/  # test helpers on chord's ct: page rules, contracts, requests, responses, SSE, crawling
//
// A page is a [github.com/protolambda/webui/page.Document] around a
// [github.com/protolambda/webui/page.Layout], with the asset set of
// [github.com/protolambda/webui/theme.Assets]. Handlers are
// [github.com/protolambda/webui/web.HandlerFunc] values that return errors
// to the error policy of [github.com/protolambda/webui/web.Errors], behind
// the middleware of [github.com/protolambda/webui/mw] and the session
// middleware of [github.com/protolambda/webui/session.Store], served by
// [github.com/protolambda/webui/server.Run]. Forms carry the CSRF tokens of
// [github.com/protolambda/webui/session.Forms], which reads and checks them.
// Tests check pages and responses with
// [github.com/protolambda/webui/webuitest].
//
// The package example is a complete application with a form, behind the
// recommended middleware stack, in the order that the package docs of mw
// and web describe.
package webui
