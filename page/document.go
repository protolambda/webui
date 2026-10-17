package page

import (
	"context"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/meta"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/hx2"
)

// Defaults of a [Document].
const (
	// DefaultLang is the language of a document without one.
	DefaultLang = "en"
	// DefaultFavicon is the icon of a document without one: an empty data
	// URL, so that browsers do not request /favicon.ico.
	DefaultFavicon = "data:,"
	// Viewport is the content of the viewport meta element.
	Viewport = "width=device-width, initial-scale=1"
)

// ColorMode is a Bootstrap color mode: the value of the data-bs-theme
// attribute of the html element, and of the color-scheme meta element that
// tells the browser to draw its own controls (scrollbars, form widgets) to
// match.
type ColorMode string

// The color modes of Bootstrap 5.3.
const (
	// ColorModeDark is the dark mode, which the webui theme styles (it
	// equals the theme package's ColorMode).
	ColorModeDark ColorMode = "dark"
	// ColorModeLight is Bootstrap's light mode.
	ColorModeLight ColorMode = "light"
)

// Valid reports whether m is one of the ColorMode constants.
func (m ColorMode) Valid() bool {
	return m == ColorModeDark || m == ColorModeLight
}

// Document is a complete HTML page: the doctype, the html element, the
// head and the body. The zero value of every field but Title is a sane
// default: language "en", the dark color mode, no favicon request, and
// webui's secure htmx configuration.
//
// It renders:
//
//	<!DOCTYPE html>
//	<html lang="en" data-bs-theme="dark">
//	<head>
//	  <meta charset="utf-8"/>
//	  <meta name="viewport" content="width=device-width, initial-scale=1"/>
//	  <title>Title</title>
//	  <meta name="color-scheme" content="dark"/>
//	  <link rel="icon" href="data:,"/>
//	  <meta name="htmx-config" content="{...}"/>
//	  stylesheets of every set, scripts of every set, Head
//	</head>
//	<body>Body</body>
//	</html>
//
// The charset comes first, within the bytes that browsers sniff. The htmx
// configuration comes before any script and before any content derived
// from input, because htmx reads the first htmx-config element of the
// document. Stylesheets of all sets come before the scripts of all sets:
// the scripts are deferred, so they run in order once the document is
// parsed, and never block the first paint. The document holds no inline
// style or script.
//
// A Document is a value; render it as often as needed. It is safe for
// concurrent use as long as nobody modifies its slices meanwhile.
type Document struct {
	// Lang is the language of the page content, a BCP 47 tag such as "en"
	// or "pt-BR". Empty means [DefaultLang].
	Lang string
	// ColorMode is the Bootstrap color mode of the page. Empty means
	// [ColorModeDark].
	ColorMode ColorMode
	// Title is the text of the title element, which names the page (in tabs,
	// history and for assistive technology). It is required.
	Title string
	// Favicon is the URL of the page icon: a path on the page's own origin
	// (e.g. an asset.Set URL) or a data: URL of an image, in printable
	// ASCII without spaces or backslashes (percent-encode other
	// characters). Paths follow the rule of web.SafeNext, without its
	// length limit. Empty means [DefaultFavicon].
	Favicon string
	// Assets are the asset sets of the page, in order: usually the theme
	// package's set first, then the application's own. A nil set fails the
	// render.
	Assets []*asset.Set
	// HTMX is the htmx configuration of the page. The zero value is webui's
	// secure configuration; see [hx2.Config].
	HTMX hx2.Config
	// Head holds extra head elements, rendered last in the head.
	Head []elem.Node
	// Body is the content of the body element, usually a [Layout]. Nil
	// renders an empty body.
	Body elem.Node
	// HTMLAttrs are extra attributes of the html element, such as data
	// attributes that application CSS reads. They must not set lang or
	// data-bs-theme, which would be duplicates (a render error).
	HTMLAttrs attr.Node
	// BodyAttrs are extra attributes of the body element.
	BodyAttrs attr.Node
}

var _ elem.Node = Document{}

// Eval builds the document. It fails with [ErrNoName] for a blank Title,
// and with [ErrInvalid] for a malformed Lang, an unknown ColorMode, a
// Favicon that is neither a local path nor a data: image, or a nil asset
// set. An invalid HTMX configuration fails with [hx2.ErrInvalidConfig].
func (d Document) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "page.Document"
	lang := d.Lang
	if lang == "" {
		lang = DefaultLang
	}
	if !validLang(lang) {
		return elem.Obj{}, fail(component, ErrInvalid, "lang")
	}
	mode := d.ColorMode
	if mode == "" {
		mode = ColorModeDark
	}
	if !mode.Valid() {
		return elem.Obj{}, fail(component, ErrInvalid, "color mode")
	}
	if blank(d.Title) {
		return elem.Obj{}, fail(component, ErrNoName, "title")
	}
	favicon := d.Favicon
	if favicon == "" {
		favicon = DefaultFavicon
	}
	if !iconURL(favicon) {
		return elem.Obj{}, fail(component, ErrInvalid, "favicon")
	}
	for _, set := range d.Assets {
		if set == nil {
			return elem.Obj{}, fail(component, ErrInvalid, "nil asset set")
		}
	}
	// Check the htmx configuration before anything renders.
	if _, err := d.HTMX.JSON(); err != nil {
		return elem.Obj{}, fmt.Errorf("%s: %w", component, err)
	}

	head := make([]elem.Node, 0, 6+2*len(d.Assets)+len(d.Head))
	head = append(head,
		meta.Meta(meta.Charset("utf-8")),
		meta.Meta(meta.Name("viewport"), meta.Content(Viewport)),
		meta.Title()(text.Text(d.Title)),
		meta.Meta(meta.Name("color-scheme"), meta.Content(string(mode))),
		meta.Link(meta.Rel("icon"), meta.Href(favicon)),
		d.HTMX.Meta(),
	)
	for _, set := range d.Assets {
		head = append(head, set.Stylesheets())
	}
	for _, set := range d.Assets {
		head = append(head, set.Scripts())
	}
	head = append(head, d.Head...)

	body := d.Body
	if body == nil {
		body = elem.Noop()
	}
	htmlAttrs := append([]attr.Node{attr.Lang(lang), attr.Data("bs-theme", string(mode))}, optionalAttrs(d.HTMLAttrs)...)
	return elem.Bundle{
		elem.Doctype(),
		meta.HTML(htmlAttrs...)(
			meta.Head()(head...),
			section.Body(optionalAttrs(d.BodyAttrs)...)(body),
		),
	}.Eval(ctx)
}
