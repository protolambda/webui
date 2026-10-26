package hx2_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/theme"
)

// pinnedHTMXVersion is the htmx release that the Config keys and their
// meanings were checked against. When the theme moves to another release,
// re-check every key and default that Config writes (the history, eval and
// response handling semantics included), then update this constant and the
// Config doc.
const pinnedHTMXVersion = "2.0.10"

// htmx merges the htmx-config meta object into its config shallowly and
// ignores unknown keys, so a key that htmx does not know (misspelled, or
// renamed by a release) would silently leave htmx's insecure default in
// place. Every key that Config writes must be a key of the config literal of
// the htmx script that the theme ships.
func TestConfigKeysMatchPinnedHTMX(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal(pinnedHTMXVersion, theme.HTMXVersion))

	data, err := fs.ReadFile(theme.FS(), theme.HTMXJS)
	t.Must(be.NoError(err))
	script := string(data)
	t.Must(be.Substring(script, `version:"`+theme.HTMXVersion+`"`))
	t.Must(be.Substring(script, `meta[name="htmx-config"]`))

	literal, err := configLiteral(script)
	t.Must(be.NoError(err))
	keys, err := literalKeys(literal)
	t.Must(be.NoError(err))
	// A sanity check of the extraction: keys before and after the nested
	// responseHandling array.
	t.Must(be.True(keys["historyEnabled"]))
	t.Must(be.True(keys["reportValidityOfForms"]))

	// DefaultSwapStyle makes Config write every key that a field controls.
	written, err := hx2.Config{DefaultSwapStyle: hx2.SwapOuterHTML}.JSON()
	t.Must(be.NoError(err))
	var m map[string]json.RawMessage
	t.Must(be.NoError(json.Unmarshal([]byte(written), &m)))
	t.Must(be.Equal(11, len(m)))
	for k := range m {
		t.Must(be.Annotate(be.True(keys[k]), "key %s is not an htmx %s config key", k, theme.HTMXVersion))
	}

	// The htmx defaults that Config overrides with lists: settling style
	// attributes (which a strict style-src blocks) and DELETE parameters in
	// the URL. If a release changes them, re-check the overrides.
	for _, def := range []string{
		`attributesToSettle:["class","style","width","height"]`,
		`methodsThatUseUrlParams:["get","delete"]`,
	} {
		t.Must(be.Annotate(be.Substring(literal, def), "htmx %s default", theme.HTMXVersion))
	}
	// htmx reads both lists when it uses them, not once at startup.
	t.Must(be.Substring(script, `.config.methodsThatUseUrlParams.`))
	t.Must(be.Substring(script, `.config.attributesToSettle.`))

	// The response rules use the member names of htmx's own default rules.
	const rulesKey = "responseHandling:"
	t.Must(be.Substring(literal, rulesKey))
	at := strings.Index(literal, rulesKey) + len(rulesKey)
	end, err := closeBracket(literal, at)
	t.Must(be.NoError(err))
	rules := literal[at : end+1]
	for _, member := range []string{"{code:", ",swap:", ",error:"} {
		t.Must(be.Annotate(be.Substring(rules, member), "rule member %s", member))
	}
}

// The docs of Validate and Config rely on htmx's default of
// reportValidityOfForms, false (a request that htmx checks and stops shows
// nothing), and on Extra setting it, which makes htmx report the field.
func TestReportValidityOfForms(gt *testing.T) {
	t := devtest.ParallelT(gt)
	data, err := fs.ReadFile(theme.FS(), theme.HTMXJS)
	t.Must(be.NoError(err))
	script := string(data)
	literal, err := configLiteral(script)
	t.Must(be.NoError(err))
	t.Must(be.Substring(literal, "reportValidityOfForms:false"))
	t.Must(be.Substring(script, ".config.reportValidityOfForms)"))
	t.Must(be.Substring(script, ".reportValidity()"))

	written, err := hx2.Config{Extra: map[string]any{"reportValidityOfForms": true}}.JSON()
	t.Must(be.NoError(err))
	t.Must(be.Substring(written, `"reportValidityOfForms":true`))
}

// The extraction helpers fail rather than guess on input they do not
// understand.
func TestConfigLiteralParsing(gt *testing.T) {
	t := devtest.ParallelT(gt)
	lit, err := configLiteral(`x={a:1,config:{b:"}",c:[{d:'{'}],e:{f:1}},g:2}`)
	t.Must(be.NoError(err))
	t.Must(be.Equal(`{b:"}",c:[{d:'{'}],e:{f:1}}`, lit))
	keys, err := literalKeys(lit)
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual(map[string]bool{"b": true, "c": true, "e": true}, keys))

	for _, src := range []string{"", "config:{a:1", "config:{a:1},config:{b:2}", `config:{a:"}`} {
		_, err := configLiteral(src)
		t.Must(be.Annotate(be.True(err != nil), "source %q", src))
	}
	for _, lit := range []string{"{a}", "{:1}", "{a b:1}"} {
		_, err := literalKeys(lit)
		t.Must(be.Annotate(be.True(err != nil), "literal %q", lit))
	}
}

// configLiteral returns the object literal that follows the one "config:"
// of a minified htmx script, matching brackets and skipping string literals,
// since the literal nests objects and arrays.
func configLiteral(script string) (string, error) {
	const marker = "config:{"
	if n := strings.Count(script, marker); n != 1 {
		return "", fmt.Errorf("%d config literals, want 1", n)
	}
	start := strings.Index(script, marker) + len(marker) - 1
	end, err := closeBracket(script, start)
	if err != nil {
		return "", err
	}
	return script[start : end+1], nil
}

// closeBracket returns the index of the bracket that closes the one at
// start.
func closeBracket(s string, start int) (int, error) {
	depth := 0
	for i := start; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'', '`':
			j, err := closeQuote(s, i)
			if err != nil {
				return 0, err
			}
			i = j
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errors.New("unclosed bracket")
}

// closeQuote returns the index of the quote that closes the string literal
// that starts at start.
func closeQuote(s string, start int) (int, error) {
	q := s[start]
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case q:
			return i, nil
		}
	}
	return 0, errors.New("unclosed string")
}

// literalKeys returns the top-level keys of a JavaScript object literal with
// identifier keys, as minified scripts write it.
func literalKeys(lit string) (map[string]bool, error) {
	keys := map[string]bool{}
	i := 1 // after '{'
	for i < len(lit)-1 {
		j := strings.IndexByte(lit[i:], ':')
		if j <= 0 {
			return nil, fmt.Errorf("no key at %d", i)
		}
		key := lit[i : i+j]
		for k := 0; k < len(key); k++ {
			if c := key[k]; !(c == '_' || c == '$' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9') {
				return nil, fmt.Errorf("key %q is not an identifier", key)
			}
		}
		keys[key] = true
		// Skip the value: up to the next ',' or the end at depth 0.
		i += j + 1
		for i < len(lit)-1 && lit[i] != ',' {
			switch lit[i] {
			case '"', '\'', '`':
				end, err := closeQuote(lit, i)
				if err != nil {
					return nil, err
				}
				i = end
			case '{', '[':
				end, err := closeBracket(lit, i)
				if err != nil {
					return nil, err
				}
				i = end
			}
			i++
		}
		i++ // the ','
	}
	return keys, nil
}
