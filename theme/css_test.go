package theme_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// The design tests of webui.css. They read the stylesheets from disk rather than through the theme package, so they
// check the stylesheet on its own terms: what it may select, read and declare, and the rules that other packages (the
// dialog, accordion and dropdown components, the htmx configuration) rely on.

const (
	themeFile     = "files/webui.css"
	bootstrapFile = "files/bootstrap-5.3.8.min.css"

	// behaviorMarker starts the part of webui.css that applies in every color mode.
	behaviorMarker = "---- Behavior"
)

// htmxClasses are the classes htmx itself puts on elements during a request and a swap.
var htmxClasses = []string{"htmx-added", "htmx-indicator", "htmx-request", "htmx-settling", "htmx-swapping"}

// The scopes of the theme part of webui.css. Each is anchored on the root element in the dark color mode, so that
// Bootstrap's per-component color modes on a light page (a navbar marked data-bs-theme="dark") stay stock Bootstrap.
const (
	// darkVars is the selector of the rules that declare variables: the dark root, and again every element marked
	// dark inside it, as Bootstrap's own [data-bs-theme=dark] block does. It is as specific as one attribute, so that
	// a page's [data-bs-theme="dark"] { --webui-...: ... } linked later overrides it.
	darkVars = `[data-bs-theme="dark"]:where(:root, :root[data-bs-theme="dark"] *)`
	// darkRoot is the selector of the rules on the root element itself.
	darkRoot = `html[data-bs-theme="dark"]`
	// darkScope prefixes the other theme selectors: as specific as one attribute, so that it beats Bootstrap's
	// rule for the same classes.
	darkScope = `[data-bs-theme="dark"]:where(:root) `
	// darkDefault prefixes the theme selectors of defaults, which add no specificity (see "Specificity" in webui.css).
	darkDefault = `:where(:root[data-bs-theme="dark"]) `
)

// allowedAtRules are the at-rules webui.css may use.
var allowedAtRules = []string{"media", "supports"}

// forbiddenConstructs may not appear in webui.css outside comments: each would fetch something past the integrity
// check (or at all), run script, or reorder the cascade against Bootstrap.
var forbiddenConstructs = []string{
	"@import", "@layer", "@font-face", "@namespace",
	"url(", "image-set(", "gradient(", "expression(", "javascript:",
}

// sheet is a parsed stylesheet: its style rules in source order, those nested in grouping at-rules included, and the
// names of all its at-rules. The parser is small and strict, enough for a hand-written stylesheet and Bootstrap's
// minified one; it rejects what it does not understand rather than skip it.
type sheet struct {
	rules   []rule
	atRules []string
}

type rule struct {
	selectors []string // whitespace-normalized
	decls     []decl
	offset    int // byte offset of the rule in the source
}

type decl struct {
	prop      string // lower case, except for custom properties, which are case-sensitive
	value     string // without !important
	important bool
}

// String renders the value as written, !important included.
func (d decl) String() string {
	if d.important {
		return d.value + " !important"
	}
	return d.value
}

var (
	errUnterminated = errors.New("unterminated")
	errUnbalanced   = errors.New("unbalanced")
	errSyntax       = errors.New("syntax")
)

// groupingAtRules hold style rules in their blocks; the blocks of other at-rules (keyframes, font-face) are skipped.
var groupingAtRules = []string{"media", "supports", "container", "layer", "scope", "document"}

var importantSuffix = regexp.MustCompile(`(?i)\s*!\s*important$`)

func parseSheet(src string) (*sheet, error) {
	s, err := blankComments(src)
	if err != nil {
		return nil, err
	}
	sh := new(sheet)
	if err := sh.parseRules(s, 0); err != nil {
		return nil, err
	}
	return sh, nil
}

// blankComments replaces each comment with spaces, so that the offsets in the result are those of the source.
func blankComments(src string) (string, error) {
	b := []byte(src)
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == '"' || b[i] == '\'':
			j := closeQuote(src, i)
			if j < 0 {
				return "", fmt.Errorf("%w string at byte %d", errUnterminated, i)
			}
			i = j
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return "", fmt.Errorf("%w comment at byte %d", errUnterminated, i)
			}
			end += i + 4
			for k := i; k < end; k++ {
				b[k] = ' '
			}
			i = end - 1
		}
	}
	return string(b), nil
}

// closeQuote returns the index of the quote that closes the string starting at s[i], or -1.
func closeQuote(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j
		case '\n':
			return -1
		}
	}
	return -1
}

// scan returns the index of the first byte of stops in s[i:] that is outside strings and brackets, or len(s).
func scan(s string, i int, stops string) (int, error) {
	var open []byte
	for ; i < len(s); i++ {
		c := s[i]
		if len(open) == 0 && strings.IndexByte(stops, c) >= 0 {
			return i, nil
		}
		switch c {
		case '\\':
			i++
		case '"', '\'':
			j := closeQuote(s, i)
			if j < 0 {
				return 0, fmt.Errorf("%w string at byte %d", errUnterminated, i)
			}
			i = j
		case '(':
			open = append(open, ')')
		case '[':
			open = append(open, ']')
		case '{':
			open = append(open, '}')
		case ')', ']', '}':
			if len(open) == 0 || open[len(open)-1] != c {
				return 0, fmt.Errorf("%w %q at byte %d", errUnbalanced, c, i)
			}
			open = open[:len(open)-1]
		}
	}
	if len(open) > 0 {
		return 0, fmt.Errorf("%w: %q missing at the end", errUnbalanced, open[len(open)-1])
	}
	return len(s), nil
}

// split splits s at each sep outside strings and brackets.
func split(s string, sep byte) ([]string, error) {
	var parts []string
	for i := 0; ; {
		j, err := scan(s, i, string(sep))
		if err != nil {
			return nil, err
		}
		parts = append(parts, s[i:j])
		if j == len(s) {
			return parts, nil
		}
		i = j + 1
	}
}

// parseRules parses the rules in s, which starts at byte base of the source.
func (sh *sheet) parseRules(s string, base int) error {
	for i := 0; ; {
		for i < len(s) && strings.IndexByte(" \t\r\n\f", s[i]) >= 0 {
			i++
		}
		if i == len(s) {
			return nil
		}
		j, err := scan(s, i, "{;}")
		if err != nil {
			return fmt.Errorf("rule at byte %d: %w", base+i, err)
		}
		prelude := strings.TrimSpace(s[i:j])
		name, isAt := atRuleName(prelude)
		switch {
		case j == len(s):
			return fmt.Errorf("%w rule at byte %d", errUnterminated, base+i)
		case s[j] == '}':
			return fmt.Errorf("%w '}' at byte %d", errUnbalanced, base+j)
		case s[j] == ';':
			if !isAt {
				return fmt.Errorf("%w: declaration outside a rule at byte %d", errSyntax, base+i)
			}
			sh.atRules = append(sh.atRules, name)
			i = j + 1
			continue
		}
		k, err := scan(s, j+1, "}")
		if err != nil {
			return fmt.Errorf("block at byte %d: %w", base+j, err)
		}
		if k == len(s) {
			return fmt.Errorf("%w block at byte %d", errUnterminated, base+j)
		}
		body := s[j+1 : k]
		if isAt {
			sh.atRules = append(sh.atRules, name)
			if slices.Contains(groupingAtRules, name) {
				if err := sh.parseRules(body, base+j+1); err != nil {
					return err
				}
			}
		} else {
			r, err := parseRule(prelude, body, base+i)
			if err != nil {
				return err
			}
			sh.rules = append(sh.rules, r)
		}
		i = k + 1
	}
}

// atRuleName returns the lower-case name of the at-rule that prelude starts, if it starts one.
func atRuleName(prelude string) (string, bool) {
	if !strings.HasPrefix(prelude, "@") {
		return "", false
	}
	end := strings.IndexFunc(prelude[1:], func(r rune) bool {
		return !(r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if end < 0 {
		end = len(prelude) - 1
	}
	return strings.ToLower(prelude[1 : 1+end]), true
}

func parseRule(prelude, body string, offset int) (rule, error) {
	r := rule{offset: offset}
	sels, err := split(prelude, ',')
	if err != nil {
		return r, fmt.Errorf("selector at byte %d: %w", offset, err)
	}
	for _, sel := range sels {
		sel = strings.Join(strings.Fields(sel), " ")
		if sel == "" {
			return r, fmt.Errorf("%w: empty selector at byte %d", errSyntax, offset)
		}
		r.selectors = append(r.selectors, sel)
	}
	parts, err := split(body, ';')
	if err != nil {
		return r, fmt.Errorf("declarations at byte %d: %w", offset, err)
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if b, err := scan(part, 0, "{"); err != nil || b != len(part) {
			return r, fmt.Errorf("%w: nested block in the rule at byte %d", errSyntax, offset)
		}
		c, err := scan(part, 0, ":")
		if err != nil || c == len(part) {
			return r, fmt.Errorf("%w: declaration without a colon in the rule at byte %d", errSyntax, offset)
		}
		d := decl{prop: strings.TrimSpace(part[:c]), value: strings.TrimSpace(part[c+1:])}
		if !strings.HasPrefix(d.prop, "--") {
			d.prop = strings.ToLower(d.prop)
		}
		if loc := importantSuffix.FindStringIndex(d.value); loc != nil {
			d.value, d.important = strings.TrimSpace(d.value[:loc[0]]), true
		}
		if d.prop == "" {
			return r, fmt.Errorf("%w: declaration without a property in the rule at byte %d", errSyntax, offset)
		}
		r.decls = append(r.decls, d)
	}
	return r, nil
}

var (
	classPattern = regexp.MustCompile(`\.(-?[_a-zA-Z][_a-zA-Z0-9-]*)`)
	attrPattern  = regexp.MustCompile(`\[[^\]]*\]`)
	varPattern   = regexp.MustCompile(`var\(\s*(--[_a-zA-Z0-9-]+)`)
)

// withoutAttributes drops the attribute selectors from sel, since their values may hold dots and hashes.
func withoutAttributes(sel string) string {
	return attrPattern.ReplaceAllString(sel, "[]")
}

// classes returns the set of classes that the selectors of sh name.
func (sh *sheet) classes() map[string]bool {
	out := make(map[string]bool)
	for _, r := range sh.rules {
		for _, sel := range r.selectors {
			for _, m := range classPattern.FindAllStringSubmatch(withoutAttributes(sel), -1) {
				out[m[1]] = true
			}
		}
	}
	return out
}

// declared returns the values of each custom property that sh declares, in source order.
func (sh *sheet) declared() map[string][]string {
	out := make(map[string][]string)
	for _, r := range sh.rules {
		for _, d := range r.decls {
			if strings.HasPrefix(d.prop, "--") {
				out[d.prop] = append(out[d.prop], d.value)
			}
		}
	}
	return out
}

// reads returns the set of custom properties that sh reads with var().
func (sh *sheet) reads() map[string]bool {
	out := make(map[string]bool)
	for _, r := range sh.rules {
		for _, d := range r.decls {
			for _, m := range varPattern.FindAllStringSubmatch(d.value, -1) {
				out[m[1]] = true
			}
		}
	}
	return out
}

// lookup returns the value that the last declaration of prop in a rule with the selector sel has.
func (sh *sheet) lookup(sel, prop string) (string, bool) {
	var value string
	found := false
	for _, r := range sh.rules {
		if !slices.Contains(r.selectors, sel) {
			continue
		}
		for _, d := range r.decls {
			if d.prop == prop {
				value, found = d.String(), true
			}
		}
	}
	return value, found
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// unknownClasses lists the classes that sh selects and that are neither in base nor htmx's.
func unknownClasses(sh, base *sheet) []string {
	known := base.classes()
	var out []string
	for _, c := range sortedKeys(sh.classes()) {
		if !known[c] && !slices.Contains(htmxClasses, c) {
			out = append(out, c)
		}
	}
	return out
}

// idSelectors lists the selectors of sh that name an id, which would tie the theme to one page.
func idSelectors(sh *sheet) []string {
	var out []string
	for _, r := range sh.rules {
		for _, sel := range r.selectors {
			if strings.Contains(withoutAttributes(sel), "#") {
				out = append(out, sel)
			}
		}
	}
	return out
}

// undefinedReads lists the custom properties that sh reads and that neither sh nor base declares.
func undefinedReads(sh, base *sheet) []string {
	own, theirs := sh.declared(), base.declared()
	var out []string
	for _, p := range sortedKeys(sh.reads()) {
		if own[p] == nil && theirs[p] == nil {
			out = append(out, p)
		}
	}
	return out
}

// unusedTokens lists the --webui- tokens that sh declares and never reads.
func unusedTokens(sh *sheet) []string {
	reads := sh.reads()
	var out []string
	for _, p := range sortedKeys(sh.declared()) {
		if strings.HasPrefix(p, "--webui-") && !reads[p] {
			out = append(out, p)
		}
	}
	return out
}

// redeclaredTokens lists the --webui- tokens that sh declares more than once, so that each has one default.
func redeclaredTokens(sh *sheet) []string {
	var out []string
	for p, values := range sh.declared() {
		if strings.HasPrefix(p, "--webui-") && len(values) > 1 {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

// foreignProperties lists the custom properties that sh declares and that are neither --webui- tokens nor --bs-
// properties of base: a misspelled Bootstrap property would have no effect.
func foreignProperties(sh, base *sheet) []string {
	theirs := base.declared()
	var out []string
	for _, p := range sortedKeys(sh.declared()) {
		switch {
		case strings.HasPrefix(p, "--webui-"):
		case strings.HasPrefix(p, "--bs-") && theirs[p] != nil:
		default:
			out = append(out, p)
		}
	}
	return out
}

// shadows lists the declarations of sh that draw a shadow: a shadow property with a value other than none, or a
// drop-shadow() filter.
func shadows(sh *sheet) []string {
	var out []string
	for _, r := range sh.rules {
		for _, d := range r.decls {
			if strings.Contains(d.prop, "shadow") && !strings.EqualFold(d.value, "none") ||
				strings.Contains(strings.ToLower(d.value), "shadow(") {
				out = append(out, d.prop+": "+d.String())
			}
		}
	}
	return out
}

// emptyOrDuplicated lists the rules without declarations and the properties declared twice in one rule.
func emptyOrDuplicated(sh *sheet) []string {
	var out []string
	for _, r := range sh.rules {
		if len(r.decls) == 0 {
			out = append(out, "empty: "+strings.Join(r.selectors, ", "))
		}
		seen := make(map[string]bool)
		for _, d := range r.decls {
			if seen[d.prop] {
				out = append(out, "duplicate "+d.prop+": "+strings.Join(r.selectors, ", "))
			}
			seen[d.prop] = true
		}
	}
	return out
}

// misscoped lists the selectors outside the dark scopes in the theme part of sh (before the byte offset marker), and
// those that name a color mode in the behavior part (from marker on).
func misscoped(sh *sheet, marker int) []string {
	var out []string
	for _, r := range sh.rules {
		for _, sel := range r.selectors {
			if r.offset < marker {
				scoped := sel == darkVars || sel == darkRoot ||
					strings.HasPrefix(sel, darkScope) || strings.HasPrefix(sel, darkDefault)
				if !scoped {
					out = append(out, "theme rule outside the dark scope: "+sel)
				}
			} else if strings.Contains(sel, "data-bs-theme") {
				out = append(out, "behavior rule bound to a color mode: "+sel)
			}
		}
	}
	return out
}

// forbiddenIn lists the forbidden constructs in src outside comments, and the at-rules of sh that are not allowed.
func forbiddenIn(src string, sh *sheet) ([]string, error) {
	code, err := blankComments(src)
	if err != nil {
		return nil, err
	}
	code = strings.ToLower(code)
	var out []string
	for _, c := range forbiddenConstructs {
		if strings.Contains(code, c) {
			out = append(out, c)
		}
	}
	for _, name := range sh.atRules {
		if !slices.Contains(allowedAtRules, name) {
			out = append(out, "@"+name)
		}
	}
	return out, nil
}

// fragilePseudos are the pseudo-classes and pseudo-elements that some browser may not know: vendor-prefixed ones
// (Chromium and Safari do not know the ::-moz- ones, Firefox not most ::-webkit- ones) and recent ones. A browser drops
// a whole selector list that names one it does not know, so a selector with one of them must be the only one of its
// rule.
var fragilePseudos = []string{":-moz-", ":-webkit-", ":modal", ":popover-open", ":has("}

// fragileLists lists the selectors that name a fragile pseudo-class or pseudo-element in a rule with other selectors.
func fragileLists(sh *sheet) []string {
	var out []string
	for _, r := range sh.rules {
		if len(r.selectors) < 2 {
			continue
		}
		for _, sel := range r.selectors {
			s := strings.ToLower(withoutAttributes(sel))
			if slices.ContainsFunc(fragilePseudos, func(p string) bool { return strings.Contains(s, p) }) {
				out = append(out, sel)
			}
		}
	}
	return out
}

// notArguments returns the selectors in the arguments of every :not() of sel, whitespace-normalized.
func notArguments(sel string) ([]string, error) {
	var out []string
	for rest := sel; ; {
		i := strings.Index(rest, ":not(")
		if i < 0 {
			return out, nil
		}
		start := i + len(":not(")
		end, err := scan(rest, start, ")")
		if err != nil {
			return nil, err
		}
		if end == len(rest) {
			return nil, fmt.Errorf("%w :not( in %q", errUnbalanced, sel)
		}
		args, err := split(rest[start:end], ',')
		if err != nil {
			return nil, err
		}
		for _, a := range args {
			out = append(out, strings.Join(strings.Fields(a), " "))
		}
		rest = rest[end+1:]
	}
}

var tableVariantPattern = regexp.MustCompile(`^\.table-[a-z]+$`)

// tableVariants returns the contextual table variants of base (.table-light and the others): the single-class rules
// that set both the text and the background color of a table's cells.
func tableVariants(base *sheet) []string {
	var out []string
	for _, r := range base.rules {
		if len(r.selectors) != 1 || !tableVariantPattern.MatchString(r.selectors[0]) {
			continue
		}
		props := make(map[string]bool)
		for _, d := range r.decls {
			props[d.prop] = true
		}
		if props["--bs-table-color"] && props["--bs-table-bg"] && !slices.Contains(out, r.selectors[0]) {
			out = append(out, r.selectors[0])
		}
	}
	return out
}

// tableColorsOverVariants lists the selectors of sh that set the text color of table cells without leaving out every
// variant, both on the cell itself and on an ancestor (the table, its head or body, the row): a variant keeps its
// fixed light background in dark mode, so the theme's light text would vanish on it.
func tableColorsOverVariants(sh *sheet, variants []string) ([]string, error) {
	var out []string
	for _, r := range sh.rules {
		if !slices.ContainsFunc(r.decls, func(d decl) bool { return d.prop == "color" }) {
			continue
		}
		for _, sel := range r.selectors {
			classes := classPattern.FindAllStringSubmatch(withoutAttributes(sel), -1)
			if !slices.ContainsFunc(classes, func(m []string) bool { return m[1] == "table" || m[1] == "table-sm" }) {
				continue
			}
			args, err := notArguments(sel)
			if err != nil {
				return nil, err
			}
			for _, v := range variants {
				if !slices.Contains(args, v) || !slices.Contains(args, v+" *") {
					out = append(out, sel)
					break
				}
			}
		}
	}
	return out, nil
}

// themeSheets holds webui.css and Bootstrap's stylesheet, read and parsed once for all tests; read-only after.
type themeSheets struct {
	src       string
	theme     *sheet
	bootstrap *sheet
}

var loadSheets = sync.OnceValues(func() (*themeSheets, error) {
	src, err := os.ReadFile(themeFile)
	if err != nil {
		return nil, err
	}
	theme, err := parseSheet(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", themeFile, err)
	}
	bsSrc, err := os.ReadFile(bootstrapFile)
	if err != nil {
		return nil, err
	}
	bootstrap, err := parseSheet(string(bsSrc))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", bootstrapFile, err)
	}
	return &themeSheets{src: string(src), theme: theme, bootstrap: bootstrap}, nil
})

func load(t devtest.T) *themeSheets {
	t.Helper()
	s, err := loadSheets()
	t.Must(be.NoError(err))
	return s
}

var (
	varOnly = regexp.MustCompile(`^var\(\s*(--[_a-zA-Z0-9-]+)\s*\)$`)
	// translucentVar is a color such as Bootstrap's muted text: rgba(var(--bs-body-color-rgb), 0.75).
	translucentVar = regexp.MustCompile(`^rgba\(\s*var\(\s*(--[_a-zA-Z0-9-]+)\s*\)\s*,\s*([0-9.]+)\s*\)$`)
)

// darkValue returns the value that the custom property prop has on an element with the class cls, on a page in the
// dark color mode: the theme's declaration for cls, else Bootstrap's for cls, else the dark mode's global value (the
// theme's, else Bootstrap's dark one, else Bootstrap's root one), following the references of values that are a single
// var().
func (s *themeSheets) darkValue(cls, prop string) (string, error) {
	name := prop
	for range 10 {
		value, ok := s.theme.lookup(darkScope+cls, name)
		if !ok {
			value, ok = s.bootstrap.lookup(cls, name)
		}
		if !ok {
			value, ok = s.theme.lookup(darkVars, name)
		}
		if !ok {
			value, ok = s.bootstrap.lookup("[data-bs-theme=dark]", name)
		}
		if !ok {
			value, ok = s.bootstrap.lookup(":root", name)
		}
		if !ok {
			return "", fmt.Errorf("%s: %s is declared nowhere", prop, name)
		}
		if m := varOnly.FindStringSubmatch(value); m != nil {
			name = m[1]
			continue
		}
		return value, nil
	}
	return "", fmt.Errorf("%s: too many var() references", prop)
}

// darkColor returns the color that the custom property prop has on an element with the class cls, as darkValue finds
// it. It understands just what the color checks need: a hex color, after var() references.
func (s *themeSheets) darkColor(cls, prop string) (rgb [3]uint8, err error) {
	value, err := s.darkValue(cls, prop)
	if err != nil {
		return rgb, err
	}
	return parseHex(value)
}

// darkColorOn is darkColor for a color drawn on bg: it also understands a translucent color written as
// rgba(var(--x-rgb), alpha), where --x-rgb is a triple such as "236, 239, 243", and composites it over bg.
func (s *themeSheets) darkColorOn(cls, prop string, bg [3]uint8) (rgb [3]uint8, err error) {
	value, err := s.darkValue(cls, prop)
	if err != nil {
		return rgb, err
	}
	m := translucentVar.FindStringSubmatch(value)
	if m == nil {
		return parseHex(value)
	}
	alpha, err := strconv.ParseFloat(m[2], 64)
	if err != nil || alpha < 0 || alpha > 1 {
		return rgb, fmt.Errorf("%w: not an alpha: %q", errSyntax, m[2])
	}
	triple, err := s.darkValue(cls, m[1])
	if err != nil {
		return rgb, err
	}
	fg, err := parseTriple(triple)
	if err != nil {
		return rgb, err
	}
	for i := range rgb {
		rgb[i] = uint8(math.Round(alpha*float64(fg[i]) + (1-alpha)*float64(bg[i])))
	}
	return rgb, nil
}

// parseTriple parses the "r, g, b" value of one of Bootstrap's -rgb properties.
func parseTriple(value string) (rgb [3]uint8, err error) {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return rgb, fmt.Errorf("%w: not an r, g, b triple: %q", errSyntax, value)
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(strings.TrimSpace(p), 10, 8)
		if err != nil {
			return rgb, fmt.Errorf("%w: not an r, g, b triple: %q", errSyntax, value)
		}
		rgb[i] = uint8(n)
	}
	return rgb, nil
}

// parseHex parses a #rgb or #rrggbb color.
func parseHex(value string) (rgb [3]uint8, err error) {
	h, ok := strings.CutPrefix(strings.ToLower(value), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if !ok || len(h) != 6 {
		return rgb, fmt.Errorf("%w: not a hex color: %q", errSyntax, value)
	}
	for i := range rgb {
		n, err := strconv.ParseUint(h[2*i:2*i+2], 16, 8)
		if err != nil {
			return rgb, fmt.Errorf("%w: not a hex color: %q", errSyntax, value)
		}
		rgb[i] = uint8(n)
	}
	return rgb, nil
}

// contrast returns the WCAG 2 contrast ratio of two colors.
func contrast(a, b [3]uint8) float64 {
	luminance := func(c [3]uint8) float64 {
		var l [3]float64
		for i, v := range c {
			f := float64(v) / 255
			if f <= 0.04045 {
				l[i] = f / 12.92
			} else {
				l[i] = math.Pow((f+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
	}
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// tokens returns the --webui- tokens of sh with their defaults, sorted by name.
func tokens(sh *sheet) [][2]string {
	var out [][2]string
	declared := sh.declared()
	for _, p := range sortedKeys(declared) {
		if strings.HasPrefix(p, "--webui-") {
			out = append(out, [2]string{p, declared[p][0]})
		}
	}
	return out
}

// TestTokens pins the theme's own custom properties, the tuning surface that the package documentation lists, and
// their defaults. They are declared only on darkVars, as specific as the [data-bs-theme="dark"] selector that the
// documentation gives for overrides.
func TestTokens(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	t.Must(be.DeepEqual([][2]string{
		{"--webui-control-border", "var(--bs-border-color)"},
		{"--webui-fill-danger", "#a12733"},
		{"--webui-fill-danger-hover", "#bb2d3b"},
		{"--webui-fill-info", "#0ba5c3"},
		{"--webui-fill-info-hover", "#0cb5d8"},
		{"--webui-fill-primary", "#0b4aa8"},
		{"--webui-fill-primary-hover", "#0d58c8"},
		{"--webui-fill-secondary", "#3a4047"},
		{"--webui-fill-secondary-hover", "#4a5159"},
		{"--webui-fill-success", "#126239"},
		{"--webui-fill-success-hover", "#157347"},
		{"--webui-fill-warning", "#c79100"},
		{"--webui-fill-warning-hover", "#dba000"},
		{"--webui-focus-color", "var(--bs-primary)"},
		{"--webui-focus-offset", "1px"},
		{"--webui-focus-width", "2px"},
		{"--webui-nav-link-color", "#9cc4ff"},
	}, tokens(s.theme)))
	for _, r := range s.theme.rules {
		for _, d := range r.decls {
			if strings.HasPrefix(d.prop, "--webui-") {
				t.Mustf(be.DeepEqual([]string{darkVars}, r.selectors), "%s declared on %q", d.prop, r.selectors)
			}
		}
	}
}

// TestThemeFile checks the encoding and the shape of the file, and that both stylesheets parse into a plausible
// number of rules, so that the other tests cannot pass on an empty parse.
func TestThemeFile(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	t.Must(be.True(utf8.ValidString(s.src)))
	t.Must(be.Suffix(s.src, "\n"))
	t.Must(be.False(strings.HasSuffix(s.src, "\n\n")))
	t.Must(be.False(strings.Contains(s.src, "\r")))
	t.Must(be.Equal(1, strings.Count(s.src, behaviorMarker)))
	t.Must(be.GreaterOrEq(len(s.theme.rules), 80))
	t.Must(be.GreaterOrEq(len(s.bootstrap.rules), 1000))
	known := s.bootstrap.classes()
	for _, c := range []string{"btn", "modal", "accordion-button", "dropdown-menu", "form-control", "toast"} {
		t.Mustf(be.True(known[c]), "Bootstrap class %q", c)
	}
	t.Must(be.NonNil(s.bootstrap.declared()["--bs-body-bg"]))
}

// TestThemeSelectors checks that the theme selects only Bootstrap's and htmx's classes, names no ids, scopes the theme
// part to the dark color mode and leaves the behavior part independent of it.
func TestThemeSelectors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	unknown := unknownClasses(s.theme, s.bootstrap)
	t.Mustf(be.Empty(unknown), "classes that are neither Bootstrap's nor htmx's: %q", unknown)
	ids := idSelectors(s.theme)
	t.Mustf(be.Empty(ids), "id selectors: %q", ids)
	scope := misscoped(s.theme, strings.Index(s.src, behaviorMarker))
	t.Mustf(be.Empty(scope), "%q", scope)
	fragile := fragileLists(s.theme)
	t.Mustf(be.Empty(fragile), "fragile selectors in a selector list: %q", fragile)
}

// TestThemeTableVariants checks that the theme's text colors for table cells leave Bootstrap's contextual table
// variants alone, wherever the variant class is: they keep their light backgrounds in dark mode, with black text.
func TestThemeTableVariants(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	variants := tableVariants(s.bootstrap)
	t.Must(be.DeepEqual([]string{".table-primary", ".table-secondary", ".table-success", ".table-info",
		".table-warning", ".table-danger", ".table-light", ".table-dark"}, variants))
	over, err := tableColorsOverVariants(s.theme, variants)
	t.Must(be.NoError(err))
	t.Mustf(be.Empty(over), "table cell colors over the variants: %q", over)

	// The header cells still get the muted color and the caption size.
	head := darkScope + `.table > thead th`
	got, ok := s.theme.lookup(head, "font-size")
	t.Must(be.True(ok))
	t.Must(be.Equal("0.8em", got))
	var muted []string
	for _, r := range s.theme.rules {
		for _, sel := range r.selectors {
			if strings.HasPrefix(sel, head+":not(") &&
				slices.ContainsFunc(r.decls, func(d decl) bool { return d.prop == "color" && d.value == "var(--bs-secondary-color)" }) {
				muted = append(muted, sel)
			}
		}
	}
	t.Must(be.Len(muted, 1))
}

// TestThemeContrast checks the non-text contrast of the selected states and of the progress bar, on the black body, as
// computed from the declared colors: 3:1 for the state, 4.5:1 for the text on a fill. A state without a border is
// shown by its fill alone.
func TestThemeContrast(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	color := func(t devtest.T, cls, prop string) [3]uint8 {
		t.Helper()
		c, err := s.darkColor(cls, prop)
		t.Must(be.NoError(err))
		return c
	}
	body := color(t, ".x", "--bs-body-bg")
	bar := color(t, ".x", "--bs-tertiary-bg")
	t.Must(be.Equal([3]uint8{0, 0, 0}, body))
	for _, c := range []struct {
		name     string
		fg, bg   [3]uint8
		minRatio float64
	}{
		{"active nav pill", color(t, ".nav-pills", "--bs-nav-pills-link-active-bg"), body, 3},
		{"active nav pill on a bar", color(t, ".nav-pills", "--bs-nav-pills-link-active-bg"), bar, 3},
		{"active nav pill text", color(t, ".nav-pills", "--bs-nav-pills-link-active-color"),
			color(t, ".nav-pills", "--bs-nav-pills-link-active-bg"), 4.5},
		{"active dropdown item", color(t, ".dropdown-menu", "--bs-dropdown-link-active-bg"),
			color(t, ".dropdown-menu", "--bs-dropdown-bg"), 3},
		{"active dropdown item text", color(t, ".dropdown-menu", "--bs-dropdown-link-active-color"),
			color(t, ".dropdown-menu", "--bs-dropdown-link-active-bg"), 4.5},
		{"active list item border", color(t, ".list-group", "--bs-list-group-active-border-color"), body, 3},
		{"active page border", color(t, ".pagination", "--bs-pagination-active-border-color"), body, 3},
		{"progress bar on its track", color(t, ".progress", "--bs-progress-bar-bg"),
			color(t, ".progress", "--bs-progress-bg"), 3},
		{"progress bar on the body", color(t, ".progress", "--bs-progress-bar-bg"), body, 3},
		{"progress label", color(t, ".progress", "--bs-progress-bar-color"),
			color(t, ".progress", "--bs-progress-bar-bg"), 4.5},
		{"progress track border", color(t, ".x", "--webui-control-border"), body, 3},
		{"focus outline", color(t, ".x", "--webui-focus-color"), body, 3},
	} {
		ratio := contrast(c.fg, c.bg)
		t.Mustf(be.GreaterOrEq(ratio, c.minRatio), "%s: %.2f:1", c.name, ratio)
	}

	// The track of a progress bar has the border whose contrast is checked above; the segments of a stacked bar do not.
	for _, want := range []struct{ selector, value string }{
		{darkScope + ".progress", "var(--bs-border-width) solid var(--webui-control-border)"},
		{darkScope + ".progress-stacked", "var(--bs-border-width) solid var(--webui-control-border)"},
		{darkScope + ".progress-stacked > .progress", "0"},
	} {
		got, ok := s.theme.lookup(want.selector, "border")
		t.Mustf(be.True(ok), "%s { border }", want.selector)
		t.Mustf(be.Equal(want.value, got), "%s { border }", want.selector)
	}
}

// buttonVariants are Bootstrap's button variants: .btn-{v} and .btn-outline-{v}.
var buttonVariants = []string{"primary", "secondary", "success", "danger", "warning", "info", "light", "dark"}

// TestThemeButtons checks the buttons of every variant, filled and outline, on the black body and on a bar (a navbar,
// a card header): the text 4.5:1 or more at rest, on the hover fill and on the fill when pressed (.active, or a checked
// .btn-check), and the edge, the border, 3:1 or more. A disabled outline button keeps its text visible: 3:1 or more
// before Bootstrap's opacity, which a disabled control is exempt from raising. Bootstrap's dark (#212529) fails these on
// black, which the theme makes up for.
func TestThemeButtons(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	backgrounds := []struct {
		name string
		prop string
	}{{"body", "--bs-body-bg"}, {"bar", "--bs-tertiary-bg"}}
	for _, v := range buttonVariants {
		for _, cls := range []string{".btn-" + v, ".btn-outline-" + v} {
			t.Run(cls, func(t devtest.T) {
				color := func(prop string, bg [3]uint8) [3]uint8 {
					t.Helper()
					c, err := s.darkColorOn(cls, prop, bg)
					t.Must(be.NoError(err))
					return c
				}
				check := func(what string, fg, bg [3]uint8, minRatio float64) {
					t.Helper()
					ratio := contrast(fg, bg)
					t.Mustf(be.GreaterOrEq(ratio, minRatio), "%s %s: %.2f:1", cls, what, ratio)
				}
				outline := strings.HasPrefix(cls, ".btn-outline-")
				for _, b := range backgrounds {
					page := color(b.prop, [3]uint8{})
					check("border on the "+b.name, color("--bs-btn-border-color", page), page, 3)
					rest := page // an outline button is transparent at rest
					if !outline {
						rest = color("--bs-btn-bg", page)
					}
					check("text on the "+b.name, color("--bs-btn-color", rest), rest, 4.5)
					if outline {
						check("disabled text on the "+b.name, color("--bs-btn-disabled-color", page), page, 3)
					}
				}
				hover := color("--bs-btn-hover-bg", [3]uint8{})
				check("text on hover", color("--bs-btn-hover-color", hover), hover, 4.5)
				active := color("--bs-btn-active-bg", [3]uint8{})
				check("text when pressed", color("--bs-btn-active-color", active), active, 4.5)
			})
		}
	}

	// The dark buttons: Bootstrap's dark would be their edge (and the outline one's text) at 1.3:1 on black.
	body, err := s.darkColor(".x", "--bs-body-bg")
	t.Must(be.NoError(err))
	dark, err := s.darkColor(".x", "--bs-dark")
	t.Must(be.NoError(err))
	t.Must(be.Equal([3]uint8{0x21, 0x25, 0x29}, dark))
	t.Must(be.Less(contrast(dark, body), 1.4))
	for _, want := range []struct{ cls, prop, value string }{
		{".btn-dark", "--bs-btn-border-color", "var(--webui-control-border)"},
		{".btn-outline-dark", "--bs-btn-border-color", "var(--webui-control-border)"},
		{".btn-outline-dark", "--bs-btn-color", "var(--bs-dark-text-emphasis)"},
		{".btn-outline-dark", "--bs-btn-hover-bg", "var(--webui-fill-secondary)"},
	} {
		got, ok := s.theme.lookup(darkScope+want.cls, want.prop)
		t.Mustf(be.True(ok), "%s { %s }", want.cls, want.prop)
		t.Mustf(be.Equal(want.value, got), "%s { %s }", want.cls, want.prop)
	}
}

// fixedColorClasses are Bootstrap classes with fixed colors that the theme leaves alone on purpose: they glare or fall
// short on black, or are meant for light backgrounds, and the header comment of webui.css says so.
// (The contextual table variants are also fixed; TestThemeTableVariants checks them.)
var fixedColorClasses = []string{
	"bg-primary", "text-secondary", "btn-light", "text-bg-light", "text-dark", "link-dark", "border-dark",
	"text-bg-dark", "btn-outline-light",
}

// namesClass reports whether text names the class cls as .cls, not as the start of a longer class name.
func namesClass(text, cls string) bool {
	return regexp.MustCompile(`\.` + regexp.QuoteMeta(cls) + `([^_a-zA-Z0-9-]|$)`).MatchString(text)
}

// TestThemeFixedColors checks that the theme selects none of the fixed-color classes, and that the paragraph of the
// header comment on them names each, so that a reader knows which classes not to rely on.
func TestThemeFixedColors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	start := strings.Index(s.src, "Fixed-color classes")
	t.Must(be.GreaterOrEq(start, 0))
	end := strings.Index(s.src[start:], "*/")
	t.Must(be.Greater(end, 0))
	paragraph := s.src[start : start+end]
	selected := s.theme.classes()
	known := s.bootstrap.classes()
	for _, c := range fixedColorClasses {
		t.Mustf(be.True(known[c]), "Bootstrap class %q", c)
		t.Mustf(be.False(selected[c]), "the theme selects .%s", c)
		t.Mustf(be.True(namesClass(paragraph, c)), "the header comment does not name .%s", c)
	}
	// The dark buttons are themed (see TestThemeButtons), so the paragraph does not list them as fixed.
	for _, c := range []string{"btn-dark", "btn-outline-dark"} {
		t.Mustf(be.True(selected[c]), "the theme does not select .%s", c)
		t.Mustf(be.False(namesClass(paragraph, c)), "the header comment lists .%s as fixed", c)
	}
}

// TestThemeVariables checks the custom properties: every one read is declared by the theme or by Bootstrap, every
// token is read and has one default, and every other property set is one of Bootstrap's.
func TestThemeVariables(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	undefined := undefinedReads(s.theme, s.bootstrap)
	t.Mustf(be.Empty(undefined), "read but declared nowhere: %q", undefined)
	unused := unusedTokens(s.theme)
	t.Mustf(be.Empty(unused), "tokens never read: %q", unused)
	again := redeclaredTokens(s.theme)
	t.Mustf(be.Empty(again), "tokens declared more than once: %q", again)
	foreign := foreignProperties(s.theme, s.bootstrap)
	t.Mustf(be.Empty(foreign), "neither --webui- tokens nor Bootstrap's properties: %q", foreign)
	t.Must(be.NonEmpty(tokens(s.theme)))
}

// TestThemeSafety checks the content security posture (no imports, fetches, layers or script) and the flat look (no
// shadow other than none), and that every rule is meaningful.
func TestThemeSafety(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	found, err := forbiddenIn(s.src, s.theme)
	t.Must(be.NoError(err))
	t.Mustf(be.Empty(found), "forbidden constructs: %q", found)
	drawn := shadows(s.theme)
	t.Mustf(be.Empty(drawn), "shadows: %q", drawn)
	bad := emptyOrDuplicated(s.theme)
	t.Mustf(be.Empty(bad), "%q", bad)
}

// TestThemeContract checks the rules that other packages rely on: the dialog markup contract and the inert page of its
// popover fallback, the details-based accordion and dropdown, the htmx indicators, and the density of the dark mode.
func TestThemeContract(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := load(t)
	const (
		accordionClosed = `details.accordion-item:not([open]) > summary.accordion-button`
		accordionMarker = `details.accordion-item > summary.accordion-button::-webkit-details-marker`
		dropdownMenu    = `details.dropdown[open] > .dropdown-menu`
		dropdownMarker  = `details.dropdown > summary::-webkit-details-marker`
		indicator       = `.htmx-indicator:not(.htmx-request .htmx-indicator, .htmx-request.htmx-indicator)`
		// Every element that neither contains the open dialog popover nor is in it: the dialog's ancestors stay
		// interactive, as an inert ancestor would make the dialog inert too, whatever interactivity it sets itself.
		popoverInert = `body:has(dialog.modal:popover-open) :not(:has(dialog.modal:popover-open))` +
			`:not(dialog.modal:popover-open):not(dialog.modal:popover-open *)`
	)
	for _, want := range []struct{ selector, prop, value string }{
		{"dialog.modal", "background-color", "transparent"},
		{"dialog.modal", "border", "0"},
		{"dialog.modal", "margin", "0"},
		{"dialog.modal", "max-height", "none"},
		{"dialog.modal", "max-width", "none"},
		{"dialog.modal", "overscroll-behavior", "contain"},
		{"dialog.modal", "padding", "0"},
		{"dialog.modal:modal", "display", "block"},
		{"dialog.modal:popover-open", "display", "block"},
		{"dialog.modal::backdrop", "background-color", "rgba(0, 0, 0, 0.75)"},
		{popoverInert, "interactivity", "inert"},
		{accordionClosed, "background-color", "var(--bs-accordion-btn-bg)"},
		{accordionClosed, "color", "var(--bs-accordion-btn-color)"},
		{accordionClosed + ":not(:focus)", "box-shadow", "none"},
		{accordionClosed + "::after", "background-image", "var(--bs-accordion-btn-icon)"},
		{accordionClosed + "::after", "transform", "none"},
		{accordionMarker, "display", "none"},
		{dropdownMenu, "display", "block"},
		{dropdownMenu, "top", "100%"},
		{dropdownMarker, "display", "none"},
		{indicator, "display", "none !important"},
		{`html[data-bs-theme="dark"]`, "font-size", "87.5%"},
	} {
		got, ok := s.theme.lookup(want.selector, want.prop)
		t.Mustf(be.True(ok), "%s { %s }", want.selector, want.prop)
		t.Mustf(be.Equal(want.value, got), "%s { %s }", want.selector, want.prop)
	}

	// The popover fallback makes the page inert by that one generic rule, never by selectors of a page layout, and
	// nothing sets interactivity on the dialog or its ancestors.
	var inert []string
	for _, r := range s.theme.rules {
		if slices.ContainsFunc(r.decls, func(d decl) bool { return d.prop == "interactivity" }) {
			inert = append(inert, r.selectors...)
		}
	}
	t.Must(be.DeepEqual([]string{popoverInert}, inert))

	// The dialog is never shown by an open attribute: the contract never renders one.
	for _, r := range s.theme.rules {
		for _, sel := range r.selectors {
			t.Mustf(be.False(strings.Contains(sel, "dialog[open]") || strings.Contains(sel, "modal[open]")), "%s", sel)
		}
	}
}

// TestCheckers runs the checks on small stylesheets with known defects, so that a check which passes the theme is
// known to be able to fail.
func TestCheckers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	base, err := parseSheet(`.btn{--bs-btn-color:#fff;color:var(--bs-btn-color)}.modal{display:none}`)
	t.Must(be.NoError(err))
	parse := func(t devtest.T, src string) *sheet {
		t.Helper()
		sh, err := parseSheet(src)
		t.Must(be.NoError(err))
		return sh
	}

	t.Run("classes", func(t devtest.T) {
		sh := parse(t, `.btn .sidebar, [data-x="a.b"] .modal:not(.btn, .widget), .htmx-request {color:red}`)
		t.Must(be.DeepEqual([]string{"sidebar", "widget"}, unknownClasses(sh, base)))
	})
	t.Run("ids", func(t devtest.T) {
		sh := parse(t, `#main .btn, [href="#top"], .modal {color:red}`)
		t.Must(be.DeepEqual([]string{"#main .btn"}, idSelectors(sh)))
	})
	t.Run("reads", func(t devtest.T) {
		sh := parse(t, `.btn{--webui-a:var(--webui-b);color:var(--webui-a) var(--bs-btn-color) var(--bs-nope)}`)
		t.Must(be.DeepEqual([]string{"--bs-nope", "--webui-b"}, undefinedReads(sh, base)))
	})
	t.Run("tokens", func(t devtest.T) {
		sh := parse(t, `.btn{--webui-a:1px;--webui-b:var(--webui-a);--webui-c:2px}.modal{--webui-a:3px}`)
		t.Must(be.DeepEqual([]string{"--webui-b", "--webui-c"}, unusedTokens(sh)))
		t.Must(be.DeepEqual([]string{"--webui-a"}, redeclaredTokens(sh)))
		t.Must(be.DeepEqual([][2]string{{"--webui-a", "1px"}, {"--webui-b", "var(--webui-a)"}, {"--webui-c", "2px"}},
			tokens(sh)))
	})
	t.Run("foreign", func(t devtest.T) {
		sh := parse(t, `.btn{--bs-btn-color:red;--bs-btn-colour:red;--app-x:1;--webui-x:1}`)
		t.Must(be.DeepEqual([]string{"--app-x", "--bs-btn-colour"}, foreignProperties(sh, base)))
	})
	t.Run("shadows", func(t devtest.T) {
		sh := parse(t, `.btn{box-shadow:none;text-shadow:NONE;--bs-box-shadow:0 0 1px red;filter:Drop-Shadow(0 0 1px red)}`)
		t.Must(be.DeepEqual([]string{"--bs-box-shadow: 0 0 1px red", "filter: Drop-Shadow(0 0 1px red)"}, shadows(sh)))
	})
	t.Run("hygiene", func(t devtest.T) {
		sh := parse(t, `.btn{} .modal{color:red;color:blue}`)
		t.Must(be.DeepEqual([]string{"empty: .btn", "duplicate color: .modal"}, emptyOrDuplicated(sh)))
	})
	t.Run("scope", func(t devtest.T) {
		src := `[data-bs-theme="dark"]:where(:root) .btn, .modal {color:red}
			[data-bs-theme="dark"]:where(:root, :root[data-bs-theme="dark"] *) {--bs-body-bg:#000}
			:where(:root[data-bs-theme="dark"]) .btn, html[data-bs-theme="dark"] {color:red}
			[data-bs-theme="dark"] {--bs-body-bg:#000} [data-bs-theme="dark"] .btn {color:red}
			/* ---- Behavior */ [data-bs-theme="dark"] .x, dialog {color:red}`
		sh := parse(t, src)
		t.Must(be.DeepEqual([]string{
			"theme rule outside the dark scope: .modal",
			`theme rule outside the dark scope: [data-bs-theme="dark"]`,
			`theme rule outside the dark scope: [data-bs-theme="dark"] .btn`,
			`behavior rule bound to a color mode: [data-bs-theme="dark"] .x`,
		}, misscoped(sh, strings.Index(src, behaviorMarker))))
	})
	t.Run("fragile", func(t devtest.T) {
		sh := parse(t, `.a::-moz-range-thumb, .b {color:red} dialog:modal {display:block}
			.c:has(.d), [data-x=":modal"] {color:red} .e::-webkit-slider-thumb {color:red}`)
		t.Must(be.DeepEqual([]string{".a::-moz-range-thumb", ".c:has(.d)"}, fragileLists(sh)))
	})
	t.Run("table variants", func(t devtest.T) {
		bs := parse(t, `.table-light{--bs-table-color:#000;--bs-table-bg:#f8f9fa}.table-dark{--bs-table-color:#fff;
			--bs-table-bg:#212529}.table-active{--bs-table-color-state:#fff}.table-sm>*{padding:0}`)
		variants := tableVariants(bs)
		t.Must(be.DeepEqual([]string{".table-light", ".table-dark"}, variants))
		sh := parse(t, `.table > thead th {color:red} .table th:not(.table-light, .table-dark) {color:red}
			.table td:not(.table-light, .table-light *, .table-dark, .table-dark *) {color:red}
			.table > thead th {font-weight:600} .x th {color:red}`)
		over, err := tableColorsOverVariants(sh, variants)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{".table > thead th", ".table th:not(.table-light, .table-dark)"}, over))
	})
	t.Run("colors", func(t devtest.T) {
		got, err := parseHex("#0D6efd")
		t.Must(be.NoError(err))
		t.Must(be.Equal([3]uint8{13, 110, 253}, got))
		got, err = parseHex("#fff")
		t.Must(be.NoError(err))
		t.Must(be.Equal([3]uint8{255, 255, 255}, got))
		for _, bad := range []string{"fff", "#ffff", "#ggg", "rgba(0, 0, 0, 0.5)", "var(--x)"} {
			_, err = parseHex(bad)
			t.Mustf(be.ErrorIs(err, errSyntax), "%q", bad)
		}
		t.Must(be.InDelta(21, contrast([3]uint8{255, 255, 255}, [3]uint8{0, 0, 0}), 0.001))
		t.Must(be.InDelta(2.559, contrast([3]uint8{0, 0, 0}, [3]uint8{0x0b, 0x4a, 0xa8}), 0.001))
		t.Must(be.InDelta(2.559, contrast([3]uint8{0x0b, 0x4a, 0xa8}, [3]uint8{0, 0, 0}), 0.001))

		sheets := &themeSheets{
			theme: parse(t, `[data-bs-theme="dark"]:where(:root, :root[data-bs-theme="dark"] *){--webui-a:var(--bs-a)}
				[data-bs-theme="dark"]:where(:root) .c{--bs-c:var(--webui-a)}`),
			bootstrap: parse(t, `:root{--bs-a:#111;--bs-b:#222}[data-bs-theme=dark]{--bs-a:#333}.c{--bs-c:#444;--bs-d:var(--bs-b)}
				.loop{--bs-e:var(--bs-f);--bs-f:var(--bs-e)}`),
		}
		for _, c := range []struct {
			cls, prop string
			want      [3]uint8
		}{
			{".c", "--bs-c", [3]uint8{0x33, 0x33, 0x33}},
			{".c", "--bs-d", [3]uint8{0x22, 0x22, 0x22}},
			{".x", "--webui-a", [3]uint8{0x33, 0x33, 0x33}},
		} {
			got, err := sheets.darkColor(c.cls, c.prop)
			t.Must(be.NoError(err))
			t.Mustf(be.Equal(c.want, got), "%s %s", c.cls, c.prop)
		}
		_, err = sheets.darkColor(".c", "--bs-nope")
		t.Must(be.Error(err))
		_, err = sheets.darkColor(".loop", "--bs-e")
		t.Must(be.Error(err))

		// Translucent colors, composited over the background they are drawn on.
		sheets = &themeSheets{
			theme: parse(t, `[data-bs-theme="dark"]:where(:root, :root[data-bs-theme="dark"] *){--bs-fg-rgb:200, 100, 0}
				[data-bs-theme="dark"]:where(:root) .c{--bs-c:var(--bs-muted)}`),
			bootstrap: parse(t, `:root{--bs-muted:rgba(var(--bs-fg-rgb), 0.5);--bs-solid:#102030;--bs-bad:rgba(var(--bs-fg-rgb), 2);
				--bs-short:rgba(var(--bs-bad-rgb), 0.5);--bs-bad-rgb:1, 2}`),
		}
		got, err = sheets.darkColorOn(".c", "--bs-c", [3]uint8{0, 0, 0})
		t.Must(be.NoError(err))
		t.Must(be.Equal([3]uint8{100, 50, 0}, got))
		got, err = sheets.darkColorOn(".c", "--bs-c", [3]uint8{0, 100, 200})
		t.Must(be.NoError(err))
		t.Must(be.Equal([3]uint8{100, 100, 100}, got))
		got, err = sheets.darkColorOn(".c", "--bs-solid", [3]uint8{255, 255, 255})
		t.Must(be.NoError(err))
		t.Must(be.Equal([3]uint8{0x10, 0x20, 0x30}, got))
		_, err = sheets.darkColor(".c", "--bs-c")
		t.Must(be.ErrorIs(err, errSyntax))
		for _, bad := range []string{"--bs-bad", "--bs-short"} {
			_, err = sheets.darkColorOn(".c", bad, [3]uint8{})
			t.Mustf(be.ErrorIs(err, errSyntax), "%s", bad)
		}
		for _, bad := range []string{"1, 2", "1, 2, 300", "a, b, c", "1, 2, 3, 4"} {
			_, err = parseTriple(bad)
			t.Mustf(be.ErrorIs(err, errSyntax), "%q", bad)
		}
	})
	t.Run("forbidden", func(t devtest.T) {
		src := `/* url( and @import in a comment are prose */
			@IMPORT "x.css"; @layer base { .btn { background: URL(x.png) } }`
		found, err := forbiddenIn(src, parse(t, src))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"@import", "@layer", "url(", "@import", "@layer"}, found))

		clean := `/* no url( here */ @media (min-width: 1px) { .btn { content: "/* not a comment */" } }`
		found, err = forbiddenIn(clean, parse(t, clean))
		t.Must(be.NoError(err))
		t.Must(be.Empty(found))
	})
	t.Run("lookup", func(t devtest.T) {
		sh := parse(t, `.a,.b{color:red}.b{color:blue!important;margin:0}@media print{.b{margin:1px}}`)
		got, ok := sh.lookup(".b", "color")
		t.Must(be.True(ok))
		t.Must(be.Equal("blue !important", got))
		got, ok = sh.lookup(".b", "margin")
		t.Must(be.True(ok))
		t.Must(be.Equal("1px", got))
		_, ok = sh.lookup(".c", "color")
		t.Must(be.False(ok))
	})
}

// TestParseErrors checks that the parser rejects broken stylesheets instead of skipping parts of them, which would
// hide rules from the checks.
func TestParseErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct {
		name, src string
		err       error
	}{
		{"unterminated comment", `.btn{color:red} /* end`, errUnterminated},
		{"unterminated string", `.btn{content:"x}`, errUnterminated},
		{"unterminated block", `.btn{color:red`, errUnterminated},
		{"unterminated nested block", `@media print{.btn{color:red}`, errUnterminated},
		{"stray brace", `.btn{color:red}}`, errUnbalanced},
		{"unbalanced bracket", `.btn:not(.x{color:red}`, errUnbalanced},
		{"declaration outside a rule", `color:red;`, errSyntax},
		{"declaration without a colon", `.btn{color}`, errSyntax},
		{"declaration without a property", `.btn{:red}`, errSyntax},
		{"empty selector", `.btn,{color:red}`, errSyntax},
		{"nested rule", `.btn{color:red;.x{color:blue}}`, errSyntax},
	} {
		t.Run(c.name, func(t devtest.T) {
			_, err := parseSheet(c.src)
			t.Must(be.ErrorIs(err, c.err))
		})
	}

	t.Run("valid edge cases", func(t devtest.T) {
		sh, err := parseSheet("@charset \"UTF-8\";\n.a[title=\"{;}\"]{content:\"/*;*/\";color:red !IMPORTANT}\n" +
			"@keyframes k{from{opacity:0}to{opacity:1}}@media (x){@supports (y){.b{margin:0}}}")
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"charset", "keyframes", "media", "supports"}, sh.atRules))
		t.Must(be.Len(sh.rules, 2))
		t.Must(be.DeepEqual([]decl{{prop: "content", value: `"/*;*/"`}, {prop: "color", value: "red", important: true}},
			sh.rules[0].decls))
		t.Must(be.DeepEqual([]string{".b"}, sh.rules[1].selectors))
	})
}
