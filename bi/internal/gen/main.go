// Command gen generates list.go of package bi from the Bootstrap Icons
// stylesheet.
//
// It runs from the bi package directory, through the go:generate directive
// in bi/doc.go:
//
//	go generate ./bi
//
// gen reads the stylesheet (by default the copy pinned by the theme package),
// takes the release version from its banner comment and the icon names from
// its ".bi-NAME::before" rules, in stylesheet order, and writes list.go with
// the Version constant, one Icon constant per icon, and the tables behind
// bi.All and bi.Lookup. The output is gofmt-formatted and depends only on the
// stylesheet bytes, so that a test can check that list.go is up to date.
//
// Constant names are the icon names in CamelCase: every hyphen-separated part
// starts with an upper-case letter, and a name that starts with a digit gets
// the prefix "Num" ("x-lg" is XLg, "1-circle" is Num1Circle). gen fails,
// rather than writing a partial file, on a stylesheet it does not understand:
// a missing banner, an icon rule of an unexpected shape, an invalid or
// duplicate name, two names that map to the same constant, or a name that
// maps to another exported identifier of package bi.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// defaultCSS is the stylesheet path relative to the bi package directory.
const defaultCSS = "../theme/files/bootstrap-icons-1.13.1.min.css"

// maxCSSSize bounds the stylesheet that gen reads; the 1.13.1 file is about
// 85 KiB.
const maxCSSSize = 4 << 20

// reserved are the exported names of package bi that are not icons. An icon
// that maps to one of them would break the package.
var reserved = map[string]bool{
	"Icon":          true,
	"Version":       true,
	"All":           true,
	"Lookup":        true,
	"ErrEmptyLabel": true,
}

var (
	// bannerRe matches the release in the leading banner comment.
	bannerRe = regexp.MustCompile(`\A/\*!\s*\*?\s*Bootstrap Icons v([0-9]+\.[0-9]+\.[0-9]+)\s`)
	// ruleRe matches one icon rule, e.g. .bi-alarm::before{content:"\f102"}.
	ruleRe = regexp.MustCompile(`\.bi-([^\s:{},.\[\]()"']*)::before\s*\{\s*content:\s*"\\[0-9a-f]+"\s*;?\s*\}`)
	// nameRe is the syntax of an icon name.
	nameRe = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
)

func main() {
	cssPath := flag.String("css", defaultCSS, "path of the Bootstrap Icons stylesheet")
	outPath := flag.String("out", "list.go", "path of the Go file to write")
	flag.Parse()
	if err := run(*cssPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

// run generates outPath from the stylesheet at cssPath.
func run(cssPath, outPath string) error {
	css, err := readCSS(cssPath)
	if err != nil {
		return err
	}
	src, err := generate(css)
	if err != nil {
		return fmt.Errorf("generate from %s: %w", cssPath, err)
	}
	if err := os.WriteFile(outPath, src, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// readCSS reads the stylesheet, refusing files larger than maxCSSSize.
func readCSS(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stylesheet: %w", err)
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(f, maxCSSSize+1)); err != nil {
		return nil, fmt.Errorf("read stylesheet: %w", err)
	}
	if buf.Len() > maxCSSSize {
		return nil, fmt.Errorf("stylesheet %s is larger than %d bytes", path, maxCSSSize)
	}
	return buf.Bytes(), nil
}

// icon is one icon of the stylesheet.
type icon struct {
	ident string // Go constant name
	name  string // icon name, without the "bi-" prefix
}

// parse extracts the release version and the icons, in stylesheet order.
func parse(css []byte) (version string, icons []icon, err error) {
	m := bannerRe.FindSubmatch(css)
	if m == nil {
		return "", nil, errors.New(`no "Bootstrap Icons vX.Y.Z" banner at the start of the stylesheet`)
	}
	version = string(m[1])

	rules := ruleRe.FindAllSubmatch(css, -1)
	if n := bytes.Count(css, []byte(".bi-")); n != len(rules) {
		return "", nil, fmt.Errorf("found %d icon rules of the expected shape, but %d \".bi-\" selectors", len(rules), n)
	}
	if len(rules) == 0 {
		return "", nil, errors.New("no icon rules")
	}
	if len(rules) > math.MaxUint16 {
		return "", nil, fmt.Errorf("%d icons do not fit the uint16 name index", len(rules))
	}
	names := make(map[string]bool, len(rules))
	idents := make(map[string]string, len(rules))
	icons = make([]icon, 0, len(rules))
	for _, r := range rules {
		name := string(r[1])
		if !nameRe.MatchString(name) {
			return "", nil, fmt.Errorf("invalid icon name %q", name)
		}
		if names[name] {
			return "", nil, fmt.Errorf("duplicate icon name %q", name)
		}
		names[name] = true
		ident := identifier(name)
		if !token.IsIdentifier(ident) || !token.IsExported(ident) {
			return "", nil, fmt.Errorf("icon %q maps to invalid constant name %q", name, ident)
		}
		if reserved[ident] {
			return "", nil, fmt.Errorf("icon %q maps to reserved name %q", name, ident)
		}
		if other, ok := idents[ident]; ok {
			return "", nil, fmt.Errorf("icons %q and %q both map to constant %s", other, name, ident)
		}
		idents[ident] = name
		icons = append(icons, icon{ident: ident, name: name})
	}
	return version, icons, nil
}

// identifier returns the Go constant name of a valid icon name.
func identifier(name string) string {
	var b strings.Builder
	if name != "" && name[0] >= '0' && name[0] <= '9' {
		b.WriteString("Num")
	}
	for part := range strings.SplitSeq(name, "-") {
		if part == "" {
			continue
		}
		c := part[0]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b.WriteByte(c)
		b.WriteString(part[1:])
	}
	return b.String()
}

// generate returns the gofmt-formatted source of list.go.
func generate(css []byte) ([]byte, error) {
	version, icons, err := parse(css)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.WriteString("// Code generated by \"go run ./internal/gen\"; DO NOT EDIT.\n\n")
	b.WriteString("package bi\n\n")

	b.WriteString("// Version is the Bootstrap Icons release that the icon constants were\n")
	b.WriteString("// generated from. The icons stylesheet and font served with the page must be\n")
	b.WriteString("// of the same release.\n")
	fmt.Fprintf(&b, "const Version = %s\n\n", strconv.Quote(version))

	fmt.Fprintf(&b, "// The %d icons of Bootstrap Icons [Version], in stylesheet order.\n", len(icons))
	b.WriteString("//\n")
	b.WriteString("// Each constant is the icon name in CamelCase, with the prefix Num when the\n")
	b.WriteString("// name starts with a digit: \"x-lg\" is [XLg] and \"1-circle\" is [Num1Circle].\n")
	b.WriteString("const (\n")
	for _, ic := range icons {
		fmt.Fprintf(&b, "\t%s Icon = %s\n", ic.ident, strconv.Quote(ic.name))
	}
	b.WriteString(")\n\n")

	b.WriteString("// all holds every icon constant in stylesheet order, for All.\n")
	b.WriteString("var all = [...]Icon{\n")
	for _, ic := range icons {
		fmt.Fprintf(&b, "\t%s,\n", ic.ident)
	}
	b.WriteString("}\n\n")

	order := make([]uint16, len(icons))
	for i := range order {
		order[i] = uint16(i)
	}
	slices.SortFunc(order, func(x, y uint16) int {
		return strings.Compare(icons[x].name, icons[y].name)
	})
	b.WriteString("// byName holds the indexes into all in ascending name order, for Lookup.\n")
	b.WriteString("var byName = [...]uint16{")
	for i, idx := range order {
		if i%16 == 0 {
			b.WriteString("\n\t")
		} else {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d,", idx)
	}
	b.WriteString("\n}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format output: %w", err)
	}
	return src, nil
}
