// Package theme embeds webui's browser stack and serves it as an
// [asset.Set]: Bootstrap 5.3 CSS, Bootstrap Icons, the webui theme
// stylesheet, htmx 2 and its SSE extension, plus opt-in Bootstrap JS and the
// htmx WebSocket extension.
//
// Every page loads these files from the application's own origin, never from
// a CDN, with integrity values, so the content security policy can allow
// only 'self'.
//
// # The webui theme
//
// webui.css is linked after Bootstrap. It adapts Bootstrap's dark color mode
// through Bootstrap's own CSS variables and classes, and adds no classes of
// its own: a deep black, dense, border-driven look like an IDE, with high
// contrast text, flat surfaces, small radii, and no shadows or gradients.
// The stylesheet also holds the few behavior rules that let components work
// without script: dialogs shown as modals through invoker commands,
// details-based accordions and dropdowns, and htmx request indicators (whose
// injected style webui's htmx configuration turns off, as the content
// security policy blocks inline styles).
//
// The theme applies to a whole page whose html element has
// data-bs-theme="dark" ([ColorMode]). A page in the light mode is stock
// Bootstrap, and so are Bootstrap's per-component dark modes on it: a navbar
// or dropdown marked data-bs-theme="dark" on a light page is stock Bootstrap
// dark. On a dark page, elements marked dark again keep the theme; a light
// region inside a dark page is not supported. Classes with fixed colors keep
// Bootstrap's colors, which glare or fall short on black: the .bg-* utilities,
// .text-secondary, .btn-light, .btn-dark and the contextual table variants
// (.table-light and the others). Prefer those that follow the theme:
// .text-bg-*, .text-body-secondary, .bg-body-tertiary, .border.
//
// # Tuning
//
// The stylesheet's own custom properties, prefixed --webui-, are its tuning
// surface; everything else in it re-points Bootstrap's --bs-* properties. A
// page overrides them in a stylesheet of its own, linked after webui.css
// (for example a second [asset.Set] after the theme's in the page's assets),
// with a selector at least as specific as [data-bs-theme="dark"], where the
// defaults are declared:
//
//	[data-bs-theme="dark"] { --webui-focus-color: var(--bs-warning); }
//
// A bare html selector is less specific and loses. A class selector sets
// them for the subtree of the elements it matches. The tokens:
//
//   - --webui-control-border: the border of fields, outline buttons, floating
//     layers (dropdown menus, modals, toasts) and progress tracks.
//   - --webui-focus-color, --webui-focus-width, --webui-focus-offset: the one
//     keyboard focus outline, which replaces Bootstrap's focus glows.
//   - --webui-nav-link-color: the links of a nav (tabs, pills, underline).
//   - --webui-fill-primary, -secondary, -success, -danger, -warning and
//     -info, each with a -hover variant: the fills of buttons, badges
//     (.text-bg-*), the selected item of a list group or pagination and a
//     checked box, darker than Bootstrap's theme colors. Selected items
//     without a border (an active nav pill, the active dropdown item) take
//     --webui-fill-primary-hover, so that they keep 3:1 against black.
//
// The header comment of webui.css documents the contrast ratios of the
// defaults; an override is responsible for its own.
//
// # Third-party files
//
// The third-party files are byte-identical to their published releases, and
// each is pinned by an integrity constant ([BootstrapCSSIntegrity] and
// others): [Assets] refuses to build a set from a file that does not match.
// Their licenses are in the files directory next to them.
//
// To update one, download exactly the file the project documents for the
// new version (never through a package manager), check its SHA-384
// (openssl dgst -sha384 -binary FILE | base64) against the value the project
// publishes, and against the npm registry tarball and the jsDelivr copy
// where the project publishes none, then replace the file and its license,
// and update the version, file name and integrity constants. Do not "fix"
// anything inside a file: the hashes cover every byte (the Bootstrap
// stylesheet, for example, contains a no-break space that must stay).
//
// Sources, and where the integrity values come from:
//   - Bootstrap CSS and JS: https://getbootstrap.com/docs/5.3/getting-started/introduction/
//     (files from https://cdn.jsdelivr.net/npm/bootstrap@VERSION/dist/).
//   - Bootstrap Icons: https://icons.getbootstrap.com/ (files from
//     https://cdn.jsdelivr.net/npm/bootstrap-icons@VERSION/font/); the
//     project publishes no integrity value, so the pin was checked against
//     the npm tarball integrity and jsDelivr's file hashes. Only the woff2
//     font is embedded: browsers that support woff2 never request the woff
//     fallback that the stylesheet also names.
//   - htmx: https://htmx.org/docs/#installing.
//   - htmx SSE and WebSocket extensions: https://htmx.org/extensions/sse/ and
//     https://htmx.org/extensions/ws/.
package theme

import (
	"embed"
	"io/fs"

	"github.com/protolambda/webui/asset"
)

// Versions of the embedded third-party files.
const (
	BootstrapVersion = "5.3.8"
	IconsVersion     = "1.13.1"
	HTMXVersion      = "2.0.10"
	SSEVersion       = "2.2.4"
	WSVersion        = "2.0.4"
)

// Names of the files in a theme [asset.Set], for [asset.Set.URL] and the
// other lookups.
const (
	BootstrapCSS = "bootstrap-" + BootstrapVersion + ".min.css"
	IconsCSS     = "bootstrap-icons-" + IconsVersion + ".min.css"
	// IconsFont is the font that IconsCSS loads, at the relative path the
	// stylesheet names.
	IconsFont = "fonts/bootstrap-icons.woff2"
	ThemeCSS  = "webui.css"
	HTMXJS    = "htmx-" + HTMXVersion + ".min.js"
	SSEJS     = "htmx-ext-sse-" + SSEVersion + ".min.js"
	// BootstrapJS is Bootstrap's script bundle (with Popper), for the
	// Bootstrap components that need it. It is opt-in: see
	// [Options.BootstrapJS].
	BootstrapJS = "bootstrap-" + BootstrapVersion + ".bundle.min.js"
	// WSJS is the htmx WebSocket extension. It is opt-in: see
	// [Options.WebSocket].
	WSJS = "htmx-ext-ws-" + WSVersion + ".min.js"
)

// Integrity values that pin the third-party files, each named after its
// file constant. Bootstrap and htmx publish these values; Bootstrap Icons
// publishes none, so its two values were checked against the npm registry
// tarball and jsDelivr (see the package documentation).
const (
	BootstrapCSSIntegrity = "sha384-sRIl4kxILFvY47J16cr9ZwB07vP4J8+LH7qKQnuqkuIAvNWLzeN8tE5YBujZqJLB"
	BootstrapJSIntegrity  = "sha384-FKyoEForCGlyvwx9Hj09JcYn3nv7wiPVlz7YYwJrWVcXK/BmnVDxM+D2scQbITxI"
	IconsCSSIntegrity     = "sha384-CK2SzKma4jA5H/MXDUU7i1TqZlCFaD4T01vtyDFvPlD97JQyS+IsSh1nI2EFbpyk"
	IconsFontIntegrity    = "sha384-xEoI56EFpIZiDZZKBZxsn3gO3u/FvXtOpHbtkMWmSdfzDw3x9XdVc3i70O9hm4SC"
	HTMXJSIntegrity       = "sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V"
	SSEJSIntegrity        = "sha384-A986SAtodyH8eg8x8irJnYUk7i9inVQqYigD6qZ9evobksGNIXfeFvDwLSHcp31N"
	WSJSIntegrity         = "sha384-1RwI/nvUSrMRuNj7hX1+27J8XDdCoSLf0EjEyF69nacuWyiJYoQ/j39RT1mSnd2G"
)

// ColorMode is the Bootstrap color mode the webui theme styles: the value
// of the data-bs-theme attribute of the html element.
const ColorMode = "dark"

// DefaultPrefix is the URL path prefix of the theme's files when
// [Options.Prefix] is empty.
const DefaultPrefix = "/assets/webui/"

//go:embed files
var files embed.FS

// FS returns the embedded files, by the names of the file constants.
func FS() fs.FS {
	sub, err := fs.Sub(files, "files")
	if err != nil {
		// The directory is embedded; fs.Sub fails only on an invalid name.
		panic(err)
	}
	return sub
}

// Options select the files of a theme set. The zero value serves the
// default stack under [DefaultPrefix].
type Options struct {
	// Prefix is the URL path prefix, see [asset.Options]. Empty means
	// DefaultPrefix.
	Prefix string
	// WithoutIcons leaves out Bootstrap Icons (stylesheet and font), for
	// pages that use no bi icons.
	WithoutIcons bool
	// WithoutSSE leaves out the htmx SSE extension, for pages without
	// server-sent events.
	WithoutSSE bool
	// BootstrapJS adds Bootstrap's script bundle, which the Bootstrap
	// components that are documented to need it (such as bs.Modal and
	// bs.Dropdown) require. webui's own components need no script. The
	// default content security policy allows it, as it is served from the
	// application's origin.
	BootstrapJS bool
	// WebSocket adds the htmx WebSocket extension, for hx2.WSConnect. A
	// WebSocket connection to the application's origin also needs the
	// content security policy's connect-src to allow it: browsers differ in
	// whether 'self' covers ws: and wss: URLs, so name them explicitly.
	WebSocket bool
}

// Files returns the file list of a theme set, in load order: stylesheets
// Bootstrap, icons, webui; scripts htmx, its extensions, then Bootstrap JS.
func (o Options) Files() []asset.File {
	out := []asset.File{{Name: BootstrapCSS, Integrity: BootstrapCSSIntegrity}}
	if !o.WithoutIcons {
		out = append(out,
			asset.File{Name: IconsCSS, Integrity: IconsCSSIntegrity},
			asset.File{Name: IconsFont, Integrity: IconsFontIntegrity},
		)
	}
	out = append(out,
		asset.File{Name: ThemeCSS},
		asset.File{Name: HTMXJS, Integrity: HTMXJSIntegrity},
	)
	if !o.WithoutSSE {
		out = append(out, asset.File{Name: SSEJS, Integrity: SSEJSIntegrity})
	}
	if o.WebSocket {
		out = append(out, asset.File{Name: WSJS, Integrity: WSJSIntegrity})
	}
	if o.BootstrapJS {
		out = append(out, asset.File{Name: BootstrapJS, Integrity: BootstrapJSIntegrity})
	}
	return out
}

// Assets returns the set of the selected theme files. It fails only if an
// embedded file does not match its pinned integrity, or opts.Prefix is
// invalid.
func Assets(opts Options) (*asset.Set, error) {
	prefix := opts.Prefix
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return asset.New(asset.Options{Prefix: prefix, FS: FS(), Files: opts.Files()})
}

// MustAssets is like [Assets] but panics on error: the files are embedded,
// so an error is a programming error (an invalid prefix).
func MustAssets(opts Options) *asset.Set {
	return asset.Must(Assets(opts))
}
