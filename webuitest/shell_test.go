package webuitest_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/meta"
	"github.com/protolambda/chord/html/script"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/webuitest"
)

// docSpec is a page built like page.Document renders it, in parts that
// tests mutate.
type docSpec struct {
	doctype bool
	html    []attr.Node
	head    []elem.Node
	body    []elem.Node
}

// Head positions of validDoc.
const (
	headCharset = iota
	headViewport
	headTitle
	headColorScheme
	headIcon
	headHTMX
	headStylesheets
	headScripts
)

func validDoc(sets ...*asset.Set) docSpec {
	if len(sets) == 0 {
		sets = []*asset.Set{theme.MustAssets(theme.Options{})}
	}
	var sheets, scripts elem.Bundle
	for _, s := range sets {
		sheets = append(sheets, s.Stylesheets())
		scripts = append(scripts, s.Scripts())
	}
	return docSpec{
		doctype: true,
		html:    []attr.Node{attr.Lang("en"), attr.KV("data-bs-theme", theme.ColorMode)},
		head: []elem.Node{
			meta.Meta(meta.Charset("utf-8")),
			meta.Meta(meta.Name("viewport"), meta.Content("width=device-width, initial-scale=1")),
			meta.Title()(text.Text("Tasks")),
			meta.Meta(meta.Name("color-scheme"), meta.Content("dark")),
			meta.Link(meta.Rel("icon"), meta.Href("data:,")),
			hx2.Config{}.Meta(),
			sheets,
			scripts,
		},
		body: []elem.Node{section.Main(attr.ID("main"))(section.H1()(text.Text("Tasks")))},
	}
}

func (d docSpec) node() elem.Node {
	return elem.Bundle{
		elem.If(d.doctype, elem.Doctype()),
		meta.HTML(d.html...)(meta.Head()(d.head...), section.Body()(d.body...)),
	}
}

func (d docSpec) with(f func(*docSpec)) docSpec {
	d.html = slices.Clone(d.html)
	d.head = slices.Clone(d.head)
	d.body = slices.Clone(d.body)
	f(&d)
	return d
}

func TestShellContract(gt *testing.T) {
	t := devtest.ParallelT(gt)
	set := theme.MustAssets(theme.Options{})
	shell := webuitest.DefaultShell(set)
	css := set.URL(theme.BootstrapCSS)
	cases := []struct {
		name   string
		mutate func(*docSpec)
		shell  func(*webuitest.Shell)
		want   []string
		// some only requires a violation with want[0], among others.
		some bool
	}{
		{name: "valid", mutate: func(*docSpec) {}},
		{name: "zero shell fields", mutate: func(*docSpec) {}, shell: func(s *webuitest.Shell) { s.Lang, s.ColorMode, s.HTMXConfig = "", "", "" }},
		{name: "no doctype", mutate: func(d *docSpec) { d.doctype = false }, want: []string{"document has no <!DOCTYPE html> before the html element"}},
		{name: "lang", mutate: func(d *docSpec) { d.html[0] = attr.Lang("de") }, want: []string{`html element has lang "de", want "en"`}},
		{name: "no theme", mutate: func(d *docSpec) { d.html = d.html[:1] }, want: []string{`html element has no data-bs-theme, want "dark"`}},
		{name: "charset not first", mutate: func(d *docSpec) { d.head[0], d.head[1] = d.head[1], d.head[0] }, want: []string{"the first element of the head is not <meta charset>"}},
		{name: "charset", mutate: func(d *docSpec) { d.head[headCharset] = meta.Meta(meta.Charset("latin1")) }, want: []string{`meta charset is "latin1", want "utf-8"`}},
		{name: "two viewports", mutate: func(d *docSpec) { d.head = append(d.head, d.head[headViewport]) }, want: []string{"document has 2 viewport meta elements, want 1"}},
		{name: "viewport", mutate: func(d *docSpec) {
			d.head[headViewport] = meta.Meta(meta.Name("viewport"), meta.Content("initial-scale=1"))
		}, want: []string{"viewport meta element at html[1]/head[0]/meta[1] lacks width=device-width"}},
		{name: "no title", mutate: func(d *docSpec) { d.head[headTitle] = elem.Noop() }, want: []string{"document has 0 title elements, want 1"}},
		{name: "empty title", mutate: func(d *docSpec) { d.head[headTitle] = meta.Title()(text.Text(" ")) }, want: []string{"title at html[1]/head[0]/title[2] is empty"}},
		{name: "title in body", mutate: func(d *docSpec) {
			d.body = append(d.body, d.head[headTitle])
			d.head[headTitle] = elem.Noop()
		}, want: []string{"is outside the head"}},
		{name: "color scheme", mutate: func(d *docSpec) {
			d.head[headColorScheme] = meta.Meta(meta.Name("color-scheme"), meta.Content("light"))
		}, want: []string{`color-scheme meta element at html[1]/head[0]/meta[3] does not list "dark"`}},
		{name: "no color scheme", mutate: func(d *docSpec) { d.head[headColorScheme] = elem.Noop() }, want: []string{"document has 0 color-scheme meta elements"}},
		{name: "favicon request", mutate: func(d *docSpec) {
			d.head[headIcon] = meta.Link(meta.Rel("icon"), meta.Href("/favicon.ico"))
		}, want: []string{`icon link at html[1]/head[0]/link[4] has href "/favicon.ico", want a data: URL`}},
		{name: "no icon", mutate: func(d *docSpec) { d.head[headIcon] = elem.Noop() }, want: []string{"document has 0 icon links, want 1"}},
		{name: "configured icon", mutate: func(d *docSpec) {
			d.head[headIcon] = meta.Link(meta.Rel("shortcut icon"), meta.Href("/i.png"))
		}, shell: func(s *webuitest.Shell) { s.Icon = "/i.png" }},
		{name: "other icon", mutate: func(*docSpec) {}, shell: func(s *webuitest.Shell) { s.Icon = "/i.png" }, want: []string{`has href "data:,", want "/i.png"`}},
		{name: "base", mutate: func(d *docSpec) { d.head = append(d.head, meta.Base(meta.Href("/x/"))) }, want: []string{"base element at html[1]/head[0]/base[11] changes how relative URLs resolve"}},
		{name: "eval", mutate: func(d *docSpec) { d.head[headHTMX] = hx2.Config{AllowEval: true}.Meta() }, want: []string{`htmx-config "allowEval" is true, want false`}},
		{name: "two config changes", mutate: func(d *docSpec) {
			d.head[headHTMX] = hx2.Config{IndicatorStyles: true, HistoryCacheSize: 10}.Meta()
		}, want: []string{`htmx-config "historyCacheSize" is 10, want 0`, `htmx-config "includeIndicatorStyles" is true, want false`}},
		{name: "extra key", mutate: func(d *docSpec) {
			d.head[headHTMX] = hx2.Config{Extra: map[string]any{"timeout": 5}}.Meta()
		}, want: []string{`htmx-config has unexpected "timeout": 5`}},
		{name: "lacks key", mutate: func(d *docSpec) {
			d.head[headHTMX] = meta.Meta(meta.Name("htmx-config"), meta.Content(`{"allowEval":false}`))
		}, want: []string{`htmx-config lacks "allowScriptTags", want false`}, some: true},
		{name: "config not json", mutate: func(d *docSpec) {
			d.head[headHTMX] = meta.Meta(meta.Name("htmx-config"), meta.Content(`allowEval: false`))
		}, want: []string{"htmx-config content is not a JSON object"}},
		{name: "no config", mutate: func(d *docSpec) { d.head[headHTMX] = elem.Noop() }, want: []string{"document has 0 htmx-config meta elements, want 1"}},
		{name: "config in body", mutate: func(d *docSpec) {
			d.body = append(d.body, d.head[headHTMX])
			d.head[headHTMX] = elem.Noop()
		}, want: []string{"htmx-config meta element at html[1]/body[1]/meta[1] is outside the head"}},
		{name: "stylesheet order", mutate: func(d *docSpec) {
			d.head[headStylesheets] = elem.Bundle{set.Stylesheet(theme.ThemeCSS), set.Stylesheet(theme.IconsCSS), set.Stylesheet(theme.BootstrapCSS)}
		}, want: []string{"stylesheet 1 at html[1]/head[0]/link[6] has href", "stylesheet 3 at html[1]/head[0]/link[8] has href"}},
		{name: "missing stylesheet", mutate: func(d *docSpec) {
			d.head[headStylesheets] = elem.Bundle{set.Stylesheet(theme.BootstrapCSS), set.Stylesheet(theme.IconsCSS)}
		}, want: []string{"stylesheet 3 (webui.css) is missing"}},
		{name: "extra stylesheet", mutate: func(d *docSpec) {
			d.head = append(d.head, meta.Link(meta.Rel("stylesheet"), meta.Href("/app.css"), meta.Integrity(sri("x"))))
		}, want: []string{`stylesheet at html[1]/head[0]/link[11] (href "/app.css") is not part of the shell`}},
		{name: "stylesheet integrity", mutate: func(d *docSpec) {
			d.head[headStylesheets] = elem.Bundle{
				meta.Link(meta.Rel("stylesheet"), meta.Href(css), meta.Integrity(sri("other"))),
				set.Stylesheet(theme.IconsCSS), set.Stylesheet(theme.ThemeCSS),
			}
		}, want: []string{"stylesheet " + theme.BootstrapCSS + " at html[1]/head[0]/link[6] has integrity"}},
		{name: "no integrity", mutate: func(d *docSpec) {
			d.head[headStylesheets] = elem.Bundle{meta.Link(meta.Rel("stylesheet"), meta.Href(css)), set.Stylesheet(theme.IconsCSS), set.Stylesheet(theme.ThemeCSS)}
		}, want: []string{"stylesheet " + theme.BootstrapCSS + " at html[1]/head[0]/link[6] has no integrity"}},
		{name: "any integrity", mutate: func(*docSpec) {}, shell: func(s *webuitest.Shell) { s.Stylesheets[0].Integrity = "" }},
		{name: "stylesheet in body", mutate: func(d *docSpec) {
			d.body = append(d.body, d.head[headStylesheets])
			d.head[headStylesheets] = elem.Noop()
		}, want: []string{"stylesheet " + theme.BootstrapCSS + " at html[1]/body[1]/link[1] is outside the head"}, some: true},
		{name: "not deferred", mutate: func(d *docSpec) {
			d.head[headScripts] = elem.Bundle{
				script.Script(script.Src(set.URL(theme.HTMXJS)), script.Integrity(set.Integrity(theme.HTMXJS)))(),
				set.Script(theme.SSEJS),
			}
		}, want: []string{"script at html[1]/head[0]/script[9] is not deferred"}},
		{name: "async", mutate: func(d *docSpec) {
			d.head[headScripts] = elem.Bundle{
				script.Script(script.Src(set.URL(theme.HTMXJS)), script.Defer(), script.Async(), script.Integrity(set.Integrity(theme.HTMXJS)))(),
				set.Script(theme.SSEJS),
			}
		}, want: []string{"script at html[1]/head[0]/script[9] is async, so it may run out of order"}},
		{name: "inline script", mutate: func(d *docSpec) { d.head = append(d.head, script.Inline("x()")) }, want: []string{`script at html[1]/head[0]/script[11] (src "") is not part of the shell`, "is not deferred"}},
		{name: "json block", mutate: func(d *docSpec) { d.body = append(d.body, script.Inline(`{"a":1}`, script.Type("application/json"))) }},
		{name: "no main", mutate: func(d *docSpec) { d.body = []elem.Node{section.Section()()} }, want: []string{"document has 0 main landmarks, want 1"}},
		{name: "two mains", mutate: func(d *docSpec) {
			d.body = append(d.body, section.Section(attr.KV("role", "main"))())
		}, want: []string{"document has 2 main landmarks, want 1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			s := shell
			s.Stylesheets = slices.Clone(shell.Stylesheets)
			s.Scripts = slices.Clone(shell.Scripts)
			if c.shell != nil {
				c.shell(&s)
			}
			doc := validDoc(set).with(c.mutate)
			got := violations(t, ct.View(doc.node()), s.Rule())
			if c.some {
				t.Must(be.NonEmpty(got))
				t.Must(be.Substring(strings.Join(got, "\n"), c.want[0]))
				return
			}
			t.Mustf(be.Len(got, len(c.want)), "violations %q", got)
			for i, w := range c.want {
				t.Must(be.Substring(got[i], w))
			}
		})
	}
}

// TestShellContractParsed checks the rendered and parsed page: the shell
// reads the same from HTML as from the view.
func TestShellContractParsed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var b strings.Builder
	t.Must(be.NoError(core.Render(t.Context(), validDoc().node(), &b)))
	page := cthtml.PageString(b.String())
	t.Must(webuitest.ShellContract(page, webuitest.DefaultShell()))
	t.Must(webuitest.PageContract(page, webuitest.DefaultShell()))

	broken := cthtml.PageString(strings.Replace(b.String(), `lang="en"`, `lang="fr"`, 1))
	err := webuitest.ShellContract(broken, webuitest.DefaultShell()).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrRule))
	t.Must(be.ErrorContains(err, `shell(): html element has lang "fr", want "en"`))
}

// A Shell takes the color mode of a page.Document as it is: a light page
// passes with page.ColorModeLight, and fails against the default shell.
func TestShellColorModeOfDocument(gt *testing.T) {
	t := devtest.ParallelT(gt)
	doc := ct.View(page.Document{
		Title:     "Notes",
		ColorMode: page.ColorModeLight,
		Assets:    []*asset.Set{theme.MustAssets(theme.Options{})},
		Body:      section.Main()(text.Text("x")),
	})
	shell := webuitest.DefaultShell()
	shell.ColorMode = page.ColorModeLight
	t.Must(webuitest.ShellContract(doc, shell))
	err := webuitest.ShellContract(doc, webuitest.DefaultShell()).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrRule))
	t.Must(be.ErrorContains(err, `data-bs-theme`))
}

func TestDefaultShell(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := webuitest.DefaultShell()
	t.Must(be.Equal("en", s.Lang))
	t.Must(be.Equal(theme.ColorMode, s.ColorMode))
	t.Must(be.Equal(webuitest.DefaultHTMXConfig(), s.HTMXConfig))
	want, err := hx2.Config{}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Equal(want, s.HTMXConfig))
	var names []string
	for _, a := range s.Stylesheets {
		names = append(names, a.Name)
	}
	t.Must(be.DeepEqual([]string{theme.BootstrapCSS, theme.IconsCSS, theme.ThemeCSS}, names))
	names = nil
	for _, a := range s.Scripts {
		names = append(names, a.String())
	}
	t.Must(be.DeepEqual([]string{theme.HTMXJS, theme.SSEJS}, names))
	t.Must(be.Equal(theme.BootstrapCSSIntegrity, s.Stylesheets[0].Integrity))
	t.Must(be.Equal(theme.HTMXJSIntegrity, s.Scripts[0].Integrity))
	t.Must(be.Equal("/assets/webui/", s.Scripts[0].URL[:len("/assets/webui/")]))

	app := asset.Must(asset.New(asset.Options{Prefix: "/assets/app/", FS: fstest.MapFS{
		"app.css":  {Data: []byte("p{}")},
		"app.js":   {Data: []byte("1")},
		"logo.svg": {Data: []byte("<svg/>")},
	}, Files: []asset.File{{Name: "app.js"}, {Name: "app.css"}, {Name: "logo.svg"}}}))
	both := webuitest.DefaultShell(theme.MustAssets(theme.Options{WithoutIcons: true}), app)
	t.Must(be.Len(both.Stylesheets, 3))
	t.Must(be.Equal("app.css", both.Stylesheets[2].Name))
	t.Must(be.Len(both.Scripts, 3))
	t.Must(be.Equal(app.URL("app.js"), both.Scripts[2].URL))
	t.Must(be.Equal(app.Integrity("app.js"), both.Scripts[2].Integrity))
	t.Must(webuitest.ShellContract(ct.View(validDoc(theme.MustAssets(theme.Options{WithoutIcons: true}), app).node()), both))
	t.Must(be.Panic(func() { webuitest.DefaultShell(nil) }))
	t.Must(be.Equal("/x.js", webuitest.Asset{URL: "/x.js"}.String()))

	t.Run("unlisted files and modules", func(t devtest.T) {
		set := asset.Must(asset.New(asset.Options{Prefix: "/assets/app/", FS: fstest.MapFS{
			"app.css":    {Data: []byte("p{}")},
			"print.css":  {Data: []byte("p{}")},
			"main.mjs":   {Data: []byte("export {};")},
			"worker.js":  {Data: []byte("1")},
			"classic.js": {Data: []byte("1")},
		}, Files: []asset.File{
			{Name: "app.css"},
			{Name: "print.css", Unlisted: true},
			{Name: "main.mjs", Kind: asset.KindModule, ContentType: "text/javascript"},
			{Name: "worker.js", Unlisted: true},
			{Name: "classic.js"},
		}}))
		s := webuitest.DefaultShell(set)
		t.Must(be.DeepEqual([]webuitest.Asset{{Name: "app.css", URL: set.URL("app.css"), Integrity: set.Integrity("app.css")}}, s.Stylesheets))
		t.Must(be.DeepEqual([]webuitest.Asset{
			{Name: "main.mjs", URL: set.URL("main.mjs"), Integrity: set.Integrity("main.mjs"), Module: true},
			{Name: "classic.js", URL: set.URL("classic.js"), Integrity: set.Integrity("classic.js")},
		}, s.Scripts))
		// The head of a document that loads the set, as page.Document
		// renders it.
		head := func(scripts ...elem.Node) elem.Node {
			return validDoc(set).with(func(d *docSpec) {
				d.head[headScripts] = elem.Bundle(scripts)
			}).node()
		}
		t.Must(webuitest.ShellContract(ct.View(head(set.Scripts())), s))
		// A module loaded as a classic script, and the reverse, differ.
		swapped := head(
			script.Script(script.Src(set.URL("main.mjs")), script.Defer(), script.Integrity(set.Integrity("main.mjs")))(),
			script.Script(script.Type("module"), script.Src(set.URL("classic.js")), script.Integrity(set.Integrity("classic.js")))(),
		)
		err := webuitest.ShellContract(ct.View(swapped), s).Check(t.Context())
		t.Must(be.ErrorContains(err, `script main.mjs at html[1]/head[0]/script[`))
		t.Must(be.ErrorContains(err, `is not type="module"`))
		t.Must(be.ErrorContains(err, "script classic.js at"))
		t.Must(be.ErrorContains(err, "is a module, want a classic script"))
		// An unlisted file that a page loads anyway is not part of the shell.
		extra := head(set.Scripts(), set.Script("worker.js"))
		err = webuitest.ShellContract(ct.View(extra), s).Check(t.Context())
		t.Must(be.ErrorContains(err, "is not part of the shell"))
	})
}

func TestPageContract(gt *testing.T) {
	t := devtest.ParallelT(gt)
	shell := webuitest.DefaultShell()
	doc := validDoc().with(func(d *docSpec) {
		d.doctype = false
		d.body = append(d.body, text.A(text.Href("javascript:x()"))(text.Text("x")))
	})
	err := webuitest.PageContract(ct.View(doc.node()), shell).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrRule))
	t.Must(be.ErrorContains(err, "shell(): document has no <!DOCTYPE html>"))
	t.Must(be.ErrorContains(err, "safeURLs(): href at"))
	// Explicit rules replace the default bundle.
	err = webuitest.PageContract(ct.View(doc.node()), shell, ct.UniqueIDs()).Check(t.Context())
	t.Must(be.ErrorContains(err, "shell()"))
	t.Must(be.False(strings.Contains(err.Error(), "safeURLs")))
	// A subject that does not load is a load failure, never a pass.
	failing := ct.View(elem.Fn(func(ctx context.Context) (elem.Node, error) { return nil, errors.New("boom") }))
	err = webuitest.PageContract(failing, shell).Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrLoad))
}

func TestFragmentContract(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(webuitest.FragmentContract(cthtml.FragmentString("div", `<p id="x">ok</p><button type="button" hx-get="/a" hx-target="#alerts">a</button>`)))
	t.Must(webuitest.FragmentContract(ct.View(section.Section()(text.P()(text.Text("ok"))))))
	cases := []struct {
		name string
		node elem.Node
		want string
	}{
		{"html", meta.HTML()(), "html element at html[0] in a fragment"},
		{"doctype", elem.Doctype(), "doctype at [0] in a fragment"},
		{"head", meta.Head()(), "head element at head[0] in a fragment"},
		{"body", section.Body()(), "body element at body[0] in a fragment"},
		{"title", meta.Title()(text.Text("x")), "title element at title[0] in a fragment"},
		{"charset", meta.Meta(meta.Charset("utf-8")), "meta charset at meta[0] in a fragment"},
		{"base", meta.Base(meta.Href("/")), "base element at base[0] in a fragment"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			err := webuitest.FragmentContract(ct.View(c.node), ct.UniqueIDs()).Check(t.Context())
			t.Must(be.ErrorContains(err, c.want))
		})
	}
	got := violations(t, ct.View(section.Section()()), webuitest.FragmentRule())
	t.Must(be.Empty(got))
}
