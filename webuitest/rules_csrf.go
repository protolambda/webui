package webuitest

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/protolambda/chord/ct"

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
//     (as a descendant, or through the form attribute), exactly one;
//   - a submit button with formmethod="post" in a form of another method
//     submits a form that owns one;
//   - every element with hx-post, hx-put, hx-patch or hx-delete (also with
//     a data- prefix) sends one: it is a form that owns the field, its form
//     owns it (htmx includes the related form in such requests), the
//     nearest hx-vals of the element or its ancestors has the field as a
//     JSON key, or the nearest hx-include names the field, an element
//     containing it, "this", or "closest form" that owns it.
//
// The CSRF token is a second defense beside the same-origin policy
// (mw.SameOrigin), which depends on browsers sending fetch metadata.
// Messages never show the token. A GET form with a formmethod="post"
// button cannot satisfy both this rule and [NoTokenLeaks]: give the POST
// action a form of its own.
func PostFormsCarryCSRF(token string) ct.Rule {
	desc := "postFormsCarryCSRF()"
	if token != "" {
		desc = "postFormsCarryCSRF(token)"
	}
	return ct.CustomRule(desc, func(root ct.Node) []string {
		idx := indexForms(root)
		ids := idIndex(root)
		var out []string
		for _, f := range idx.forms {
			if formMethod(f) != "post" {
				continue
			}
			if p := csrfProblem(idx.controls[f], token); p != "" {
				out = append(out, fmt.Sprintf("post form at %s %s", loc(f), p))
			}
		}
		for e := range Elements(root, "") {
			if submitButton(e) {
				if m, ok := attrOf(e, "formmethod"); ok && normMethod(m) == "post" {
					if f := idx.owner[e]; f != nil && formMethod(f) != "post" {
						if p := csrfProblem(idx.controls[f], token); p != "" {
							out = append(out, fmt.Sprintf("%s at %s submits with POST, but its form at %s %s", e.Tag(), loc(e), loc(f), p))
						}
					}
				}
			}
			for _, name := range htmxUnsafeAttrs {
				if _, attr, ok := hxAttrOf(e, name); ok && !htmxCarriesCSRF(e, idx, ids, token) {
					out = append(out, fmt.Sprintf("%s at %s sends no %q field%s", attr, loc(e), csrfFieldName, withValue(token)))
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
// controls of a form, or returns "".
func csrfProblem(controls []ct.Node, token string) string {
	var fields, enabled []ct.Node
	for _, c := range controls {
		if c.Tag() == "input" && AttrIs(c, "name", csrfFieldName) {
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
	v, _ := attrOf(enabled[0], "value")
	if !csrfMatches(v, token) {
		if token == "" {
			return fmt.Sprintf("has an empty %q field", csrfFieldName)
		}
		return fmt.Sprintf("has a %q field with another value", csrfFieldName)
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
// elements) contains it.
func carriesCSRF(e ct.Node, idx formIndex, token string) bool {
	if e.Tag() == "form" {
		return csrfProblem(idx.controls[e], token) == ""
	}
	if csrfField(e) {
		v, _ := attrOf(e, "value")
		return csrfMatches(v, token)
	}
	var fields []ct.Node
	for d := range Elements(e, "input") {
		if csrfField(d) {
			fields = append(fields, d)
		}
	}
	if len(fields) != 1 {
		return false
	}
	v, _ := attrOf(fields[0], "value")
	return csrfMatches(v, token)
}

// htmxCarriesCSRF reports whether an element's non-GET htmx request sends
// the CSRF field.
func htmxCarriesCSRF(e ct.Node, idx formIndex, ids map[string]ct.Node, token string) bool {
	if e.Tag() == "form" && carriesCSRF(e, idx, token) {
		return true
	}
	related := idx.owner[e]
	if related == nil {
		related = inside(e, "form")
	}
	if related != nil && carriesCSRF(related, idx, token) {
		return true
	}
	if v, ok := inherited(e, "hx-vals"); ok && valsCarry(v, token) {
		return true
	}
	for _, target := range includeTargets(e, ids) {
		if carriesCSRF(target, idx, token) {
			return true
		}
	}
	return false
}

// inherited returns the value of an inherited htmx attribute: from the
// element or its nearest ancestor that has it.
func inherited(e ct.Node, name string) (string, bool) {
	for p := e; p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		if v, _, ok := hxAttrOf(p, name); ok {
			return v, true
		}
	}
	return "", false
}

// valsCarry reports whether an hx-vals JSON object has the CSRF field.
func valsCarry(v string, token string) bool {
	if scriptValue(v) {
		return false
	}
	var vals map[string]any
	if err := json.Unmarshal([]byte(v), &vals); err != nil {
		return false
	}
	s, ok := vals[csrfFieldName].(string)
	return ok && csrfMatches(s, token)
}

// includeTargets resolves the elements of the nearest hx-include of e that
// this package can resolve: "this", "closest form" and "#id" selectors.
func includeTargets(e ct.Node, ids map[string]ct.Node) []ct.Node {
	v, ok := inherited(e, "hx-include")
	if !ok {
		return nil
	}
	var out []ct.Node
	for _, part := range splitSelectors(v) {
		switch {
		case part == "this":
			out = append(out, e)
		case part == "closest form":
			if f := Closest(e, func(n ct.Node) bool { return n.Tag() == "form" }); f != nil {
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
//   - a form with method GET (the default) that owns a control named
//     [session.CSRFField];
//   - a submit button with formmethod="get" in a form that owns one;
//   - an element with hx-get (also data-hx-get) that is such a form or whose
//     nearest hx-include names the field or an element containing it;
//   - a field of that name that is not of type hidden;
//   - when token is not "": the token in any attribute other than the value
//     of the CSRF field, hx-vals and hx-headers (URL attributes are named as
//     such), and in any text or comment, also percent-encoded.
//
// Messages never show the token.
func NoTokenLeaks(token string) ct.Rule {
	desc := "noTokenLeaks()"
	if token != "" {
		desc = "noTokenLeaks(token)"
	}
	return ct.CustomRule(desc, func(root ct.Node) []string {
		idx := indexForms(root)
		ids := idIndex(root)
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
		for e := range Elements(root, "") {
			if _, attr, ok := hxAttrOf(e, "hx-get"); ok {
				leaks := e.Tag() == "form" && carriesCSRF(e, idx, "")
				for _, target := range includeTargets(e, ids) {
					if carriesCSRF(target, idx, "") {
						leaks = true
					}
				}
				if leaks {
					out = append(out, fmt.Sprintf("%s at %s sends the %q field in the URL", attr, loc(e), csrfFieldName))
				}
			}
			if e.Tag() == "input" && AttrIs(e, "name", csrfFieldName) && inputType(e) != "hidden" {
				out = append(out, fmt.Sprintf("%q field at %s is not hidden", csrfFieldName, loc(e)))
			}
		}
		if token == "" {
			return out
		}
		variants := encodedVariants(token)
		for d := range Descendants(root) {
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
		for d := range Descendants(root) {
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
// location bar, the history or server logs. Messages name a value by its
// position, never by the value. NotInURLs panics if a value is empty.
func NotInURLs(values ...string) ct.Rule {
	checkValues("NotInURLs", values)
	variants := make([][]string, len(values))
	for i, v := range values {
		variants[i] = encodedVariants(v)
	}
	return ct.CustomRule(fmt.Sprintf("notInURLs(%d values)", len(values)), func(root ct.Node) []string {
		var out []string
		for e := range Elements(root, "") {
			for _, u := range urlAttrs(e) {
				for i, vs := range variants {
					if containsAny(u.url, vs) {
						out = append(out, fmt.Sprintf("value %d in %s at %s", i+1, u.attr, loc(e)))
					}
				}
			}
		}
		return out
	})
}
