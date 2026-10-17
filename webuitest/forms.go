package webuitest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/protolambda/chord/ct"
)

// ErrFormData reports a form or submitter that [FormData] cannot submit.
var ErrFormData = errors.New("webuitest: cannot compute form data")

// Encoding types of form submissions.
const (
	EnctypeURLEncoded = "application/x-www-form-urlencoded"
	EnctypeMultipart  = "multipart/form-data"
	EnctypeText       = "text/plain"
)

// Submission is what a browser submits for a form: [FormData] computes it
// from a page, and [Submission.Request] (or [Client.Submit] and
// [Browser.Submit]) sends it.
type Submission struct {
	// Method is "GET", "POST" or "DIALOG" (a form of method dialog closes
	// its dialog and sends nothing).
	Method string
	// Action is the action URL as the page writes it: the submitter's
	// formaction, else the form's action. A browser resolves it against
	// the page's URL, which the page subject does not know: "" means the
	// page's URL itself, and a reference without a leading slash ("done",
	// "./x", "?page=2") is relative to it. Sending such an action needs
	// the page's URL ([PageURL]); otherwise, set an absolute path or URL.
	Action string
	// Enctype is [EnctypeURLEncoded], [EnctypeMultipart] or [EnctypeText].
	Enctype string
	// Fields are the entries of the form data set, in tree order: text
	// fields, and file fields (File set) for file inputs, which browsers
	// send with an empty file name when no file is selected. Line breaks
	// in names and text values are CRLF, as browsers send them.
	Fields []Part

	// names are the names of the form's controls, successful or not.
	names map[string]bool
}

// Values returns the text fields of the submission.
func (s Submission) Values() url.Values {
	out := make(url.Values)
	for _, p := range s.Fields {
		if !p.File {
			out[p.Name] = append(out[p.Name], p.Value)
		}
	}
	return out
}

// HasControl reports whether the form has a control called name, whether
// or not it is part of the submission (such as an unchecked checkbox).
func (s Submission) HasControl(name string) bool { return s.names[name] }

// Set replaces the fields called name with text fields of values, at the
// position of the first such field, or at the end when there is none, as
// if a user had entered them. Without values, it removes the fields.
func (s *Submission) Set(name string, values ...string) {
	parts := make([]Part, len(values))
	for i, v := range values {
		parts[i] = Part{Name: name, Value: normalizeNewlines(v)}
	}
	s.replace(name, parts)
}

// SetFile replaces the fields called name with one file field, as if a
// user had chosen a file. An empty contentType means
// application/octet-stream.
func (s *Submission) SetFile(name, filename, contentType string, content []byte) {
	s.replace(name, []Part{{Name: name, Value: string(content), File: true, Filename: filename, ContentType: contentType}})
}

func (s *Submission) replace(name string, parts []Part) {
	out := make([]Part, 0, len(s.Fields)+len(parts))
	done := false
	for _, p := range s.Fields {
		if p.Name != name {
			out = append(out, p)
			continue
		}
		if !done {
			out = append(out, parts...)
			done = true
		}
	}
	if !done {
		out = append(out, parts...)
	}
	s.Fields = out
}

// Body returns the request body of a POST submission in its encoding.
func (s Submission) Body() Body {
	parts := append([]Part(nil), s.Fields...)
	switch s.Enctype {
	case EnctypeMultipart:
		return Body{kind: bodyMultipart, parts: parts}
	case EnctypeText:
		return Body{kind: bodyText, parts: parts}
	}
	return Body{kind: bodyForm, parts: parts}
}

// target returns the method, URL and body of the submission's request: a
// GET submission replaces the query of the action with the fields. An
// action relative to the page's URL needs [PageURL] among opts, against
// which the request target is resolved.
func (s Submission) target(opts []ReqOption) (method, target string, body Body, err error) {
	switch s.Method {
	case "DIALOG":
		return "", "", Body{}, fmt.Errorf("%w: a dialog form sends no request", ErrFormData)
	case "GET", "POST":
	default:
		return "", "", Body{}, fmt.Errorf("%w: unknown method %q", ErrFormData, s.Method)
	}
	// Browsers strip leading and trailing ASCII white space from URLs.
	action := strings.Trim(s.Action, "\t\n\f\r ")
	u, err := url.Parse(action)
	if err != nil {
		return "", "", Body{}, fmt.Errorf("%w: invalid action: %w", ErrFormData, err)
	}
	if u.Scheme == "" && u.Host == "" && !strings.HasPrefix(u.Path, "/") {
		var rs reqSettings
		for _, o := range opts {
			if o != nil {
				o(&rs)
			}
		}
		switch {
		case rs.page != nil:
		case action == "":
			return "", "", Body{}, fmt.Errorf("%w: the form has no action; set Submission.Action or pass PageURL with the page's URL", ErrFormData)
		default:
			return "", "", Body{}, fmt.Errorf("%w: the action is relative to the page's URL; pass PageURL with it", ErrFormData)
		}
	}
	if s.Method == "POST" {
		return http.MethodPost, action, s.Body(), nil
	}
	u.RawQuery = string(Body{kind: bodyForm, parts: s.Fields}.Bytes())
	u.Fragment, u.RawFragment = "", ""
	return http.MethodGet, u.String(), Body{}, nil
}

// Request builds the request of the submission, as [Request] does: a GET
// with the fields as the query of the action, or a POST with the body.
// An action relative to the page (see [Submission.Action]) is resolved
// against the [PageURL] among opts. A dialog submission, or a relative
// action without PageURL, fails the test.
func (s Submission) Request(t TB, opts ...ReqOption) *http.Request {
	t.Helper()
	method, target, body, err := s.target(opts)
	if err != nil {
		fatalf(t, "%v", err)
		return nil
	}
	return Request(t, method, target, body, opts...)
}

// FormData computes what a browser submits when submitter submits form,
// both selections of page, which must each match one element: the form,
// and a submit button of the form (a button of type submit, or an input of
// type submit or image, associated with it); a nil submitter submits the
// form without one, as form.requestSubmit() does.
//
// It follows the HTML algorithm that constructs the entry list: the
// form's controls (its descendants, and elements elsewhere whose form
// attribute names it) in tree order; disabled controls (also in a
// disabled fieldset) and nameless ones are skipped; buttons count only as
// the submitter (an image button sends name.x and name.y as 0);
// checkboxes and radios only when checked, with "on" as the default
// value, and of a radio group (radios of the form with one name) only the
// last with the checked attribute, since checking a radio unchecks the
// others (also when that one is disabled, and so sends nothing); selects
// send their selected, enabled options, a single select its first enabled
// option when none is selected; textareas their raw text, as a parser
// reads it from the rendered page; file inputs a file field without a
// file; range and color inputs their default values; _charset_ hidden
// inputs "UTF-8"; dirname adds "ltr". Controls in the content of a
// template element belong to no form outside it. The submitter's
// formaction, formmethod and formenctype override the form's.
//
// Errors wrap [ErrFormData], or the load or query error of the
// selections.
func FormData(ctx context.Context, page *ct.Subject, form, submitter *ct.Selection) (Submission, error) {
	doc, err := page.Load(ctx)
	if err != nil {
		return Submission{}, err
	}
	root := doc.Root()
	f, err := one(ctx, form, "form")
	if err != nil {
		return Submission{}, err
	}
	if f.Tag() != "form" {
		return Submission{}, fmt.Errorf("%w: %s is a %s, not a form", ErrFormData, form, f.Tag())
	}
	if rootOf(f) != root {
		return Submission{}, fmt.Errorf("%w: %s is not part of the page", ErrFormData, form)
	}
	idx := indexForms(root)
	var sub ct.Node
	if submitter != nil {
		sub, err = one(ctx, submitter, "submitter")
		if err != nil {
			return Submission{}, err
		}
		if !submitButton(sub) {
			return Submission{}, fmt.Errorf("%w: %s is not a submit button", ErrFormData, submitter)
		}
		if idx.owner[sub] != f {
			return Submission{}, fmt.Errorf("%w: %s does not belong to the form", ErrFormData, submitter)
		}
		if disabledControl(sub) {
			return Submission{}, fmt.Errorf("%w: %s is disabled", ErrFormData, submitter)
		}
	}
	override := func(formAttr, buttonAttr string) string {
		if sub != nil {
			if v, ok := attrOf(sub, buttonAttr); ok {
				return v
			}
		}
		v, _ := attrOf(f, formAttr)
		return v
	}
	s := Submission{
		Method:  strings.ToUpper(normMethod(override("method", "formmethod"))),
		Action:  override("action", "formaction"),
		Enctype: normEnctype(override("enctype", "formenctype")),
		names:   make(map[string]bool),
	}
	radios := checkedRadios(idx.controls[f])
	for _, c := range idx.controls[f] {
		if name, ok := attrOf(c, "name"); ok && name != "" {
			s.names[name] = true
		}
		s.Fields = append(s.Fields, entries(c, sub, radios)...)
	}
	for i := range s.Fields {
		s.Fields[i].Name = normalizeNewlines(s.Fields[i].Name)
		if !s.Fields[i].File {
			s.Fields[i].Value = normalizeNewlines(s.Fields[i].Value)
		}
	}
	return s, nil
}

// one returns the single node of a selection.
func one(ctx context.Context, sel *ct.Selection, what string) (ct.Node, error) {
	if sel == nil {
		return nil, fmt.Errorf("%w: no %s selection", ErrFormData, what)
	}
	nodes, err := sel.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) != 1 {
		return nil, fmt.Errorf("%w: %s matches %d elements, want one %s", ErrFormData, sel, len(nodes), what)
	}
	return nodes[0], nil
}

func normEnctype(v string) string {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case EnctypeMultipart, EnctypeText:
		return v
	}
	return EnctypeURLEncoded
}

// normalizeNewlines turns every CR, LF and CRLF into CRLF.
func normalizeNewlines(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// checkedRadios returns the checked radio of each radio group of a form's
// controls, by name: checking a radio unchecks the others of its group
// (same form owner and name, compared case-sensitively), also while
// parsing, so the last radio with the checked attribute in tree order is
// the checked one, even when it is disabled and so submits nothing.
func checkedRadios(controls []ct.Node) map[string]ct.Node {
	out := make(map[string]ct.Node)
	for _, c := range controls {
		if c.Tag() != "input" || inputType(c) != "radio" || !hasAttr(c, "checked") {
			continue
		}
		if name, ok := attrOf(c, "name"); ok && name != "" {
			out[name] = c
		}
	}
	return out
}

// entries returns the entries that control c adds to the form data set;
// radios holds the checked radio of each group ([checkedRadios]).
func entries(c ct.Node, submitter ct.Node, radios map[string]ct.Node) []Part {
	if inside(c, "datalist") != nil || disabledControl(c) {
		return nil
	}
	name, _ := attrOf(c, "name")
	value, hasValue := attrOf(c, "value")
	switch c.Tag() {
	case "button":
		if c != submitter || name == "" {
			return nil
		}
		return []Part{{Name: name, Value: value}}
	case "input":
	case "select":
		if name == "" {
			return nil
		}
		return selectEntries(c, name)
	case "textarea":
		if name == "" {
			return nil
		}
		// The text is the value: the parser drops a line break right after
		// <textarea>, and chord's renderer writes an extra one before text
		// that starts with a line break, so that the parser keeps it.
		out := []Part{{Name: name, Value: rawText(c)}}
		if dir, ok := attrOf(c, "dirname"); ok && dir != "" {
			out = append(out, Part{Name: dir, Value: "ltr"})
		}
		return out
	default:
		return nil
	}
	switch t := inputType(c); t {
	case "submit", "button", "reset":
		if c != submitter || name == "" {
			return nil
		}
		return []Part{{Name: name, Value: value}}
	case "image":
		if c != submitter {
			return nil
		}
		x, y := "x", "y"
		if name != "" {
			x, y = name+".x", name+".y"
		}
		return []Part{{Name: x, Value: "0"}, {Name: y, Value: "0"}}
	case "checkbox", "radio":
		if name == "" || !hasAttr(c, "checked") || (t == "radio" && radios[name] != c) {
			return nil
		}
		if !hasValue {
			value = "on"
		}
		return []Part{{Name: name, Value: value}}
	case "file":
		if name == "" {
			return nil
		}
		return []Part{{Name: name, File: true, ContentType: "application/octet-stream"}}
	case "hidden":
		if name == "" {
			return nil
		}
		if strings.EqualFold(name, "_charset_") {
			value = "UTF-8"
		}
		return []Part{{Name: name, Value: value}}
	default:
		if name == "" {
			return nil
		}
		value = sanitizeValue(c, t, value)
		out := []Part{{Name: name, Value: value}}
		if dir, ok := attrOf(c, "dirname"); ok && dir != "" && (t == "text" || t == "search") {
			out = append(out, Part{Name: dir, Value: "ltr"})
		}
		return out
	}
}

// sanitizeValue applies the value sanitization of HTML input types that
// change a default value.
func sanitizeValue(c ct.Node, t, v string) string {
	switch t {
	case "text", "search", "tel", "password":
		return strings.NewReplacer("\r", "", "\n", "").Replace(v)
	case "url", "email":
		return strings.TrimSpace(strings.NewReplacer("\r", "", "\n", "").Replace(v))
	case "range":
		lo, hi := numberAttr(c, "min", 0), numberAttr(c, "max", 100)
		if hi < lo {
			hi = lo
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return strconv.FormatFloat(min(max(f, lo), hi), 'f', -1, 64)
		}
		return strconv.FormatFloat(lo+(hi-lo)/2, 'f', -1, 64)
	case "color":
		if len(v) == 7 && v[0] == '#' && strings.Trim(strings.ToLower(v[1:]), "0123456789abcdef") == "" {
			return strings.ToLower(v)
		}
		return "#000000"
	}
	return v
}

func numberAttr(c ct.Node, name string, def float64) float64 {
	v, ok := attrOf(c, name)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return def
	}
	return f
}

// selectEntries returns the entries of a select element.
func selectEntries(sel ct.Node, name string) []Part {
	var options []ct.Node
	for o := range Elements(sel, "option") {
		options = append(options, o)
	}
	disabled := func(o ct.Node) bool {
		if hasAttr(o, "disabled") {
			return true
		}
		p := o.Parent()
		return p != nil && p.Tag() == "optgroup" && hasAttr(p, "disabled")
	}
	multiple := hasAttr(sel, "multiple")
	size := 1
	if v, ok := attrOf(sel, "size"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			size = n
		}
	}
	var selected []ct.Node
	for _, o := range options {
		if hasAttr(o, "selected") {
			selected = append(selected, o)
		}
	}
	if !multiple {
		switch {
		case len(selected) > 1:
			// The last selected option wins.
			selected = selected[len(selected)-1:]
		case len(selected) == 0 && size == 1:
			for _, o := range options {
				if !disabled(o) {
					selected = []ct.Node{o}
					break
				}
			}
		}
	}
	var out []Part
	for _, o := range selected {
		if disabled(o) {
			continue
		}
		v, ok := attrOf(o, "value")
		if !ok {
			v = collapse(rawText(o))
		}
		out = append(out, Part{Name: name, Value: v})
	}
	return out
}
