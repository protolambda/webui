package webuitest

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/protolambda/chord/ct"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/session"
)

// csrfFieldName is the name of the hidden CSRF field of webui forms.
const csrfFieldName = session.CSRFField

// htmxUnsafeAttrs are the htmx attributes of requests other than GET.
var htmxUnsafeAttrs = []string{"hx-post", "hx-put", "hx-patch", "hx-delete"}

// PostFormsCarryCSRF requires that every state-changing request the page
// can make carries the CSRF field ([session.CSRFField], rendered by
// [session.CSRFInput]) with the value token, or any non-empty value when
// token is "":
//
//   - every form with method="post" owns an enabled input of that name
//     (as a descendant, or through the form attribute), exactly one, and
//     it is the first control in tree order that submits a value of that
//     name, as [FormData] constructs the submission: the server reads the
//     first one ([session.Forms.Read]), so a select of that name before it
//     that submits an option, a textarea of that name, a control whose
//     dirname is that name, or a submit button of that name (which a
//     submission with it sends at its place in tree order) takes the place
//     of the token, and the submission is refused;
//   - a submit button with formmethod="post" in a form of another method
//     submits a form that owns one, first among the values of that name
//     that its submission sends;
//   - every element with hx-post, hx-put, hx-patch or hx-delete (also with
//     a data- prefix) sends it first: the first value of that name among
//     the parameters of its request, as [HTMXData] collects them, is the
//     token. They are the fields of the element's form (htmx includes the
//     related form in such requests, without its submit buttons), the
//     element itself, and the elements of its hx-include with the inputs,
//     selects and textareas in them, in tree order (a select sends its
//     value, also without options; the parts of an hx-include that
//     HTMXData cannot resolve add nothing); the fields of the form replace
//     those of their names, the merged hx-vals of the element and its
//     ancestors (nearer keys win; "unset" on any of them drops them all)
//     then replace those of theirs, and hx-params filters them ("none", a
//     "not" list that names the field, or a list without it drops it). A
//     form that owns the field thus fails with an hx-vals that sets
//     another value of that name.
//
// hx-include and hx-params are inherited as htmx inherits them: from the
// nearest element with a non-empty value, unless an "unset" value or an
// hx-disinherit that names them stops the inheritance on the way.
// Fields in the content of a template element are not part of the
// document, and fields in a datalist are no part of a form's submission:
// neither counts there.
//
// The CSRF token is a second defense beside the same-origin policy
// (mw.SameOrigin), which depends on browsers sending fetch metadata.
// Messages never show the token. A GET form with a formmethod="post"
// button cannot satisfy both this rule and [NoTokenLeaks]: give the POST
// action a form of its own.
//
// An hx-delete that carries the field satisfies both rules under webui's
// htmx configuration: the zero hx2.Config (page.Document.HTMX) sends the
// parameters of DELETE requests in the body (methodsThatUseUrlParams
// ["get"]), which the form package reads (net/http's ParseForm reads only
// POST, PUT and PATCH bodies). Under htmx's own default (["get",
// "delete"]), which hx2.Config.DeleteURLParams restores, a DELETE request
// has no body and puts its parameters, the CSRF field too, in the URL,
// which [NoTokenLeaks] reports; use hx-post, hx-put or hx-patch there.
// [NoTokenLeaks] says where it takes the configuration from.
func PostFormsCarryCSRF(token string) ct.Rule {
	desc := "postFormsCarryCSRF()"
	if token != "" {
		desc = "postFormsCarryCSRF(token)"
	}
	return ct.CustomRule(desc, func(root ct.Node) []string {
		idx := indexForms(root)
		var out []string
		for _, f := range idx.forms {
			if formMethod(f) != "post" {
				continue
			}
			if p := csrfProblem(idx.controls[f], token, nil, true); p != "" {
				out = append(out, fmt.Sprintf("post form at %s %s", loc(f), p))
			}
		}
		var sc *htmxScope // of the htmx requests, built for the first
		for e := range ct.Elements(root) {
			if submitButton(e) {
				if m, ok := attrOf(e, "formmethod"); ok && normMethod(m) == "post" {
					if f := idx.owner[e]; f != nil && formMethod(f) != "post" {
						if p := csrfProblem(idx.controls[f], token, e, false); p != "" {
							out = append(out, fmt.Sprintf("%s at %s submits with POST, but its form at %s %s", e.Tag(), loc(e), loc(f), p))
						}
					}
				}
			}
			problem, checked := "", false
			for _, name := range htmxUnsafeAttrs {
				_, attr, ok := hxAttrOf(e, name)
				if !ok {
					continue
				}
				if !checked {
					if sc == nil {
						sc = newHTMXScope(root)
					}
					problem, checked = htmxCSRFProblem(sc, idx, e, token), true
				}
				if problem != "" {
					out = append(out, fmt.Sprintf("%s at %s %s", attr, loc(e), problem))
				}
			}
		}
		return out
	})
}

func withValue(token string) string {
	if token == "" {
		return ""
	}
	return " with the expected value"
}

// csrfProblem describes what is wrong with the CSRF field among the
// controls of a form, in a submission by the submit button submitter (nil
// for none), or in every submission of the form when every is set, or
// returns "".
func csrfProblem(controls []ct.Node, token string, submitter ct.Node, every bool) string {
	var fields, enabled []ct.Node
	for _, c := range controls {
		if c.Tag() == "input" && AttrIs(c, "name", csrfFieldName) && inside(c, "datalist") == nil {
			fields = append(fields, c)
			if !disabledControl(c) {
				enabled = append(enabled, c)
			}
		}
	}
	switch {
	case len(fields) == 0:
		return fmt.Sprintf("has no %q field", csrfFieldName)
	case len(enabled) == 0:
		return fmt.Sprintf("has only a disabled %q field", csrfFieldName)
	case len(enabled) > 1:
		return fmt.Sprintf("has %d %q fields", len(enabled), csrfFieldName)
	}
	field := enabled[0]
	v, _ := attrOf(field, "value")
	if !csrfMatches(v, token) {
		if token == "" {
			return fmt.Sprintf("has an empty %q field", csrfFieldName)
		}
		return fmt.Sprintf("has a %q field with another value", csrfFieldName)
	}
	for _, c := range controls {
		if c == field {
			break
		}
		if what := csrfEntry(c, submitter, every); what != "" {
			return fmt.Sprintf("sends a %q value from the %s at %s before its %q field, and the server reads the first", csrfFieldName, what, loc(c), csrfFieldName)
		}
	}
	return ""
}

// csrfEntry describes c, a control of a form other than an input named
// session.CSRFField, when it adds a value of that name to the form's
// submission, as [FormData] constructs it: a select that submits an
// option, or a textarea, of that name, a control whose dirname is that
// name, or a submit button of that name when it is the submitter, or,
// when every is set, may submit the form with POST. It returns ""
// otherwise.
func csrfEntry(c, submitter ct.Node, every bool) string {
	if every && submitButton(c) {
		if m, ok := attrOf(c, "formmethod"); !ok || normMethod(m) == "post" {
			submitter = c
		}
	}
	// No radios: a radio named csrf is a field of that name (see
	// csrfProblem), and no radio has a dirname entry.
	for _, p := range entries(c, submitter, nil) {
		if p.Name != csrfFieldName {
			continue
		}
		if !AttrIs(c, "name", csrfFieldName) {
			return "dirname of the " + c.Tag()
		}
		if tag := c.Tag(); tag == "select" || tag == "textarea" {
			return tag
		}
		return "submit button"
	}
	return ""
}

func csrfMatches(v, token string) bool {
	if token == "" {
		return v != ""
	}
	return v == token
}

// carriesCSRF reports whether the element sends a matching CSRF field when
// htmx includes it: it is the field, a form that owns it, or (for other
// elements) contains it in its own tree (not in template content), before
// every select and textarea of that name.
func carriesCSRF(e ct.Node, idx formIndex, token string) bool {
	if e.Tag() == "form" {
		// htmx sends the fields of a form without its submit buttons.
		return csrfProblem(idx.controls[e], token, nil, false) == ""
	}
	if csrfField(e) {
		v, _ := attrOf(e, "value")
		return csrfMatches(v, token)
	}
	// htmx sends the values of the inputs, selects and textareas below e
	// (no buttons, no dirname) in tree order: the field must be the first
	// of that name.
	var fields []ct.Node
	var first ct.Node
	for d := range ct.Elements(e) {
		switch {
		case !sameTree(d, e):
			continue
		case csrfField(d):
			fields = append(fields, d)
		case d.Tag() == "select" || d.Tag() == "textarea":
			// htmx sends the value of such a control, also of a select
			// without options.
			if !AttrIs(d, "name", csrfFieldName) || inside(d, "datalist") != nil || disabledControl(d) {
				continue
			}
		default:
			continue
		}
		if first == nil {
			first = d
		}
	}
	if len(fields) != 1 || first != fields[0] {
		return false
	}
	v, _ := attrOf(fields[0], "value")
	return csrfMatches(v, token)
}

// htmxCSRFProblem describes what is wrong with the CSRF field of the htmx
// request of e, a request other than GET, or returns "": the first value
// of that name among the parameters that htmx sends, as [HTMXData]
// collects them, must match token. sc and idx are of the tree of e.
func htmxCSRFProblem(sc *htmxScope, idx formIndex, e ct.Node, token string) string {
	fields, _ := sc.params(idx, e, nil, "post", nil, true)
	for _, p := range fields {
		switch {
		case p.Name != csrfFieldName || p.File:
			continue
		case csrfMatches(p.Value, token):
			return ""
		case token == "":
			return fmt.Sprintf("sends an empty %q value first, and the server reads the first", csrfFieldName)
		}
		return fmt.Sprintf("sends a %q value other than the token first, and the server reads the first", csrfFieldName)
	}
	return fmt.Sprintf("sends no %q field%s", csrfFieldName, withValue(token))
}

// relatedForm returns the form that htmx includes in a request of e other
// than GET: the form owner of e, else its nearest form ancestor.
func relatedForm(e ct.Node, idx formIndex) ct.Node {
	if f := idx.owner[e]; f != nil {
		return f
	}
	return nearestForm(e)
}

// htmxVals returns the values that htmx adds to a request of e from
// hx-vals: the objects of e and its ancestors merged, nearer keys winning
// (a null value counts as absent), as htmx's getValuesForElement does.
// "unset" on any of them drops every hx-vals value, nearer ones included.
// Script values, which htmx ignores with allowEval false, and malformed
// values add nothing. A value without braces is wrapped in them, as htmx
// does.
func htmxVals(e ct.Node) map[string]any {
	vals := make(map[string]any)
	for p := e; p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		// htmx reads data-hx-vals when hx-vals is absent or empty.
		v, _ := attrOf(p, "hx-vals")
		if v == "" {
			v, _ = attrOf(p, "data-hx-vals")
		}
		v = strings.TrimSpace(v)
		switch {
		case v == "" || scriptValue(v):
			continue
		case v == "unset":
			return nil
		}
		if !strings.HasPrefix(v, "{") {
			v = "{" + v + "}"
		}
		var obj map[string]any
		if json.Unmarshal([]byte(v), &obj) != nil {
			continue
		}
		for k, x := range obj {
			if vals[k] == nil {
				vals[k] = x
			}
		}
	}
	return vals
}

// valsHave reports whether merged hx-vals contain one of variants in a key
// or a value; values that are not strings are compared as JSON.
func valsHave(vals map[string]any, variants []string) bool {
	for k, v := range vals {
		if containsAny(k, variants) {
			return true
		}
		text, ok := v.(string)
		if !ok {
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			text = string(b)
		}
		if containsAny(text, variants) {
			return true
		}
	}
	return false
}

// paramsKeepCSRF reports whether the hx-params of e, inherited as htmx
// inherits it ([closestHX]), lets htmx send the CSRF field: htmx sends no
// parameters for "none", drops the names of a "not a, b" list, and sends
// only the names of any other list; "*", and no value ("unset", empty or
// disinherited), filter nothing. The comparisons are exact, as in htmx:
// "None" is a list of one name.
func paramsKeepCSRF(e ct.Node) bool {
	v, _, ok := closestHX(e, "hx-params")
	switch {
	case !ok, v == "*":
		return true
	case v == "none":
		return false
	}
	if rest, ok := strings.CutPrefix(v, "not "); ok {
		return !namesCSRF(rest)
	}
	return namesCSRF(v)
}

// namesCSRF reports whether a comma-separated hx-params list names the
// CSRF field.
func namesCSRF(list string) bool {
	for name := range strings.SplitSeq(list, ",") {
		if strings.TrimSpace(name) == csrfFieldName {
			return true
		}
	}
	return false
}

// htmxURLMethods returns the methods whose htmx requests carry their
// parameters in the URL. A complete document runs under its own
// configuration: the methodsThatUseUrlParams of its htmx-config meta
// element (htmx reads the first one), else htmx's default, GET and DELETE
// (also for a meta element without the key or with malformed JSON, which
// htmx ignores). A fragment runs under the configuration of the page it is
// swapped into, whose methods fragment holds (nil: the zero hx2.Config);
// htmx never reads a meta element of swapped content.
func htmxURLMethods(root ct.Node, fragment map[string]bool) map[string]bool {
	if isFragment(root) {
		if fragment == nil {
			return webuiURLMethods()
		}
		return fragment
	}
	for m := range elementsByTag(root, "meta") {
		if n, _ := attrOf(m, "name"); !strings.EqualFold(n, "htmx-config") {
			continue
		}
		content, _ := attrOf(m, "content")
		if methods, ok := parseURLMethods(content); ok {
			return methods
		}
		break
	}
	return htmxDefaultURLMethods
}

// htmxDefaultURLMethods are htmx 2's default methodsThatUseUrlParams. The
// map is shared: never modify it.
var htmxDefaultURLMethods = map[string]bool{"get": true, "delete": true}

// webuiURLMethods returns the methodsThatUseUrlParams of the zero
// hx2.Config. The map is shared: never modify it.
var webuiURLMethods = sync.OnceValue(func() map[string]bool {
	return configURLMethods("webuitest", hx2.Config{})
})

// configURLMethods returns the methodsThatUseUrlParams of an htmx
// configuration (see parseURLMethods). A config that does not encode
// panics with the message prefix who: it is a mistake in the test.
func configURLMethods(who string, c hx2.Config) map[string]bool {
	content, err := c.JSON()
	if err != nil {
		panic(fmt.Sprintf("%s: %v", who, err))
	}
	methods, ok := parseURLMethods(content)
	if !ok {
		// hx2.Config always writes the key.
		panic(fmt.Sprintf("%s: the htmx config has no methodsThatUseUrlParams", who))
	}
	return methods
}

// parseURLMethods reads methodsThatUseUrlParams from the JSON of an
// htmx-config meta element, as written: htmx compares the verb of a
// request with them case-sensitively, and its verbs are lower-case
// ("get"), unless a submitter's formmethod writes another case. It
// returns false when the JSON is malformed or lacks the key.
func parseURLMethods(content string) (map[string]bool, bool) {
	var cfg struct {
		Methods []string `json:"methodsThatUseUrlParams"`
	}
	if json.Unmarshal([]byte(content), &cfg) != nil || cfg.Methods == nil {
		return nil, false
	}
	out := make(map[string]bool, len(cfg.Methods))
	for _, x := range cfg.Methods {
		out[x] = true
	}
	return out, true
}

// includeTargets resolves the elements of the hx-include of e, inherited as
// htmx inherits it ([closestHX]), that this package can resolve: "this",
// "closest form" and "#id" selectors. htmx resolves "this" only as the
// whole value, compared exactly, to the element that declares the
// hx-include; in a list it is a CSS type selector, which matches nothing.
func includeTargets(e ct.Node, ids map[string]ct.Node) []ct.Node {
	v, from, ok := closestHX(e, "hx-include")
	if !ok {
		return nil
	}
	if v == "this" {
		return []ct.Node{from}
	}
	var out []ct.Node
	for _, part := range splitSelectors(v) {
		switch {
		case part == "closest form":
			if f := ct.Closest(e, func(n ct.Node) bool { return n.Tag() == "form" }); f != nil {
				out = append(out, f)
			}
		case simpleIDSelector(part):
			if t := ids[part[1:]]; t != nil {
				out = append(out, t)
			}
		}
	}
	return out
}

// NoTokenLeaks forbids placements that would leak the CSRF token: a token
// in a URL reaches the browser history, server and proxy logs, and the
// Referer header of other sites. It reports:
//
//   - a form with method GET (the default) that owns an enabled input
//     named [session.CSRFField];
//   - a submit button with formmethod="get" in a form that owns one;
//   - an htmx request that puts its parameters in the URL (also with a
//     data- prefix) and sends the field: the element is such a form or the
//     field, its related form owns it (htmx includes it for every method
//     but GET), its hx-include names the field or an element containing
//     it, or the merged hx-vals of the element and its ancestors have it
//     (or, when token is not "", the token anywhere), unless its hx-params
//     drops it; hx-include and hx-params are inherited as for
//     [PostFormsCarryCSRF]. The methods that put their
//     parameters in the URL are the methodsThatUseUrlParams of the htmx
//     configuration, compared case-sensitively as htmx does ("GET" does
//     not name hx-get): in a complete document, of its first htmx-config
//     meta element, else htmx's default (hx-get and hx-delete); in a
//     fragment, of the page it is swapped into, which is the zero
//     hx2.Config here (hx-get only) and [WithHTMXConfig] in [Rules]. A
//     fragment answer parsed with [cthttp.Result.HTML] is a document
//     without that meta element, under htmx's default: parse it with
//     [cthttp.Result.HTMLFragment] (see [Rules]);
//   - a control of that name other than a hidden input: an input of
//     another type, a select, a textarea or a button, which shows the
//     token or sends a value of its own in its place;
//   - when token is not "": the token in any attribute other than the value
//     of the CSRF field, hx-vals and hx-headers (URL attributes are named as
//     such), and in any text or comment, also percent-encoded.
//
// Fields in template content or in a datalist are never submitted and do
// not count. Messages never show the token.
func NoTokenLeaks(token string) ct.Rule {
	return noTokenLeaks(token, nil)
}

// noTokenLeaks is [NoTokenLeaks] for fragments of pages whose
// methodsThatUseUrlParams are fragmentMethods (nil: the zero hx2.Config).
func noTokenLeaks(token string, fragmentMethods map[string]bool) ct.Rule {
	desc := "noTokenLeaks()"
	if token != "" {
		desc = "noTokenLeaks(token)"
	}
	var variants []string
	if token != "" {
		variants = encodedVariants(token)
	}
	return ct.CustomRule(desc, func(root ct.Node) []string {
		idx := indexForms(root)
		ids := documentIDs(root)
		urlMethods := htmxURLMethods(root, fragmentMethods)
		var out []string
		for _, f := range idx.forms {
			has := false
			for _, c := range idx.controls[f] {
				if csrfField(c) {
					has = true
				}
			}
			if !has {
				continue
			}
			switch formMethod(f) {
			case "get":
				out = append(out, fmt.Sprintf("get form at %s owns the %q field, which puts the token in the URL", loc(f), csrfFieldName))
			case "post":
				for _, c := range idx.controls[f] {
					if m, ok := attrOf(c, "formmethod"); ok && submitButton(c) && normMethod(m) == "get" {
						out = append(out, fmt.Sprintf("%s at %s submits the %q field with GET", c.Tag(), loc(c), csrfFieldName))
					}
				}
			}
		}
		for e := range ct.Elements(root) {
			for _, name := range htmxRequestAttrs {
				_, attr, ok := hxAttrOf(e, name)
				if !ok || !urlMethods[strings.TrimPrefix(name, "hx-")] || !paramsKeepCSRF(e) {
					continue
				}
				switch {
				case htmxURLCarriesCSRF(e, name != "hx-get", idx, ids):
					out = append(out, fmt.Sprintf("%s at %s sends the %q field in the URL", attr, loc(e), csrfFieldName))
				case variants != nil && valsHave(htmxVals(e), variants):
					out = append(out, fmt.Sprintf("%s at %s sends the token in the URL (hx-vals)", attr, loc(e)))
				}
			}
			if visibleCSRFControl(e) {
				out = append(out, fmt.Sprintf("%q field at %s is not hidden", csrfFieldName, loc(e)))
			}
		}
		if token == "" {
			return out
		}
		for d := range ct.Descendants(root) {
			switch d.Kind() {
			case ct.KindElement:
				urls := make(map[string]bool)
				for _, u := range urlAttrs(d) {
					urls[strings.ToLower(u.attr)] = true
				}
				for key, v := range d.Attrs() {
					lk := strings.ToLower(key)
					if lk == "value" && d.Tag() == "input" && AttrIs(d, "name", csrfFieldName) {
						continue
					}
					if n := hxName(lk); n == "hx-vals" || n == "hx-headers" {
						continue
					}
					if !containsAny(v, variants) {
						continue
					}
					if urls[lk] {
						out = append(out, fmt.Sprintf("%s at %s has the token in a URL", key, loc(d)))
					} else {
						out = append(out, fmt.Sprintf("%s at %s has the token", key, loc(d)))
					}
				}
			case ct.KindText, ct.KindRaw:
				if containsAny(d.Data(), variants) {
					out = append(out, "text at "+loc(d)+" shows the token")
				}
			case ct.KindComment:
				if containsAny(d.Data(), variants) {
					out = append(out, "comment at "+loc(d)+" has the token")
				}
			}
		}
		return out
	})
}

// visibleCSRFControl reports whether e is a control named
// session.CSRFField other than a hidden input: an input of another type, a
// select, a textarea or a button.
func visibleCSRFControl(e ct.Node) bool {
	switch e.Tag() {
	case "input":
		if inputType(e) == "hidden" {
			return false
		}
	case "select", "textarea", "button":
	default:
		return false
	}
	return AttrIs(e, "name", csrfFieldName)
}

// htmxURLCarriesCSRF reports whether an htmx request of e that carries its
// parameters in the URL includes a CSRF field (of any value): e is a form
// that owns one or the field itself, its related form owns one (related:
// for methods other than GET), an hx-include target has one, or the merged
// hx-vals have the field.
func htmxURLCarriesCSRF(e ct.Node, related bool, idx formIndex, ids map[string]ct.Node) bool {
	if (e.Tag() == "form" || csrfField(e)) && carriesCSRF(e, idx, "") {
		return true
	}
	if related {
		if f := relatedForm(e, idx); f != nil && carriesCSRF(f, idx, "") {
			return true
		}
	}
	for _, target := range includeTargets(e, ids) {
		if carriesCSRF(target, idx, "") {
			return true
		}
	}
	return htmxVals(e)[csrfFieldName] != nil
}

// encodedVariants returns v and its percent-encoded forms, as they appear
// in URLs.
func encodedVariants(v string) []string {
	out := []string{v}
	for _, e := range []string{url.QueryEscape(v), url.PathEscape(v)} {
		if e != v && !contains(out, e) {
			out = append(out, e)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// checkValues panics on empty secret values: an empty value would match
// everything, which is a mistake in the test.
func checkValues(rule string, values []string) {
	for i, v := range values {
		if v == "" {
			panic(fmt.Sprintf("webuitest.%s: value %d is empty", rule, i+1))
		}
	}
}

// SecretsHidden requires that each of values (secrets the page holds but
// must not show, such as a revealed key kept behind a disclosure) appears
// only in text inside a template element, or inside a closed details
// element outside its summary, and in no attribute, comment or other text:
// so it never reaches a URL, a form value, an accessible name or the
// visible page. A closed details element hides its content from view but
// not from find-in-page, which opens it on a match; only a template keeps
// a value out of the rendered document.
//
// Each text node is checked by itself: a value split across elements is
// not found. Messages name a value by its position in values, never by the
// value. SecretsHidden panics if a value is empty.
func SecretsHidden(values ...string) ct.Rule {
	checkValues("SecretsHidden", values)
	values = append([]string(nil), values...)
	return ct.CustomRule(fmt.Sprintf("secretsHidden(%d values)", len(values)), func(root ct.Node) []string {
		var out []string
		report := func(where string, s string) {
			for i, v := range values {
				if strings.Contains(s, v) {
					out = append(out, fmt.Sprintf("secret %d %s", i+1, where))
				}
			}
		}
		for d := range ct.Descendants(root) {
			switch d.Kind() {
			case ct.KindElement:
				for key, v := range d.Attrs() {
					report(fmt.Sprintf("in attribute %s at %s", key, loc(d)), v)
				}
			case ct.KindText, ct.KindRaw:
				if !hiddenText(d) {
					report("in visible text at "+loc(d), d.Data())
				}
			case ct.KindComment:
				report("in a comment at "+loc(d), d.Data())
			}
		}
		return out
	})
}

// hiddenText reports whether a text node is inside a template, or inside
// a closed details element and outside its summary.
func hiddenText(n ct.Node) bool {
	if inside(n, "template") != nil {
		return true
	}
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement || p.Tag() != "details" || hasAttr(p, "open") {
			continue
		}
		if s := firstChild(p, "summary"); s != nil && isAncestor(s, n) {
			continue
		}
		return true
	}
	return false
}

// firstChild returns the first element child of n with the tag, or the
// first element child when tag is "".
func firstChild(n ct.Node, tag string) ct.Node {
	for c := range n.Children() {
		if c.Kind() == ct.KindElement && (tag == "" || c.Tag() == tag) {
			return c
		}
	}
	return nil
}

// NotInURLs forbids each of values in every URL attribute ([SafeURLs]
// lists them), also percent-encoded: values that belong in form bodies,
// such as file paths or search terms of private data, must not reach the
// location bar, the history or server logs. It also reports each of values
// among what a form submits into a URL: the controls (value attributes,
// textarea text, every option of a select, every submit button) of a form
// that submits with GET (its method, the default; a formmethod="get"
// submit button; or an htmx request of the form that carries its
// parameters in the URL, see [NoTokenLeaks] for which do, in documents and
// in fragments), and the merged hx-vals of an htmx request that carries its
// parameters in the URL. Control values are matched as written, since the
// browser encodes them when it submits. Messages name a value by its
// position, never by the value. NotInURLs panics if a value is empty.
func NotInURLs(values ...string) ct.Rule {
	return notInURLs(nil, values...)
}

// notInURLs is [NotInURLs] for fragments of pages whose
// methodsThatUseUrlParams are fragmentMethods (nil: the zero hx2.Config).
func notInURLs(fragmentMethods map[string]bool, values ...string) ct.Rule {
	checkValues("NotInURLs", values)
	variants := make([][]string, len(values))
	for i, v := range values {
		variants[i] = encodedVariants(v)
	}
	values = append([]string(nil), values...)
	return ct.CustomRule(fmt.Sprintf("notInURLs(%d values)", len(values)), func(root ct.Node) []string {
		var out []string
		urlMethods := htmxURLMethods(root, fragmentMethods)
		for e := range ct.Elements(root) {
			for _, u := range urlAttrs(e) {
				for i, vs := range variants {
					if containsAny(u.url, vs) {
						out = append(out, fmt.Sprintf("value %d in %s at %s", i+1, u.attr, loc(e)))
					}
				}
			}
			for _, name := range htmxRequestAttrs {
				_, attr, ok := hxAttrOf(e, name)
				if !ok || !urlMethods[strings.TrimPrefix(name, "hx-")] {
					continue
				}
				vals := htmxVals(e)
				for i := range values {
					if valsHave(vals, []string{values[i]}) {
						out = append(out, fmt.Sprintf("value %d in the hx-vals that %s at %s sends in the URL", i+1, attr, loc(e)))
					}
				}
			}
		}
		idx := indexForms(root)
		for _, f := range idx.forms {
			if !submitsInURL(f, idx, urlMethods) {
				continue
			}
			for _, c := range idx.controls[f] {
				for _, v := range controlValues(c) {
					for i := range values {
						if strings.Contains(v, values[i]) {
							out = append(out, fmt.Sprintf("value %d in a field of %s at %s, which the form at %s submits in the URL", i+1, c.Tag(), loc(c), loc(f)))
						}
					}
				}
			}
		}
		return out
	})
}

// submitsInURL reports whether a form can submit its fields in a URL: its
// method is GET, a submit button of it has formmethod="get", or the form
// itself makes an htmx request whose method carries its parameters in the
// URL. The declared method counts also for a form with hx-post: it is the
// form's submission without script.
func submitsInURL(f ct.Node, idx formIndex, urlMethods map[string]bool) bool {
	if formMethod(f) == "get" {
		return true
	}
	for _, c := range idx.controls[f] {
		if m, ok := attrOf(c, "formmethod"); ok && submitButton(c) && normMethod(m) == "get" {
			return true
		}
	}
	for _, name := range htmxRequestAttrs {
		if _, _, ok := hxAttrOf(f, name); ok && urlMethods[strings.TrimPrefix(name, "hx-")] {
			return true
		}
	}
	return false
}

// controlValues returns the values that a named, enabled control can put
// into a form submission: its value attribute (for any submit button or
// checkable input, not only a checked or pressed one), the text of a
// textarea, and every option of a select, which a user may pick.
func controlValues(c ct.Node) []string {
	if name, _ := attrOf(c, "name"); name == "" || disabledControl(c) || inside(c, "datalist") != nil {
		return nil
	}
	switch c.Tag() {
	case "textarea":
		return []string{rawText(c)}
	case "select":
		var out []string
		for o := range elementsByTag(c, "option") {
			v, ok := attrOf(o, "value")
			if !ok {
				v = collapse(rawText(o))
			}
			out = append(out, v)
		}
		return out
	case "button":
		if !submitButton(c) {
			return nil
		}
	case "input":
		switch inputType(c) {
		case "file", "image", "reset", "button":
			return nil
		}
	default:
		return nil
	}
	v, _ := attrOf(c, "value")
	return []string{v}
}
