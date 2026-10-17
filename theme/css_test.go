package theme_test

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
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

// darkScopes are the prefixes that scope a theme selector to the dark color mode.
var darkScopes = []string{`[data-bs-theme="dark"]`, `:where([data-bs-theme="dark"])`, `html[data-bs-theme="dark"]`}

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

// misscoped lists the selectors outside the dark scope in the theme part of sh (before the byte offset marker), and
// those that name a color mode in the behavior part (from marker on).
func misscoped(sh *sheet, marker int) []string {
	var out []string
	for _, r := range sh.rules {
		for _, sel := range r.selectors {
			if r.offset < marker {
				if !slices.ContainsFunc(darkScopes, func(p string) bool { return strings.HasPrefix(sel, p) }) {
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

// The theme's own custom properties and their defaults. A page tunes the theme by overriding them in a stylesheet of
// its own, linked after the theme, e.g. [data-bs-theme="dark"] { --webui-focus-color: var(--bs-warning); }.
func Example_tokens() {
	s, err := loadSheets()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, tok := range tokens(s.theme) {
		fmt.Printf("%s: %s\n", tok[0], tok[1])
	}
	// Output:
	// --webui-control-border: var(--bs-border-color)
	// --webui-fill-danger: #a12733
	// --webui-fill-danger-hover: #bb2d3b
	// --webui-fill-info: #0ba5c3
	// --webui-fill-info-hover: #0cb5d8
	// --webui-fill-primary: #0b4aa8
	// --webui-fill-primary-hover: #0d58c8
	// --webui-fill-secondary: #3a4047
	// --webui-fill-secondary-hover: #4a5159
	// --webui-fill-success: #126239
	// --webui-fill-success-hover: #157347
	// --webui-fill-warning: #c79100
	// --webui-fill-warning-hover: #dba000
	// --webui-focus-color: var(--bs-primary)
	// --webui-focus-offset: 1px
	// --webui-focus-width: 2px
	// --webui-nav-link-color: #9cc4ff
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
		src := `[data-bs-theme="dark"] .btn, .modal {color:red}
			/* ---- Behavior */ [data-bs-theme="dark"] .x, dialog {color:red}`
		sh := parse(t, src)
		t.Must(be.DeepEqual([]string{
			"theme rule outside the dark scope: .modal",
			`behavior rule bound to a color mode: [data-bs-theme="dark"] .x`,
		}, misscoped(sh, strings.Index(src, behaviorMarker))))
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
