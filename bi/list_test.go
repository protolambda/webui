package bi_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/bi"
)

// themeFiles is the directory of the pinned browser assets of the theme
// package, relative to this package.
const themeFiles = "../theme/files/"

func iconsCSS(t devtest.T) []byte {
	t.Helper()
	css, err := os.ReadFile(themeFiles + "bootstrap-icons-" + bi.Version + ".min.css")
	t.Must(be.NoError(err))
	return css
}

// stylesheetNames extracts the icon names from the stylesheet, in order. It
// is deliberately independent of the generator.
func stylesheetNames(t devtest.T, css []byte) []string {
	t.Helper()
	re := regexp.MustCompile(`\.bi-([a-z0-9-]+)::before\{content:"\\f[0-9a-f]+"\}`)
	var names []string
	for _, m := range re.FindAllSubmatch(css, -1) {
		names = append(names, string(m[1]))
	}
	t.Must(be.Equal(bytes.Count(css, []byte(".bi-")), len(names)))
	return names
}

// camel is the documented naming rule of the constants.
func camel(name string) string {
	var b strings.Builder
	if name[0] >= '0' && name[0] <= '9' {
		b.WriteString("Num")
	}
	for _, part := range strings.Split(name, "-") {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// declaredConstants returns the Icon constants of list.go, in declaration
// order, as name and value.
func declaredConstants(t devtest.T) (idents, values []string) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "list.go", nil, parser.SkipObjectResolution)
	t.Must(be.NoError(err))
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			if typ, ok := vs.Type.(*ast.Ident); !ok || typ.Name != "Icon" {
				continue
			}
			t.Must(be.Equal(len(vs.Names), len(vs.Values)))
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				t.Must(be.True(ok))
				v, err := strconv.Unquote(lit.Value)
				t.Must(be.NoError(err))
				idents = append(idents, name.Name)
				values = append(values, v)
			}
		}
	}
	return idents, values
}

func TestConstantsMatchStylesheet(gt *testing.T) {
	t := devtest.ParallelT(gt)
	want := stylesheetNames(t, iconsCSS(t))
	t.Must(be.Equal(2078, len(want)))

	idents, values := declaredConstants(t)
	t.Must(be.DeepEqual(want, values))
	for i, ident := range idents {
		t.Mustf(be.Equal(camel(values[i]), ident), "constant for %q", values[i])
	}

	var all []string
	for ic := range bi.All() {
		all = append(all, string(ic))
	}
	t.Must(be.DeepEqual(want, all))

	sorted := slices.Clone(want)
	slices.Sort(sorted)
	t.Must(be.Equal(len(want), len(slices.Compact(sorted))))
}

func TestVersionMatchesStylesheet(gt *testing.T) {
	t := devtest.ParallelT(gt)
	css := iconsCSS(t)
	t.Must(be.Prefix(string(css), "/*!\n * Bootstrap Icons v"+bi.Version+" "))
}

func TestStylesheetFontExists(gt *testing.T) {
	t := devtest.ParallelT(gt)
	css := iconsCSS(t)
	m := regexp.MustCompile(`url\("([^"?]+\.woff2)(\?[^"]*)?"\)`).FindSubmatch(css)
	t.Must(be.NonNil(m))
	ref := string(m[1])
	t.Must(be.Equal("fonts/bootstrap-icons.woff2", ref))
	t.Must(be.False(path.IsAbs(ref) || strings.Contains(ref, "..") || strings.Contains(ref, "://")))

	font, err := os.ReadFile(themeFiles + ref)
	t.Must(be.NoError(err))
	t.Must(be.Prefix(string(font), "wOF2"))
}
