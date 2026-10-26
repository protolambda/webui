package webuitest

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"

	"github.com/protolambda/webui/hx2"
)

// HTMXRequest is the request that htmx 2 sends when an element triggers
// its request (hx-get, hx-post, hx-put, hx-patch or hx-delete): [HTMXData]
// computes it from a page, and [HTMXRequest.Request] (or
// [Client.SubmitHTMX] and [Browser.SubmitHTMX]) sends it.
type HTMXRequest struct {
	// Method is "GET", "POST", "PUT", "PATCH" or "DELETE".
	Method string
	// Path is the request URL as the page writes it: the value of the
	// element's request attribute, or the formaction of the submitter of a
	// form. As for [Submission.Action], "" means the page's URL and a
	// reference without a leading slash is relative to it; sending either
	// needs [PageURL].
	Path string
	// URLParams reports that the parameters go in the query of the URL and
	// the request has no body: the method, as htmx compares it (a
	// formmethod as written, see [HTMXData]), is one of the
	// methodsThatUseUrlParams of the htmx configuration.
	URLParams bool
	// Multipart reports a multipart/form-data body: hx-encoding is
	// "multipart/form-data" on the element or an ancestor, or the element
	// is a form with that enctype. Otherwise the body is urlencoded.
	Multipart bool
	// Fields are the parameters, in the order htmx sends them. Line breaks
	// stay as the page and the edits have them; a multipart body sends them
	// as CRLF, as browsers encode multipart bodies.
	Fields []Part
	// Trigger and TriggerName are the id and the name of the element, sent
	// as HX-Trigger and HX-Trigger-Name; "" sends none.
	Trigger, TriggerName string
	// Target is the id of the element that htmx swaps the response into,
	// sent as HX-Target; "" sends none.
	Target string
	// Header holds the headers of the hx-headers of the element and its
	// ancestors, merged as htmx merges them, with the values that the
	// browser sends: without leading and trailing white space, and with
	// characters up to U+00FF as one byte each (Latin-1, which
	// hx2.RequestHeaders decodes). A value that the browser rejects (other
	// characters, or a NUL, CR or LF inside) is percent-encoded, with the
	// name's -URI-AutoEncoded header, as htmx then sends it. It leaves out
	// the headers that htmx sets itself and those that browsers do not let
	// scripts set (Origin, Cookie, Sec-*, ..., and X-HTTP-Method,
	// X-HTTP-Method-Override and X-Method-Override when they name CONNECT,
	// TRACE or TRACK). Nil means none.
	Header http.Header

	// unresolved is the hx-target that HTMXData could not resolve within
	// the subject, which Request then needs HXTarget for.
	unresolved string
	// verb is the method as htmx compares it (see htmxVerb).
	verb string
}

// htmxVerb returns the method as htmx compares it with "get" and the
// methodsThatUseUrlParams: a submitter's formmethod as the page writes it
// ("GET" is not "get"), else the lower-cased Method, also when Method was
// changed to another method after HTMXData.
func (r HTMXRequest) htmxVerb() string {
	if r.verb != "" && strings.EqualFold(r.verb, r.Method) {
		return r.verb
	}
	return strings.ToLower(r.Method)
}

// Values returns the text fields of the request.
func (r HTMXRequest) Values() url.Values {
	out := make(url.Values)
	for _, p := range r.Fields {
		if !p.File {
			out[p.Name] = append(out[p.Name], p.Value)
		}
	}
	return out
}

// htmxVerbs are the methods of htmx requests, lower-cased as htmx writes
// them.
var htmxVerbs = []string{"get", "post", "put", "patch", "delete"}

// firstRequestAttr returns the method, attribute name and URL of the
// first htmx request attribute of e (hx-get, ..., also with a data-
// prefix).
func firstRequestAttr(e ct.Node) (verb, attr, path string, ok bool) {
	for _, v := range htmxVerbs {
		if path, attr, ok := hxAttrOf(e, "hx-"+v); ok {
			return v, attr, path, true
		}
	}
	return "", "", "", false
}

// HTMXData computes the request that htmx 2 sends when elt triggers its
// request: a form when it is submitted, a button when it is clicked, an
// input when it changes. elt is a selection of page that must match one
// element with one of hx-get, hx-post, hx-put, hx-patch and hx-delete (also
// with a data- prefix). submitter, for a form element only, is the submit
// button that submits it (nil for none, as form.requestSubmit()): its
// formaction and formmethod replace the form's request URL and method, and
// its name and value are sent. htmx compares the formmethod as written:
// formmethod="GET" is not "get", so the form's fields go in a body, which
// a GET request does not send, with a urlencoded Content-Type, and
// formmethod="DELETE" is not the "delete" of methodsThatUseUrlParams.
// edits replace the values of the named controls, as a user would enter
// them, before hx-vals and hx-params apply; editing a name that is no
// control of the request is an error.
//
// A submitter that htmx would not use for the form's request is an error:
//
//   - one with its own request attribute (hx-delete, ...), whose click
//     htmx handles (no hx-trigger, or one with a click event, and not
//     inside hx-disable): htmx sends that request instead, and cancels the
//     submission; pass the submitter as elt, with a nil submitter;
//   - one that htmx does not track as the submitter, when it has a name,
//     formaction or formmethod that would then be left out: htmx tracks
//     the submit buttons inside a form element (any form, also when the
//     form attribute names another), and outside of forms only those whose
//     type attribute is exactly "submit" (not absent, not "SUBMIT") and
//     that have a form attribute, outside hx-disable; a footer button
//     <button form="f"> needs type="submit".
//
// It follows htmx's algorithm for the parameters:
//
//   - for every method but GET, the fields of the element's form (its form
//     owner, else its nearest form ancestor), as the browser constructs
//     them without a submitter (see [FormData]), but without file inputs
//     that have no file;
//   - the element itself, when it is a named control that is not disabled
//     (also by any disabled fieldset ancestor), not a button of type
//     submit or button, and not an unchecked checkbox or radio, with its
//     current value; a select sends its selected options, also disabled
//     ones; a form adds its fields;
//   - the name and value of the submitter, or of the element itself when
//     it is a button or a submit input;
//   - the elements of the nearest hx-include, each as the element itself,
//     with the inputs, textareas and selects inside (no fields twice);
//   - the fields of the form replace those of the same name of the element
//     and the includes; then the values of the merged hx-vals of the
//     element and its ancestors (see [PostFormsCarryCSRF]; strings as they
//     are, arrays as one field per item, objects and null as JavaScript's
//     JSON.stringify writes them) replace those of the same name, and the
//     nearest hx-params filters the result.
//
// The parameters go in the URL when the method is one of the
// methodsThatUseUrlParams of the configuration that the subject runs
// under, as [NoTokenLeaks] reads it, compared case-sensitively as htmx
// does: a complete document's first htmx-config meta element, else
// htmx's default (get and delete); for a fragment, the zero hx2.Config of
// the page around it (get only; [PageHTMXConfig] sets another for
// [Client.SubmitHTMX] and [Browser.SubmitHTMX], or set URLParams).
//
// An hx-headers member whose name is not a valid header name is an error:
// the browser rejects it, and htmx sends no request.
//
// The target is the element that the nearest hx-target selects, else the
// element itself. Selectors are resolved within the subject when they are
// "this", or lists of closest, find, next and previous with simple
// selectors (a tag, an id, classes and attribute tests, as [HXTargets]
// takes them), next, previous, body and simple selectors. In a fragment, an
// "#id" that the fragment does not contain names an element of the page.
// A target that HTMXData cannot resolve, such as a closest target outside
// the fragment or a selector with combinators, is left to
// [HTMXRequest.Request], which then needs [HXTarget]; one that selects
// nothing in a document is an error, as htmx then sends no request. An
// hx-include that it cannot resolve is an error. So is, when the request
// is built ([HTMXRequest.Request]), a request URL of another origin than
// the page's.
//
// HTMXData does not run what needs a user, script or an extension:
// hx-confirm, hx-prompt, hx-vars, js: values, hx-on, constraint
// validation, hx-boost, hx-request and extensions. hx-disinherit and an
// "unset" value stop the inheritance of hx-target, hx-include, hx-params
// and hx-encoding, as in htmx; hx-inherit, which htmx reads only with
// htmx.config.disableInheritance (off in webui's configuration), is
// ignored. An element in template content or inside hx-disable makes no
// request, and a disabled one cannot be used: both are errors. Errors
// wrap [ErrFormData], or the load or query error of the selections.
func HTMXData(ctx context.Context, page *ct.Subject, elt, submitter *ct.Selection, edits url.Values) (HTMXRequest, error) {
	return htmxData(ctx, page, elt, submitter, edits, nil)
}

// htmxData is [HTMXData] for fragments of pages whose
// methodsThatUseUrlParams are fragmentMethods (nil: the zero hx2.Config).
func htmxData(ctx context.Context, page *ct.Subject, elt, submitter *ct.Selection, edits url.Values, fragmentMethods map[string]bool) (HTMXRequest, error) {
	doc, err := page.Load(ctx)
	if err != nil {
		return HTMXRequest{}, err
	}
	root := doc.Root()
	e, err := one(ctx, elt, "element")
	if err != nil {
		return HTMXRequest{}, err
	}
	if rootOf(e) != root {
		return HTMXRequest{}, fmt.Errorf("%w: %s is not part of the page", ErrFormData, elt)
	}
	verb, _, path, ok := firstRequestAttr(e)
	switch {
	case !ok:
		return HTMXRequest{}, fmt.Errorf("%w: %s has no hx-get, hx-post, hx-put, hx-patch or hx-delete", ErrFormData, elt)
	case requestAttrCount(e) > 1:
		return HTMXRequest{}, fmt.Errorf("%w: %s has several htmx request attributes", ErrFormData, elt)
	case inside(e, "template") != nil:
		return HTMXRequest{}, fmt.Errorf("%w: %s is template content, which htmx does not process", ErrFormData, elt)
	case underHXDisable(e):
		return HTMXRequest{}, fmt.Errorf("%w: %s is inside hx-disable, which htmx does not process", ErrFormData, elt)
	case listedTags[e.Tag()] && disabledControl(e):
		return HTMXRequest{}, fmt.Errorf("%w: %s is disabled", ErrFormData, elt)
	}
	idx := indexForms(root)
	var sub ct.Node
	if submitter != nil {
		if e.Tag() != "form" {
			return HTMXRequest{}, fmt.Errorf("%w: %s is a %s; only a form element has a submitter", ErrFormData, elt, e.Tag())
		}
		sub, err = one(ctx, submitter, "submitter")
		if err != nil {
			return HTMXRequest{}, err
		}
		switch {
		case !submitButton(sub) || sub.Tag() == "input" && inputType(sub) == "image":
			// htmx tracks buttons and submit inputs, not image inputs.
			return HTMXRequest{}, fmt.Errorf("%w: %s is not a submit button or submit input", ErrFormData, submitter)
		case idx.owner[sub] != e:
			return HTMXRequest{}, fmt.Errorf("%w: %s does not belong to the form", ErrFormData, submitter)
		case disabledControl(sub):
			return HTMXRequest{}, fmt.Errorf("%w: %s is disabled", ErrFormData, submitter)
		}
		if _, attr, _, ok := firstRequestAttr(sub); ok && htmxHandlesClick(sub) {
			return HTMXRequest{}, fmt.Errorf("%w: %s has its own %s, which htmx sends instead of the form's request when it is clicked; pass it as elt with a nil submitter", ErrFormData, submitter, attr)
		}
		if !htmxTracksSubmitter(sub) {
			if hasAttr(sub, "name") || hasAttr(sub, "formaction") || hasAttr(sub, "formmethod") {
				return HTMXRequest{}, fmt.Errorf(`%w: htmx does not track %s as the submitter of the form, so it would send the form's request without its name, value, formaction and formmethod: outside of form elements, htmx tracks only submit buttons with a form attribute and a type attribute of exactly "submit"`, ErrFormData, submitter)
			}
			// It changes nothing: the form's request without a submitter.
			sub = nil
		}
	}
	if sub != nil {
		if a, ok := attrOf(sub, "formaction"); ok {
			path = a
		}
		if m, ok := attrOf(sub, "formmethod"); ok {
			if !slices.Contains(htmxVerbs, strings.ToLower(m)) {
				return HTMXRequest{}, fmt.Errorf("%w: %s has formmethod %q, for which htmx sends no request", ErrFormData, submitter, m)
			}
			// htmx keeps the formmethod as written and compares it
			// case-sensitively.
			verb = m
		}
	}
	sc := newHTMXScope(root)
	r := HTMXRequest{
		Method:    strings.ToUpper(verb),
		Path:      path,
		URLParams: htmxURLMethods(root, fragmentMethods)[verb],
		Multipart: htmxMultipart(e),
		verb:      verb,
	}
	r.Target, r.unresolved, err = sc.target(e)
	if err != nil {
		return HTMXRequest{}, fmt.Errorf("%w: %s: %w", ErrFormData, elt, err)
	}
	r.Trigger, _ = attrOf(e, "id")
	r.TriggerName, _ = attrOf(e, "name")

	c := paramCollector{idx: idx, root: root, processed: make(map[ct.Node]bool), names: make(map[string]bool)}
	var priority, regular []Part
	if verb != "get" {
		// The element's form, as htmx finds it: e.form, else the closest
		// form, which is e itself for a form.
		f := e
		if e.Tag() != "form" {
			f = relatedForm(e, idx)
		}
		if f != nil {
			priority = c.process(priority, f)
		}
	}
	regular = c.process(regular, e)
	btn := sub
	if btn == nil {
		// htmx compares the type attribute as written.
		if t, _ := attrOf(e, "type"); e.Tag() == "button" || e.Tag() == "input" && t == "submit" {
			btn = e
		}
	}
	if btn != nil {
		if name, ok := attrOf(btn, "name"); ok {
			v, _ := attrOf(btn, "value")
			priority = append(priority, Part{Name: name, Value: v})
		}
	}
	includes, err := sc.includes(e)
	if err != nil {
		return HTMXRequest{}, fmt.Errorf("%w: %s: %w", ErrFormData, elt, err)
	}
	for _, n := range includes {
		regular = c.process(regular, n)
		if n.Tag() == "form" {
			continue
		}
		for d := range ct.Elements(n) {
			switch d.Tag() {
			case "input", "textarea", "select":
				if sameTree(d, n) {
					regular = c.process(regular, d)
				}
			}
		}
	}
	fields := overrideFields(regular, priority)
	for _, name := range slices.Sorted(maps.Keys(edits)) {
		if !c.names[name] {
			return HTMXRequest{}, fmt.Errorf("%w: the request of %s includes no control %q to edit", ErrFormData, elt, name)
		}
		parts := make([]Part, len(edits[name]))
		for i, v := range edits[name] {
			parts[i] = Part{Name: name, Value: v}
		}
		fields = replaceFields(fields, name, parts)
	}
	fields = overrideFields(fields, htmxValsFields(e))
	r.Fields = filterParams(fields, e)
	if r.Header, err = htmxHeaders(e, r); err != nil {
		return HTMXRequest{}, fmt.Errorf("%w: %s: %w", ErrFormData, elt, err)
	}
	return r, nil
}

// underHXDisable reports whether n is inside hx-disable (also with a data-
// prefix), where htmx processes no element.
func underHXDisable(n ct.Node) bool {
	return ct.Closest(n, func(p ct.Node) bool { _, _, ok := hxAttrOf(p, "hx-disable"); return ok }) != nil
}

// htmxHandlesClick reports whether htmx handles a click of n, an element
// with a request attribute: n is not inside hx-disable, and its own
// hx-trigger (htmx does not inherit it) is absent or empty, a button's
// default trigger "click", or lists a click event. For a submit button,
// htmx then sends n's own request and cancels the submission of its form.
func htmxHandlesClick(n ct.Node) bool {
	if underHXDisable(n) {
		return false
	}
	v, _ := attrOf(n, "hx-trigger")
	if v == "" {
		v, _ = attrOf(n, "data-hx-trigger")
	}
	triggers := splitSelectors(v)
	if len(triggers) == 0 {
		return true
	}
	for _, trig := range triggers {
		event, _, _ := strings.Cut(strings.Fields(trig)[0], "[")
		if event == "click" {
			return true
		}
	}
	return false
}

// htmxTracksSubmitter reports whether htmx 2 records the submit button n
// as the submitter of its form (lastButtonClicked). htmx records it from
// click and focusin listeners on every form element and on every element
// whose type attribute is exactly "submit" and that has a form (or
// data-form) attribute, outside hx-disable; the listener resolves the
// button's form owner. A click on n reaches such a listener when n or an
// ancestor has one.
func htmxTracksSubmitter(n ct.Node) bool {
	for p := n; p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		t, _ := attrOf(p, "type")
		listens := p.Tag() == "form" || t == "submit" && (hasAttr(p, "form") || hasAttr(p, "data-form"))
		if listens && !underHXDisable(p) {
			return true
		}
	}
	return false
}

// requestAttrCount counts the htmx request attributes of e.
func requestAttrCount(e ct.Node) int {
	n := 0
	for _, v := range htmxVerbs {
		if _, _, ok := hxAttrOf(e, "hx-"+v); ok {
			n++
		}
	}
	return n
}

// closestHX returns the value of an inherited htmx attribute as htmx's
// getClosestAttributeValue reads it: from the nearest of e and its
// ancestors with a non-empty value (an empty hx- attribute falls back to
// the data- one, see [hxValue]), with the element that declares it. Both
// "unset" there and an ancestor whose hx-disinherit is "*" or lists name
// mean none: the search stops. hx-inherit is not read: htmx reads it only
// with htmx.config.disableInheritance, which webui's configuration leaves
// off.
func closestHX(e ct.Node, name string) (string, ct.Node, bool) {
	for p := e; p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		if p != e && disinherits(p, name) {
			return "", nil, false
		}
		v, _ := hxValue(p, name)
		if v == "" {
			continue
		}
		if v == "unset" {
			return "", nil, false
		}
		return v, p, true
	}
	return "", nil, false
}

// disinherits reports whether the hx-disinherit of e keeps its descendants
// from inheriting the attribute name: it is "*" or a list of names
// separated by single spaces that has name, compared exactly, as htmx
// compares them.
func disinherits(e ct.Node, name string) bool {
	v, _ := hxValue(e, "hx-disinherit")
	return v == "*" || slices.Contains(strings.Split(v, " "), name)
}

// htmxMultipart reports whether htmx sends the request of e as
// multipart/form-data.
func htmxMultipart(e ct.Node) bool {
	if v, _, ok := closestHX(e, "hx-encoding"); ok && v == "multipart/form-data" {
		return true
	}
	t, _ := attrOf(e, "enctype")
	return e.Tag() == "form" && t == "multipart/form-data"
}

// paramCollector collects the parameters of an htmx request as htmx's
// processInputValue does, processing each element once.
type paramCollector struct {
	idx       formIndex
	root      ct.Node
	processed map[ct.Node]bool
	names     map[string]bool // of the controls considered, for edits
}

// process adds the value of n, if htmx includes it, to list, and the
// fields of n when it is a form.
func (c *paramCollector) process(list []Part, n ct.Node) []Part {
	if n == nil || c.processed[n] {
		return list
	}
	c.processed[n] = true
	c.noteName(n)
	if htmxIncluded(n, c) {
		name, _ := attrOf(n, "name")
		for _, v := range htmxValue(n, c) {
			list = append(list, Part{Name: name, Value: v})
		}
	}
	if n.Tag() != "form" {
		return list
	}
	controls := c.idx.controls[n]
	for _, ctl := range controls {
		if ctl.Tag() == "input" && inputType(ctl) == "image" {
			// Not one of the form's elements.
			continue
		}
		c.noteName(ctl)
		if !c.processed[ctl] {
			c.processed[ctl] = true
			continue
		}
		// The form's fields add it again: drop its value.
		if name, _ := attrOf(ctl, "name"); name != "" {
			list = removeValues(list, name, htmxValue(ctl, c))
		}
	}
	radios := checkedRadios(controls)
	for _, ctl := range controls {
		for _, p := range entries(ctl, nil, radios) {
			if p.File && p.Filename == "" {
				// htmx skips file inputs without a file.
				continue
			}
			list = append(list, p)
		}
	}
	return list
}

func (c *paramCollector) noteName(n ct.Node) {
	switch n.Tag() {
	case "input", "select", "textarea", "button", "output":
		if name, ok := attrOf(n, "name"); ok && name != "" {
			c.names[name] = true
		}
	}
}

// htmxIncluded reports whether htmx includes the value of the element n
// itself (htmx's shouldInclude): a named control that is not disabled, nor
// in a disabled fieldset (legends included), nor a button of type submit
// or button, nor an unchecked checkbox or radio.
func htmxIncluded(n ct.Node, c *paramCollector) bool {
	switch n.Tag() {
	case "input", "select", "textarea", "button", "output":
	default:
		return false
	}
	if name, _ := attrOf(n, "name"); name == "" {
		return false
	}
	if n.Tag() != "output" && hasAttr(n, "disabled") {
		return false
	}
	if ct.Closest(n, func(p ct.Node) bool { return p.Tag() == "fieldset" && hasAttr(p, "disabled") }) != nil {
		return false
	}
	switch n.Tag() {
	case "button":
		t, ok := attrOf(n, "type")
		t = strings.ToLower(strings.TrimSpace(t))
		return ok && t == "reset"
	case "input":
		switch inputType(n) {
		case "button", "submit":
			return false
		case "checkbox":
			return hasAttr(n, "checked")
		case "radio":
			return c.radioChecked(n)
		}
	}
	return true
}

// radioChecked reports whether the radio n is checked: the last radio of
// its group with the checked attribute.
func (c *paramCollector) radioChecked(n ct.Node) bool {
	if !hasAttr(n, "checked") {
		return false
	}
	name, _ := attrOf(n, "name")
	if f := c.idx.owner[n]; f != nil {
		return checkedRadios(c.idx.controls[f])[name] == n
	}
	var group []ct.Node
	for r := range elementsByTag(c.root, "input") {
		if inputType(r) == "radio" && c.idx.owner[r] == nil && sameTree(r, n) {
			group = append(group, r)
		}
	}
	return checkedRadios(group)[name] == n
}

// htmxValue returns the current value of a control as htmx reads it (its
// value property): the selected options of a multiple select, nothing for
// a file input without files, else one value.
func htmxValue(n ct.Node, c *paramCollector) []string {
	v, hasValue := attrOf(n, "value")
	switch n.Tag() {
	case "select":
		return selectedValues(n)
	case "textarea", "output":
		return []string{rawText(n)}
	case "button":
		return []string{v}
	case "input":
	default:
		return nil
	}
	switch t := inputType(n); t {
	case "file":
		return nil
	case "checkbox", "radio":
		if !hasValue {
			v = "on"
		}
		return []string{v}
	case "hidden", "submit", "image", "reset", "button":
		return []string{v}
	default:
		return []string{sanitizeValue(n, t, v)}
	}
}

// selectedValues returns the values of the selected options of a select,
// as its value property (a single select) or its checked options (a
// multiple select) have them, disabled options included.
func selectedValues(sel ct.Node) []string {
	var options, selected []ct.Node
	for o := range elementsByTag(sel, "option") {
		options = append(options, o)
		if hasAttr(o, "selected") {
			selected = append(selected, o)
		}
	}
	optionValue := func(o ct.Node) string {
		if v, ok := attrOf(o, "value"); ok {
			return v
		}
		return collapse(rawText(o))
	}
	if hasAttr(sel, "multiple") {
		out := make([]string, len(selected))
		for i, o := range selected {
			out[i] = optionValue(o)
		}
		return out
	}
	switch {
	case len(selected) > 0:
		return []string{optionValue(selected[len(selected)-1])}
	case selectSize(sel) == 1:
		for _, o := range options {
			if !hasAttr(o, "disabled") && !(o.Parent() != nil && o.Parent().Tag() == "optgroup" && hasAttr(o.Parent(), "disabled")) {
				return []string{optionValue(o)}
			}
		}
	}
	return []string{""}
}

func selectSize(sel ct.Node) int {
	if v, ok := attrOf(sel, "size"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

// removeValues drops the fields called name whose value is one of values.
func removeValues(list []Part, name string, values []string) []Part {
	out := list[:0:0]
	for _, p := range list {
		if p.Name == name && !p.File && slices.Contains(values, p.Value) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// overrideFields drops the fields of receiver that donor has a field of
// the same name for, and appends the fields of donor, as htmx's
// overrideFormData does.
func overrideFields(receiver, donor []Part) []Part {
	if len(donor) == 0 {
		return receiver
	}
	names := make(map[string]bool, len(donor))
	for _, p := range donor {
		names[p.Name] = true
	}
	out := make([]Part, 0, len(receiver)+len(donor))
	for _, p := range receiver {
		if !names[p.Name] {
			out = append(out, p)
		}
	}
	return append(out, donor...)
}

// filterParams applies the nearest hx-params of e to fields, as htmx's
// filterValues does: "none" drops every field, "*" none, "not a, b" the
// listed names, and any other list keeps only the listed names, in the
// order of the list.
func filterParams(fields []Part, e ct.Node) []Part {
	v, _, ok := closestHX(e, "hx-params")
	switch {
	case !ok || v == "*":
		return fields
	case v == "none":
		return nil
	}
	if rest, ok := strings.CutPrefix(v, "not "); ok {
		drop := make(map[string]bool)
		for name := range strings.SplitSeq(rest, ",") {
			drop[strings.TrimSpace(name)] = true
		}
		var out []Part
		for _, p := range fields {
			if !drop[p.Name] {
				out = append(out, p)
			}
		}
		return out
	}
	var out []Part
	for name := range strings.SplitSeq(v, ",") {
		name = strings.TrimSpace(name)
		for _, p := range fields {
			if p.Name == name {
				out = append(out, p)
			}
		}
	}
	return out
}

// jsObject is a JSON object with its members in the order in which
// JavaScript enumerates them: integer keys ascending, then the others in
// insertion order.
type jsObject struct {
	keys []string
	vals map[string]json.RawMessage
}

// set adds a member, or replaces the value of an existing one at its
// position, as an assignment does.
func (o *jsObject) set(k string, v json.RawMessage) {
	if o.vals == nil {
		o.vals = make(map[string]json.RawMessage)
	}
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

// isNull reports whether the object lacks k or has null for it.
func (o *jsObject) isNull(k string) bool {
	v, ok := o.vals[k]
	return !ok || isJSONNull(v)
}

func isJSONNull(v json.RawMessage) bool { return string(bytes.TrimSpace(v)) == "null" }

// ordered returns the keys in enumeration order.
func (o *jsObject) ordered() []string {
	var ints, others []string
	for _, k := range o.keys {
		if arrayIndex(k) {
			ints = append(ints, k)
		} else {
			others = append(others, k)
		}
	}
	slices.SortFunc(ints, func(a, b string) int {
		x, _ := strconv.ParseUint(a, 10, 32)
		y, _ := strconv.ParseUint(b, 10, 32)
		return cmp.Compare(x, y)
	})
	return append(ints, others...)
}

// arrayIndex reports whether k is a canonical array index, which
// JavaScript enumerates before other keys.
func arrayIndex(k string) bool {
	if k == "" || len(k) > 1 && k[0] == '0' {
		return false
	}
	n, err := strconv.ParseUint(k, 10, 32)
	return err == nil && n < math.MaxUint32
}

// parseJSObject parses a JSON object, keeping its members in order; a
// repeated key keeps its first position and its last value, as
// JSON.parse does.
func parseJSObject(s string) (jsObject, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	var o jsObject
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return o, false
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return o, false
		}
		k, ok := tok.(string)
		if !ok {
			return o, false
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return o, false
		}
		o.set(k, v)
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return o, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		// Trailing data: JSON.parse fails.
		return o, false
	}
	return o, true
}

// mergeHTMXObject merges the JSON objects of the attribute name (hx-vals
// or hx-headers) of e and its ancestors into into, as htmx's
// getValuesForElement does: a member is set only while into has no value
// or null for it, so nearer elements win. Script values (js:,
// javascript:), which htmx ignores without allowEval, and malformed JSON
// add nothing. It returns false at an "unset" value, where htmx stops.
func mergeHTMXObject(e ct.Node, name string, into *jsObject) bool {
	for p := e; p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		v, _ := hxValue(p, name)
		v = strings.TrimSpace(v)
		switch {
		case v == "":
			continue
		case v == "unset":
			return false
		case strings.HasPrefix(v, "javascript:"), strings.HasPrefix(v, "js:"):
			continue
		}
		if !strings.HasPrefix(v, "{") {
			v = "{" + v + "}"
		}
		obj, ok := parseJSObject(v)
		if !ok {
			continue
		}
		for _, k := range obj.keys {
			if into.isNull(k) {
				into.set(k, obj.vals[k])
			}
		}
	}
	return true
}

// htmxValsFields returns the fields of the merged hx-vals of e, as htmx
// converts them to form data: one field per item of an array, objects and
// null as JSON.stringify writes them, other values as strings. "unset" on
// any of the elements drops them all.
func htmxValsFields(e ct.Node) []Part {
	var obj jsObject
	if !mergeHTMXObject(e, "hx-vals", &obj) {
		return nil
	}
	var out []Part
	for _, k := range obj.ordered() {
		v := bytes.TrimSpace(obj.vals[k])
		switch {
		case len(v) > 0 && v[0] == '[':
			var items []json.RawMessage
			if json.Unmarshal(v, &items) != nil {
				continue
			}
			for _, item := range items {
				out = append(out, Part{Name: k, Value: jsString(item)})
			}
		case len(v) > 0 && v[0] == '{', isJSONNull(v):
			s, ok := jsonStringify(v)
			if !ok {
				continue
			}
			out = append(out, Part{Name: k, Value: s})
		default:
			out = append(out, Part{Name: k, Value: jsString(v)})
		}
	}
	return out
}

// jsonStringify returns what JavaScript's JSON.stringify writes for the
// value that JSON.parse reads from the JSON text v: object members in
// enumeration order (see [jsObject]), a repeated name once with its last
// value; numbers as JavaScript writes them, non-finite ones as null;
// strings with only quotes, backslashes, control characters and lone
// surrogates escaped. Member names are decoded as Go decodes them, so a
// lone surrogate in a name becomes U+FFFD. ok is false for malformed
// JSON.
func jsonStringify(v json.RawMessage) (string, bool) {
	var b strings.Builder
	if !writeJSON(&b, v) {
		return "", false
	}
	return b.String(), true
}

func writeJSON(b *strings.Builder, v json.RawMessage) bool {
	v = bytes.TrimSpace(v)
	if len(v) == 0 {
		return false
	}
	switch v[0] {
	case '{':
		obj, ok := parseJSObject(string(v))
		if !ok {
			return false
		}
		b.WriteByte('{')
		for i, k := range obj.ordered() {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSString(b, utf16.Encode([]rune(k)))
			b.WriteByte(':')
			if !writeJSON(b, obj.vals[k]) {
				return false
			}
		}
		b.WriteByte('}')
	case '[':
		var items []json.RawMessage
		if json.Unmarshal(v, &items) != nil {
			return false
		}
		b.WriteByte('[')
		for i, item := range items {
			if i > 0 {
				b.WriteByte(',')
			}
			if !writeJSON(b, item) {
				return false
			}
		}
		b.WriteByte(']')
	case '"':
		units, ok := jsonStringUnits(v)
		if !ok {
			return false
		}
		writeJSString(b, units)
	case 't', 'f', 'n':
		switch s := string(v); s {
		case "true", "false", "null":
			b.WriteString(s)
		default:
			return false
		}
	default:
		if !json.Valid(v) {
			return false
		}
		switch s := jsNumber(string(v)); s {
		case "Infinity", "-Infinity":
			// JSON.parse reads an overflowing number as an infinity, which
			// JSON.stringify writes as null.
			b.WriteString("null")
		default:
			b.WriteString(s)
		}
	}
	return true
}

// jsonStringUnits decodes the JSON string token v into UTF-16 code units,
// as JSON.parse does, keeping lone surrogates of \u escapes.
func jsonStringUnits(v []byte) ([]uint16, bool) {
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return nil, false
	}
	s := string(v[1 : len(v)-1])
	var out []uint16
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"' || c < 0x20:
			return nil, false
		case c != '\\':
			r, size := utf8.DecodeRuneInString(s[i:])
			out = utf16.AppendRune(out, r)
			i += size
			continue
		}
		if i+1 >= len(s) {
			return nil, false
		}
		switch e := s[i+1]; e {
		case '"', '\\', '/':
			out = append(out, uint16(e))
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			if i+6 > len(s) {
				return nil, false
			}
			u, err := strconv.ParseUint(s[i+2:i+6], 16, 16)
			if err != nil {
				return nil, false
			}
			out = append(out, uint16(u))
			i += 6
			continue
		default:
			return nil, false
		}
		i += 2
	}
	return out, true
}

// writeJSString writes the UTF-16 string s quoted as JSON.stringify
// quotes it.
func writeJSString(b *strings.Builder, s []uint16) {
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		u := s[i]
		switch {
		case u == '"':
			b.WriteString(`\"`)
		case u == '\\':
			b.WriteString(`\\`)
		case u == '\b':
			b.WriteString(`\b`)
		case u == '\f':
			b.WriteString(`\f`)
		case u == '\n':
			b.WriteString(`\n`)
		case u == '\r':
			b.WriteString(`\r`)
		case u == '\t':
			b.WriteString(`\t`)
		case u < 0x20:
			fmt.Fprintf(b, `\u%04x`, u)
		case utf16.IsSurrogate(rune(u)):
			if u < 0xDC00 && i+1 < len(s) && 0xDC00 <= s[i+1] && s[i+1] <= 0xDFFF {
				b.WriteRune(utf16.DecodeRune(rune(u), rune(s[i+1])))
				i++
				continue
			}
			fmt.Fprintf(b, `\u%04x`, u)
		default:
			b.WriteRune(rune(u))
		}
	}
	b.WriteByte('"')
}

// jsString converts a JSON value to a string as JavaScript's String does.
func jsString(v json.RawMessage) string {
	v = bytes.TrimSpace(v)
	if len(v) == 0 {
		return ""
	}
	switch v[0] {
	case '"':
		var s string
		_ = json.Unmarshal(v, &s)
		return s
	case '{':
		return "[object Object]"
	case '[':
		var items []json.RawMessage
		if json.Unmarshal(v, &items) != nil {
			return ""
		}
		parts := make([]string, len(items))
		for i, item := range items {
			if !isJSONNull(item) {
				parts[i] = jsString(item)
			}
		}
		return strings.Join(parts, ",")
	case 't', 'f', 'n':
		return string(v)
	}
	return jsNumber(string(v))
}

// jsNumber formats a JSON number as JavaScript does: fixed notation from
// 1e-6 up to 1e21, else an exponent without leading zeros.
func jsNumber(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	switch {
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case err != nil:
		return s
	case f == 0:
		return "0"
	}
	if a := math.Abs(f); a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	mant, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	sign := exp[:1]
	exp = strings.TrimLeft(exp[1:], "0")
	return mant + "e" + sign + exp
}

// forbiddenHeader reports whether browsers keep scripts from setting the
// request header name to value, a valid header value (the forbidden
// request-headers of the Fetch standard, and Chromium's
// Access-Control-Request-Private-Network): the method-override headers
// only when one of the comma-separated values is CONNECT, TRACE or TRACK.
func forbiddenHeader(name, value string) bool {
	l := strings.ToLower(name)
	switch l {
	case "accept-charset", "accept-encoding", "access-control-request-headers",
		"access-control-request-method", "access-control-request-private-network",
		"connection", "content-length", "cookie", "cookie2", "date", "dnt", "expect",
		"host", "keep-alive", "origin", "referer", "set-cookie", "te", "trailer",
		"transfer-encoding", "upgrade", "via":
		return true
	case "x-http-method", "x-http-method-override", "x-method-override":
		for _, m := range splitHeaderValues(value) {
			if asciiEqualFold(m, "CONNECT") || asciiEqualFold(m, "TRACE") || asciiEqualFold(m, "TRACK") {
				return true
			}
		}
		return false
	}
	return strings.HasPrefix(l, "proxy-") || strings.HasPrefix(l, "sec-")
}

// splitHeaderValues splits a header value as Fetch's "get, decode, and
// split" does: at the commas outside quoted strings, which stay as
// written, trimming spaces and tabs.
func splitHeaderValues(v string) []string {
	var out []string
	start := 0
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '"':
			for i++; i < len(v) && v[i] != '"'; i++ {
				if v[i] == '\\' {
					i++
				}
			}
		case ',':
			out = append(out, strings.Trim(v[start:i], " \t"))
			start = i + 1
		}
	}
	return append(out, strings.Trim(v[start:], " \t"))
}

// asciiEqualFold reports whether a and b are equal under ASCII case
// folding (header values are bytes, not UTF-8).
func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// htmxHeaders returns the headers of the merged hx-headers of e, leaving
// out the ones that htmx sets itself for r and the ones that scripts may
// not set, as the browser sends them when htmx sets them with
// setRequestHeader: null values are skipped; a name that is not a valid
// header name is an error, since the browser throws and htmx sends no
// request; a value is trimmed and sent as Latin-1 bytes, and one that the
// browser rejects (characters beyond Latin-1, or NUL, CR or LF inside) is
// sent percent-encoded, with the name's -URI-AutoEncoded header, as htmx
// sends it then.
func htmxHeaders(e ct.Node, r HTMXRequest) (http.Header, error) {
	var obj jsObject
	set := func(k string) { obj.set(k, json.RawMessage(`"set"`)) }
	set(hx2.HeaderRequest)
	set(hx2.HeaderCurrentURL)
	if hasAttr(e, "id") {
		set(hx2.HeaderTrigger)
	}
	if hasAttr(e, "name") {
		set(hx2.HeaderTriggerName)
	}
	if r.Target != "" {
		set(hx2.HeaderTarget)
	}
	builtin := len(obj.keys)
	mergeHTMXObject(e, "hx-headers", &obj)
	if len(obj.keys) == builtin {
		return nil, nil
	}
	h := make(http.Header)
	// add is the browser's setRequestHeader for a valid name and value: it
	// drops a forbidden header silently.
	add := func(name, value string) {
		if !forbiddenHeader(name, value) {
			h.Add(name, value)
		}
	}
	for _, k := range obj.keys[builtin:] {
		v := obj.vals[k]
		if isJSONNull(v) {
			// htmx sets no header for null.
			continue
		}
		if !validHeaderName(k) {
			return nil, fmt.Errorf("the hx-headers name %q is not a valid header name, so htmx sends no request", k)
		}
		if k == "Content-Type" && r.htmxVerb() != "get" && !r.Multipart {
			// htmx sets it after merging hx-headers.
			continue
		}
		s := jsString(v)
		if b, ok := headerBytes(s); ok {
			add(k, b)
		} else {
			add(k, encodeURIComponent(s))
			add(k+"-URI-AutoEncoded", "true")
		}
	}
	if len(h) == 0 {
		return nil, nil
	}
	return h, nil
}

func validHeaderName(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0) {
			return false
		}
	}
	return true
}

// headerBytes returns the bytes that a browser sends for the header value
// s that a script sets: s without leading and trailing HTTP white space
// (space, tab, CR, LF), each character as one byte (Latin-1). ok is false
// when the browser rejects the value: a character beyond U+00FF, or a NUL,
// CR or LF inside.
func headerBytes(s string) (string, bool) {
	s = strings.Trim(s, " \t\r\n")
	b := make([]byte, 0, len(s))
	for _, c := range s {
		if c > 0xFF || c == 0 || c == '\r' || c == '\n' {
			return "", false
		}
		b = append(b, byte(c))
	}
	return string(b), true
}

// encodeURIComponent percent-encodes s as JavaScript's
// encodeURIComponent does.
func encodeURIComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// htmxScope resolves htmx's extended selectors in a subject.
type htmxScope struct {
	root  ct.Node
	frag  bool
	elems []ct.Node // the elements of the document, not in template content
	pos   map[ct.Node]int
	ids   map[string]ct.Node
}

func newHTMXScope(root ct.Node) *htmxScope {
	sc := &htmxScope{root: root, frag: isFragment(root), pos: make(map[ct.Node]int), ids: documentIDs(root)}
	for e := range ct.Elements(root) {
		if inside(e, "template") == nil {
			sc.pos[e] = len(sc.elems)
			sc.elems = append(sc.elems, e)
		}
	}
	return sc
}

// inheritKeyword matches the inherit keyword of an hx-include list.
var inheritKeyword = regexp.MustCompile(`(^|,)\s*inherit\s*($|,)`)

// target resolves the target of the htmx request of e: its id, or the
// hx-target selector that the subject cannot resolve.
func (sc *htmxScope) target(e ct.Node) (id, unresolved string, err error) {
	v, from, ok := closestHX(e, "hx-target")
	t := e
	switch {
	case !ok:
	case v == "this":
		t = from
	default:
		if p := unwrapSelector(v); sc.frag && simpleIDSelector(p) && sc.ids[p[1:]] == nil {
			// An element of the page around the fragment.
			return p[1:], "", nil
		}
		nodes, known := sc.selectAll(e, v)
		switch {
		case !known:
			return "", v, nil
		case len(nodes) == 0:
			return "", "", fmt.Errorf("the hx-target %q selects no element, so htmx sends no request", v)
		}
		t = nodes[0]
	}
	id, _ = attrOf(t, "id")
	return id, "", nil
}

// includes resolves the elements of the nearest hx-include of e.
func (sc *htmxScope) includes(e ct.Node) ([]ct.Node, error) {
	v, from, ok := closestHX(e, "hx-include")
	switch {
	case !ok:
		return nil, nil
	case v == "this":
		return []ct.Node{from}, nil
	case inheritKeyword.MatchString(v):
		return nil, fmt.Errorf("the hx-include %q inherits, which is not supported", v)
	}
	nodes, known := sc.selectAll(e, v)
	if !known {
		return nil, fmt.Errorf("cannot resolve the hx-include %q within the subject", v)
	}
	return nodes, nil
}

// selectAll resolves an extended selector from e as htmx's
// querySelectorAllExt does: the elements of the keyword parts in order,
// then the elements that the other parts match, in tree order. known is
// false when the subject cannot tell: a selector that is not simple, a
// non-element (document, window), or, in a fragment, an element that may
// be outside it.
func (sc *htmxScope) selectAll(e ct.Node, v string) (nodes []ct.Node, known bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "global ")
	var std []simpleSelector
	var stdIDs []string
	add := func(n ct.Node) bool {
		if n == nil {
			// Not found: in a fragment, it may be in the page.
			return !sc.frag
		}
		nodes = append(nodes, n)
		return true
	}
	for _, part := range splitSelectors(v) {
		p := unwrapSelector(part)
		if kw, rest, ok := keywordSelector(p); ok {
			s, ok := parseSimple(unwrapSelector(rest))
			if !ok {
				return nil, false
			}
			switch kw {
			case "closest":
				if !add(ct.Closest(e, s.match)) {
					return nil, false
				}
			case "find":
				for _, d := range sc.elems[sc.pos[e]+1:] {
					if !isAncestor(e, d) {
						break
					}
					if s.match(d) {
						nodes = append(nodes, d)
						break
					}
				}
			case "next":
				var found ct.Node
				for _, d := range sc.elems[sc.pos[e]+1:] {
					if s.match(d) && !isAncestor(e, d) {
						found = d
						break
					}
				}
				if !add(found) {
					return nil, false
				}
			case "previous":
				var found ct.Node
				for i := sc.pos[e] - 1; i >= 0; i-- {
					if d := sc.elems[i]; s.match(d) && !isAncestor(d, e) {
						found = d
						break
					}
				}
				if !add(found) {
					return nil, false
				}
			}
			continue
		}
		switch p {
		case "next", "nextElementSibling":
			if !add(sibling(e, 1)) {
				return nil, false
			}
		case "previous", "previousElementSibling":
			if !add(sibling(e, -1)) {
				return nil, false
			}
		case "body":
			if sc.frag {
				return nil, false
			}
			for _, d := range sc.elems {
				if d.Tag() == "body" {
					nodes = append(nodes, d)
					break
				}
			}
		case "document", "window", "root", "host":
			return nil, false
		default:
			s, ok := parseSimple(p)
			if !ok {
				return nil, false
			}
			if sc.frag {
				// Only an id is unique: other matches may precede the
				// fragment in the page.
				if !simpleIDSelector(p) {
					return nil, false
				}
				stdIDs = append(stdIDs, p[1:])
			}
			std = append(std, s)
		}
	}
	for _, id := range stdIDs {
		if sc.ids[id] == nil {
			return nil, false
		}
	}
	if len(std) > 0 {
		for _, d := range sc.elems {
			for _, s := range std {
				if s.match(d) {
					nodes = append(nodes, d)
					break
				}
			}
		}
	}
	return nodes, true
}

// keywordSelector splits "closest S", "find S", "next S" and "previous S".
func keywordSelector(p string) (kw, rest string, ok bool) {
	for _, kw := range []string{"closest", "find", "next", "previous"} {
		if rest, ok := cutKeyword(p, kw); ok {
			return kw, rest, true
		}
	}
	return "", "", false
}

// sibling returns the next (dir 1) or previous (dir -1) element sibling of
// e, or nil.
func sibling(e ct.Node, dir int) ct.Node {
	p := e.Parent()
	if p == nil {
		return nil
	}
	var kids []ct.Node
	for c := range p.Children() {
		if c.Kind() == ct.KindElement {
			kids = append(kids, c)
		}
	}
	i := slices.Index(kids, e) + dir
	if i < 0 || i >= len(kids) {
		return nil
	}
	return kids[i]
}

// build returns the method, URL, body and the options of the htmx request,
// given the base URL that the request goes to and the settings of the
// caller's options.
func (r HTMXRequest) build(base *url.URL, rs reqSettings) (method, target string, body Body, pre []ReqOption, err error) {
	method = strings.ToUpper(r.Method)
	if !slices.Contains(htmxVerbs, strings.ToLower(method)) {
		return "", "", Body{}, nil, fmt.Errorf("%w: htmx sends no %q requests", ErrFormData, r.Method)
	}
	if r.unresolved != "" && rs.target == nil {
		return "", "", Body{}, nil, fmt.Errorf("%w: the hx-target %q cannot be resolved within the subject; pass HXTarget with the id of the target", ErrFormData, r.unresolved)
	}
	path := trimURL(r.Path)
	u, err := url.Parse(path)
	if err != nil {
		return "", "", Body{}, nil, fmt.Errorf("%w: invalid request URL: %w", ErrFormData, err)
	}
	if u.Scheme == "" && u.Host == "" && !strings.HasPrefix(u.Path, "/") {
		switch {
		case rs.page != nil:
			if path == "" {
				// htmx requests location.href, query included.
				path = *rs.page
			}
		case path == "":
			return "", "", Body{}, nil, fmt.Errorf("%w: the request URL is empty, which is the page's URL; pass PageURL with it", ErrFormData)
		default:
			return "", "", Body{}, nil, fmt.Errorf("%w: the request URL is relative to the page's URL; pass PageURL with it", ErrFormData)
		}
	}
	if page, ok := sameOrigin(base, rs.page, path); !ok {
		// Also when the configuration allows cross-origin requests, the
		// request would go to another server than the handler.
		return "", "", Body{}, nil, fmt.Errorf("%w: the request URL %q is not of the page's origin %s: htmx refuses it (selfRequestsOnly), and otherwise sends it to another server", ErrFormData, r.Path, page)
	}
	switch {
	case r.URLParams:
		if len(r.Fields) > 0 {
			base, _, _ := strings.Cut(path, "#")
			sep := "?"
			if strings.Contains(base, "?") {
				sep = "&"
			}
			path = base + sep + string(Body{kind: bodyForm, parts: r.Fields}.Bytes())
		}
	case method == http.MethodGet:
		// A browser sends no body with GET.
	case r.Multipart:
		parts := make([]Part, len(r.Fields))
		for i, p := range r.Fields {
			p.Name = normalizeNewlines(p.Name)
			if !p.File {
				p.Value = normalizeNewlines(p.Value)
			}
			parts[i] = p
		}
		body = Body{kind: bodyMultipart, parts: parts}
	default:
		body = Body{kind: bodyForm, parts: append([]Part(nil), r.Fields...)}
	}
	pre = []ReqOption{HTMX()}
	if r.Target != "" {
		pre = append(pre, HXTarget(r.Target))
	}
	if body.kind == bodyNone && r.htmxVerb() != "get" && !r.Multipart {
		// htmx sets it for every verb but "get", also without a body, as
		// for a formmethod="GET" whose body a GET request drops.
		pre = append(pre, Header("Content-Type", "application/x-www-form-urlencoded"))
	}
	if r.Trigger != "" {
		pre = append(pre, Header(hx2.HeaderTrigger, r.Trigger))
	}
	if r.TriggerName != "" {
		pre = append(pre, Header(hx2.HeaderTriggerName, r.TriggerName))
	}
	names := make([]string, 0, len(r.Header))
	for name := range r.Header {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		for _, v := range r.Header[name] {
			pre = append(pre, addHeader(name, v))
		}
	}
	return method, path, body, pre, nil
}

// Request builds the request as [Request] does, as htmx sends it: an htmx
// request ([HTMX]) with HX-Target, HX-Trigger, HX-Trigger-Name and the
// headers of r, then opts, which may replace them ([HXTarget], [Header]).
// The parameters go in the query of the URL (after a query that it has)
// or in the body (see [HTMXRequest.URLParams]). An empty or relative Path
// is resolved against the [PageURL] among opts, which is also sent as
// HX-Current-URL. An empty or relative Path without PageURL, a target
// that [HTMXData] could not resolve without [HXTarget], or a Path of
// another origin than the page (PageURL, else [BaseURL]; scheme, host and
// port) fails the test: htmx refuses such a URL by default
// (selfRequestsOnly), and where a configuration allows it, the request
// goes to another server than the one under test.
func (r HTMXRequest) Request(t TB, opts ...ReqOption) *http.Request {
	t.Helper()
	method, target, body, pre, err := r.build(defaultBase, settings(opts))
	if err != nil {
		fatalf(t, "%v", err)
		return nil
	}
	return Request(t, method, target, body, append(pre, opts...)...)
}

// sameOrigin reports whether the request URL path, resolved against the
// page's URL (page resolved against base, else base), has the page's
// origin, which it returns. A URL that does not parse is left to
// buildRequest, which reports it.
func sameOrigin(base *url.URL, page *string, path string) (string, bool) {
	from := base
	if page != nil {
		p, err := url.Parse(*page)
		if err != nil {
			return "", true
		}
		from = base.ResolveReference(p)
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", true
	}
	o := urlOrigin(from)
	return o, o == urlOrigin(from.ResolveReference(ref))
}

// urlOrigin returns the origin of u as browsers compare it: the scheme,
// the lower-cased host and the port, without the default port of http
// and https.
func urlOrigin(u *url.URL) string {
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	switch {
	case port == "", u.Scheme == "https" && port == "443", u.Scheme == "http" && port == "80":
		return u.Scheme + "://" + host
	}
	return u.Scheme + "://" + host + ":" + port
}

// htmxSubmission computes the htmx request of elt and the parts of its
// request, which goes to base; opts are the options of the request. A
// failure fails the test.
func htmxSubmission(t TB, base *url.URL, page *ct.Subject, elt, submitter *ct.Selection, edits url.Values, opts []ReqOption) (method, target string, body Body, pre []ReqOption, ok bool) {
	t.Helper()
	rs := settings(opts)
	r, err := htmxData(t.Context(), page, elt, submitter, edits, rs.urlMethods)
	if err == nil {
		method, target, body, pre, err = r.build(base, rs)
	}
	if err != nil {
		fatalf(t, "%v", prefixed(err))
		return "", "", Body{}, nil, false
	}
	return method, target, body, pre, true
}

// SubmitHTMX sends the request that htmx sends when elt, an element of
// page with an htmx request attribute, triggers it ([HTMXData]): for
// example a form with hx-post when it is submitted (by submitter, or nil),
// or a button with hx-delete when it is clicked. edits replace the values
// of the named controls as a user would enter them. The client's Options
// and opts apply after htmx's headers, so [HXTarget] and [Header] can
// replace them; [PageURL] resolves an empty or relative request URL and
// is sent as HX-Current-URL, and [PageHTMXConfig] names the configuration
// of a fragment's page.
//
// As with [Client.Submit], the request carries what htmx sends:
// [Client.CSRF] is not added. A request that htmx would not send (see
// [HTMXData] and [HTMXRequest.Request]), such as one of a submitter that
// sends its own request instead, or one to another origin, fails the
// test.
func (c *Client) SubmitHTMX(t TB, page *ct.Subject, elt, submitter *ct.Selection, edits url.Values, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	base, ok := c.base(t)
	if !ok {
		return nil
	}
	all := c.options(opts)
	method, target, body, pre, ok := htmxSubmission(t, base, page, elt, submitter, edits, all)
	if !ok {
		return nil
	}
	req := c.build(t, method, target, body, append(pre, all...))
	if req == nil {
		return nil
	}
	return c.Do(t, req)
}

// SubmitHTMX sends the request that htmx sends when elt triggers it, as
// [Client.SubmitHTMX] does, with the browser's cookies.
func (b *Browser) SubmitHTMX(t TB, page *ct.Subject, elt, submitter *ct.Selection, edits url.Values, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	all := append(append([]ReqOption(nil), b.Options...), opts...)
	method, target, body, pre, ok := htmxSubmission(t, b.base, page, elt, submitter, edits, all)
	if !ok {
		return nil
	}
	req, err := buildRequest(t.Context(), b.base, method, target, body, append(pre, all...), false)
	if err != nil {
		fatalf(t, "webuitest: %v", err)
		return nil
	}
	return b.Do(t, req)
}
