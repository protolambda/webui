// Package webui is a minimal, security-conscious toolkit for server-rendered
// web user interfaces in Go.
//
// Pages are typed [github.com/protolambda/chord] element trees, styled by
// Bootstrap 5.3 CSS and a dark, dense, high-contrast theme, and made
// interactive by htmx 2 and its server-sent events extension. Pages need no
// application JavaScript and no Bootstrap JavaScript: dialogs, accordions and
// dropdowns are native HTML, and htmx swaps server-rendered fragments. Every
// asset is embedded and served from the application's own origin with
// subresource integrity, so the content security policy allows only 'self'.
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
//	├── session/    # cookie sessions and CSRF tokens
//	├── form/       # bounded form reading into owned, zeroable bytes
//	├── server/     # hardened http.Server defaults and a graceful run loop
//	└── webuitest/  # test helpers on chord's ct: page rules, contracts, requests, responses, SSE, crawling
//
// A page is a [github.com/protolambda/webui/page.Document] around a
// [github.com/protolambda/webui/page.Layout], with the asset set of
// [github.com/protolambda/webui/theme.Assets]. Handlers are
// [github.com/protolambda/webui/web.HandlerFunc] values that return errors,
// behind the middleware of [github.com/protolambda/webui/mw], served by
// [github.com/protolambda/webui/server.Run]. Tests check pages and responses
// with [github.com/protolambda/webui/webuitest].
package webui
