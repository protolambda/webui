package webuitest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/mustbe/assertion"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/theme"
)

// Asset is a stylesheet or script that a page shell loads.
type Asset struct {
	// Name names the file in messages, e.g. its name in an [asset.Set].
	// It may be empty.
	Name string
	// URL is the exact href of the stylesheet or src of the script.
	URL string
	// Integrity is the exact integrity value. Empty requires any
	// well-formed sha256, sha384 or sha512 value.
	Integrity string
	// Module marks a JavaScript module (asset.KindModule), loaded with
	// <script type="module">, which runs deferred by nature. A script
	// without it is a classic script, which must have defer and must not
	// be a module: the two kinds run with different semantics.
	Module bool
}

func (a Asset) String() string {
	if a.Name != "" {
		return a.Name
	}
	return a.URL
}

// Shell is what every page of an application has in common, the
// expectation that [ShellContract] checks: the document head that webui's
// page.Document renders, with the application's assets.
//
// The zero value of each string field means webui's default, so a Shell
// literal states only what differs; [DefaultShell] fills every field.
type Shell struct {
	// Lang is the lang attribute of the html element; "" means "en".
	Lang string
	// ColorMode is the data-bs-theme attribute of the html element, also
	// listed by the color-scheme meta element; "" means [theme.ColorMode].
	ColorMode string
	// Icon is the exact href of the icon link; "" requires a data: URL,
	// such as the empty "data:,", which keeps the browser from requesting
	// /favicon.ico.
	Icon string
	// HTMXConfig is the JSON content of the htmx-config meta element,
	// compared as JSON (key order does not matter); "" means the secure
	// default, hx2.Config{}.JSON().
	HTMXConfig string
	// Stylesheets are all stylesheet links of the page, in order.
	Stylesheets []Asset
	// Scripts are all scripts and modules of the page, in order. Classic
	// scripts must be deferred; modules ([Asset.Module]) run deferred by
	// nature.
	Scripts []Asset
}

var defaultThemeSet = sync.OnceValue(func() *asset.Set {
	return theme.MustAssets(theme.Options{})
})

// DefaultHTMXConfig returns the JSON of the zero [hx2.Config], webui's
// secure htmx configuration.
func DefaultHTMXConfig() string {
	s, err := hx2.Config{}.JSON()
	if err != nil {
		// The zero Config always encodes.
		panic(fmt.Sprintf("webuitest: default htmx config: %v", err))
	}
	return s
}

// DefaultShell returns the shell of pages that load sets, in order, as
// page.Document renders them: the stylesheets of all sets, then the
// scripts and modules of all sets, each in the order of the files of its
// set, with their URLs and integrity values, and every other field at
// webui's default. Files that the sets leave out of their rendered
// elements (asset.File.Unlisted) are left out, as are files of other
// kinds; a module (asset.KindModule) is an [Asset] with Module set.
// Without sets, it uses the default theme set,
// theme.MustAssets(theme.Options{}). A nil set panics. Each call returns a
// new Shell, which the caller may extend.
func DefaultShell(sets ...*asset.Set) Shell {
	if len(sets) == 0 {
		sets = []*asset.Set{defaultThemeSet()}
	}
	s := Shell{
		Lang:       "en",
		ColorMode:  theme.ColorMode,
		HTMXConfig: DefaultHTMXConfig(),
	}
	for i, set := range sets {
		if set == nil {
			panic(fmt.Sprintf("webuitest.DefaultShell: set %d is nil", i))
		}
		for info := range set.Files() {
			if info.Unlisted {
				continue
			}
			a := Asset{Name: info.Name, URL: info.URL, Integrity: info.Integrity}
			switch info.Kind {
			case asset.KindStylesheet:
				s.Stylesheets = append(s.Stylesheets, a)
			case asset.KindScript:
				s.Scripts = append(s.Scripts, a)
			case asset.KindModule:
				a.Module = true
				s.Scripts = append(s.Scripts, a)
			}
		}
	}
	return s
}

// Rule returns the shell expectation as a document rule, "shell()", which
// [ShellContract] and [PageContract] apply. The page must have:
//
//   - a <!DOCTYPE html> (without it, browsers render in quirks mode);
//   - an html element with lang and data-bs-theme;
//   - a head whose first element is <meta charset="utf-8">;
//   - one viewport meta element with width=device-width;
//   - one non-empty title, in the head;
//   - one color-scheme meta element that lists the color mode;
//   - one icon link: a data: URL, or [Shell.Icon];
//   - one htmx-config meta element in the head with the expected JSON
//     (each differing key is named);
//   - no base element (webui's policy sets base-uri 'none');
//   - exactly the stylesheet links of [Shell.Stylesheets] and the scripts
//     of [Shell.Scripts], in order, in the head, each with its integrity
//     value; classic scripts deferred, modules ([Asset.Module]) with
//     type="module", and none async;
//   - one main landmark.
func (s Shell) Rule() ct.Rule {
	return ct.CustomRule("shell()", s.check)
}

func (s Shell) lang() string {
	if s.Lang == "" {
		return "en"
	}
	return s.Lang
}

func (s Shell) colorMode() string {
	if s.ColorMode == "" {
		return theme.ColorMode
	}
	return s.ColorMode
}

func (s Shell) htmxConfig() string {
	if s.HTMXConfig == "" {
		return DefaultHTMXConfig()
	}
	return s.HTMXConfig
}

func (s Shell) check(root ct.Node) []string {
	var out []string
	var html ct.Node
	doctype := false
	for c := range root.Children() {
		switch c.Kind() {
		case ct.KindDoctype:
			if html == nil && strings.EqualFold(strings.TrimSpace(c.Data()), "html") {
				doctype = true
			}
		case ct.KindElement:
			if c.Tag() == "html" && html == nil {
				html = c
			}
		}
	}
	if !doctype {
		out = append(out, "document has no <!DOCTYPE html> before the html element")
	}
	if html == nil {
		return append(out, "document has no html element")
	}
	out = append(out, wantAttr(html, "lang", s.lang())...)
	out = append(out, wantAttr(html, "data-bs-theme", s.colorMode())...)
	head := firstChild(html, "head")
	if head == nil {
		return append(out, "html element has no head")
	}
	first := firstChild(head, "")
	if first == nil || first.Tag() != "meta" || !hasAttr(first, "charset") {
		out = append(out, "the first element of the head is not <meta charset>")
	} else if cs, _ := attrOf(first, "charset"); !strings.EqualFold(strings.TrimSpace(cs), "utf-8") {
		out = append(out, fmt.Sprintf("meta charset is %q, want \"utf-8\"", cs))
	}
	metas := func(name string) []ct.Node {
		var found []ct.Node
		for m := range Elements(root, "meta") {
			if n, _ := attrOf(m, "name"); strings.EqualFold(n, name) {
				found = append(found, m)
			}
		}
		return found
	}
	if vp := metas("viewport"); len(vp) != 1 {
		out = append(out, fmt.Sprintf("document has %d viewport meta elements, want 1", len(vp)))
	} else if c, _ := attrOf(vp[0], "content"); !strings.Contains(strings.ReplaceAll(c, " ", ""), "width=device-width") {
		out = append(out, "viewport meta element at "+loc(vp[0])+" lacks width=device-width")
	}
	var titles []ct.Node
	for t := range Elements(root, "title") {
		if inside(t, "svg") == nil {
			titles = append(titles, t)
		}
	}
	if len(titles) != 1 {
		out = append(out, fmt.Sprintf("document has %d title elements, want 1", len(titles)))
	} else if !isAncestor(head, titles[0]) {
		out = append(out, "title at "+loc(titles[0])+" is outside the head")
	} else if collapse(rawText(titles[0])) == "" {
		out = append(out, "title at "+loc(titles[0])+" is empty")
	}
	if cs := metas("color-scheme"); len(cs) != 1 {
		out = append(out, fmt.Sprintf("document has %d color-scheme meta elements, want 1", len(cs)))
	} else if c, _ := attrOf(cs[0], "content"); !hasFieldFold(c, s.colorMode()) {
		out = append(out, fmt.Sprintf("color-scheme meta element at %s does not list %q", loc(cs[0]), s.colorMode()))
	}
	var icons []ct.Node
	for l := range Elements(root, "link") {
		if rel, _ := attrOf(l, "rel"); hasFieldFold(rel, "icon") {
			icons = append(icons, l)
		}
	}
	if len(icons) != 1 {
		out = append(out, fmt.Sprintf("document has %d icon links, want 1", len(icons)))
	} else {
		href, _ := attrOf(icons[0], "href")
		switch {
		case s.Icon != "" && href != s.Icon:
			out = append(out, fmt.Sprintf("icon link at %s has href %q, want %q", loc(icons[0]), href, s.Icon))
		case s.Icon == "" && parseRef(href).scheme != "data":
			out = append(out, fmt.Sprintf("icon link at %s has href %q, want a data: URL", loc(icons[0]), href))
		}
	}
	for b := range Elements(root, "base") {
		out = append(out, "base element at "+loc(b)+" changes how relative URLs resolve")
	}
	if hc := metas("htmx-config"); len(hc) != 1 {
		out = append(out, fmt.Sprintf("document has %d htmx-config meta elements, want 1", len(hc)))
	} else {
		if !isAncestor(head, hc[0]) {
			out = append(out, "htmx-config meta element at "+loc(hc[0])+" is outside the head")
		}
		got, _ := attrOf(hc[0], "content")
		out = append(out, configDiff(got, s.htmxConfig())...)
	}
	var sheets, scripts []ct.Node
	for e := range Elements(root, "") {
		switch e.Tag() {
		case "link":
			if rel, _ := attrOf(e, "rel"); hasFieldFold(rel, "stylesheet") {
				sheets = append(sheets, e)
			}
		case "script":
			if !jsonDataBlock(e) {
				scripts = append(scripts, e)
			}
		}
	}
	out = append(out, assetList("stylesheet", "href", sheets, s.Stylesheets, head)...)
	out = append(out, assetList("script", "src", scripts, s.Scripts, head)...)
	for i, sc := range scripts {
		t, _ := attrOf(sc, "type")
		module := strings.EqualFold(strings.TrimSpace(t), "module")
		switch {
		case i < len(s.Scripts) && s.Scripts[i].Module && !module:
			out = append(out, fmt.Sprintf("script %s at %s is not type=\"module\"", s.Scripts[i], loc(sc)))
		case i < len(s.Scripts) && !s.Scripts[i].Module && module:
			out = append(out, fmt.Sprintf("script %s at %s is a module, want a classic script", s.Scripts[i], loc(sc)))
		case !module && !hasAttr(sc, "defer"):
			out = append(out, "script at "+loc(sc)+" is not deferred")
		}
		if hasAttr(sc, "async") {
			out = append(out, "script at "+loc(sc)+" is async, so it may run out of order")
		}
	}
	mains := 0
	for e := range Elements(root, "") {
		if roleOf(e) == "main" {
			mains++
		}
	}
	if mains != 1 {
		out = append(out, fmt.Sprintf("document has %d main landmarks, want 1", mains))
	}
	return out
}

// roleOf returns the explicit role of e, or its implicit role.
func roleOf(e ct.Node) string {
	if role, ok := attrOf(e, "role"); ok {
		if f := strings.Fields(role); len(f) > 0 {
			return strings.ToLower(f[0])
		}
	}
	return ct.ImplicitRole(e)
}

// wantAttr describes a missing or different attribute value.
func wantAttr(e ct.Node, name, want string) []string {
	v, ok := attrOf(e, name)
	switch {
	case !ok:
		return []string{fmt.Sprintf("%s element has no %s, want %q", e.Tag(), name, want)}
	case v != want:
		return []string{fmt.Sprintf("%s element has %s %q, want %q", e.Tag(), name, v, want)}
	}
	return nil
}

// assetList compares the stylesheets or scripts of a page with the
// expected assets, in order.
func assetList(kind, attr string, got []ct.Node, want []Asset, head ct.Node) []string {
	var out []string
	for i := 0; i < max(len(got), len(want)); i++ {
		switch {
		case i >= len(got):
			out = append(out, fmt.Sprintf("%s %d (%s) is missing", kind, i+1, want[i]))
			continue
		case i >= len(want):
			u, _ := attrOf(got[i], attr)
			out = append(out, fmt.Sprintf("%s at %s (%s %q) is not part of the shell", kind, loc(got[i]), attr, u))
			continue
		}
		e, a := got[i], want[i]
		if u, _ := attrOf(e, attr); u != a.URL {
			out = append(out, fmt.Sprintf("%s %d at %s has %s %q, want %q (%s)", kind, i+1, loc(e), attr, u, a.URL, a))
			continue
		}
		integrity, ok := attrOf(e, "integrity")
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("%s %s at %s has no integrity", kind, a, loc(e)))
		case a.Integrity != "" && integrity != a.Integrity:
			out = append(out, fmt.Sprintf("%s %s at %s has integrity %q, want %q", kind, a, loc(e), integrity, a.Integrity))
		case a.Integrity == "" && !validIntegrity(integrity):
			out = append(out, fmt.Sprintf("%s %s at %s has no well-formed integrity value", kind, a, loc(e)))
		}
		if !isAncestor(head, e) {
			out = append(out, fmt.Sprintf("%s %s at %s is outside the head", kind, a, loc(e)))
		}
	}
	return out
}

// configDiff compares the htmx config JSON of a page with the expected
// JSON, and names every key that differs.
func configDiff(got, want string) []string {
	var w map[string]any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		return []string{fmt.Sprintf("expected htmx config is not a JSON object: %v", err)}
	}
	var g map[string]any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		return []string{"htmx-config content is not a JSON object"}
	}
	keys := make([]string, 0, len(w)+len(g))
	for k := range w {
		keys = append(keys, k)
	}
	for k := range g {
		if _, ok := w[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		wv, inW := w[k]
		gv, inG := g[k]
		switch {
		case !inG:
			out = append(out, fmt.Sprintf("htmx-config lacks %q, want %s", k, jsonText(wv)))
		case !inW:
			out = append(out, fmt.Sprintf("htmx-config has unexpected %q: %s", k, jsonText(gv)))
		case !reflect.DeepEqual(wv, gv):
			out = append(out, fmt.Sprintf("htmx-config %q is %s, want %s", k, jsonText(gv), jsonText(wv)))
		}
	}
	return out
}

// jsonText encodes a decoded JSON value for a message, shortened.
func jsonText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "?"
	}
	const limit = 80
	if len(b) > limit {
		return string(b[:limit]) + "..."
	}
	return string(b)
}

// ShellContract asserts that page has the shell s ([Shell.Rule]). The
// failure lists every difference.
func ShellContract(page *ct.Subject, s Shell) assertion.Assertion {
	return page.Valid(s.Rule())
}

// PageContract asserts that page has the shell s and satisfies rules, or
// [Rules]() when no rules are given, in one check that lists every
// violation.
func PageContract(page *ct.Subject, s Shell, rules ...ct.Rule) assertion.Assertion {
	if len(rules) == 0 {
		rules = Rules()
	}
	return page.Valid(append([]ct.Rule{s.Rule()}, rules...)...)
}

// FragmentContract asserts that fragment, a response that htmx swaps into a
// page or the data of a server-sent event, is a fragment ([FragmentRule])
// and satisfies rules, or [Rules]() when no rules are given.
func FragmentContract(fragment *ct.Subject, rules ...ct.Rule) assertion.Assertion {
	if len(rules) == 0 {
		rules = Rules()
	}
	return fragment.Valid(append([]ct.Rule{FragmentRule()}, rules...)...)
}

// FragmentRule requires that a subject is a fragment and not a document:
// no doctype and no html, head, body, title, base or meta charset
// elements, which a swap would nest into the page. The HTML parser drops
// some of these tags from a fragment parsed in a body context (as
// cthtml.Fragment and cthttp.Result.HTMLFragment do), so the rule is most
// telling for views.
func FragmentRule() ct.Rule {
	return ct.CustomRule("fragment()", func(root ct.Node) []string {
		var out []string
		for d := range Descendants(root) {
			switch d.Kind() {
			case ct.KindDoctype:
				out = append(out, "doctype at "+loc(d)+" in a fragment")
			case ct.KindElement:
				switch d.Tag() {
				case "html", "head", "body", "base":
					out = append(out, fmt.Sprintf("%s element at %s in a fragment", d.Tag(), loc(d)))
				case "title":
					if inside(d, "svg") == nil {
						out = append(out, "title element at "+loc(d)+" in a fragment")
					}
				case "meta":
					if hasAttr(d, "charset") {
						out = append(out, "meta charset at "+loc(d)+" in a fragment")
					}
				}
			}
		}
		return out
	})
}
