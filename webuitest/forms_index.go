package webuitest

import (
	"strings"

	"github.com/protolambda/chord/ct"
)

// listedTags are the listed form-associated elements of HTML: the ones
// whose form owner is the nearest form ancestor or the form their form
// attribute names.
var listedTags = map[string]bool{
	"button": true, "fieldset": true, "input": true, "object": true,
	"output": true, "select": true, "textarea": true,
}

// formIndex holds the form owners of a document.
type formIndex struct {
	forms    []ct.Node
	controls map[ct.Node][]ct.Node // form -> listed elements it owns, in tree order
	owner    map[ct.Node]ct.Node   // listed element -> form owner
}

// indexForms computes the form owner of every listed element below root:
// the form element whose id the element's form attribute names (none when
// that id names no form), else the nearest form ancestor in its tree. The
// content of a template element is an inert tree of its own, which is
// never connected to the document: a form outside the template does not
// own it, and its form attributes and ids do not apply.
func indexForms(root ct.Node) formIndex {
	ids := documentIDs(root)
	idx := formIndex{
		controls: make(map[ct.Node][]ct.Node),
		owner:    make(map[ct.Node]ct.Node),
	}
	for e := range Elements(root, "") {
		if e.Tag() == "form" {
			idx.forms = append(idx.forms, e)
		}
		if !listedTags[e.Tag()] {
			continue
		}
		var owner ct.Node
		if ref, ok := attrOf(e, "form"); ok && inside(e, "template") == nil {
			if f := ids[ref]; f != nil && f.Tag() == "form" {
				owner = f
			}
		} else {
			owner = nearestForm(e)
		}
		if owner != nil {
			idx.owner[e] = owner
			idx.controls[owner] = append(idx.controls[owner], e)
		}
	}
	return idx
}

// nearestForm returns the nearest form ancestor of n in its tree, or nil:
// the search stops at a template element, whose content is a tree of its
// own.
func nearestForm(n ct.Node) ct.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement {
			continue
		}
		switch p.Tag() {
		case "form":
			return p
		case "template":
			return nil
		}
	}
	return nil
}

// sameTree reports whether a and b are in the same tree: the document, or
// the content of the same template element.
func sameTree(a, b ct.Node) bool {
	return inside(a, "template") == inside(b, "template")
}

// documentIDs maps the ids of the elements of the document below root, not
// in template content, to the first element with each, as
// document.getElementById finds them.
func documentIDs(root ct.Node) map[string]ct.Node {
	out := make(map[string]ct.Node)
	for e := range Elements(root, "") {
		if id, ok := attrOf(e, "id"); ok && id != "" && inside(e, "template") == nil {
			if _, dup := out[id]; !dup {
				out[id] = e
			}
		}
	}
	return out
}

// formMethod returns the method a form submits with: "post", "dialog" or
// "get" (also for a missing or invalid method).
func formMethod(form ct.Node) string {
	m, _ := attrOf(form, "method")
	return normMethod(m)
}

func normMethod(m string) string {
	switch m = strings.ToLower(strings.TrimSpace(m)); m {
	case "post", "dialog":
		return m
	}
	return "get"
}

// inputType returns the lower-cased type of an input element, "text" when
// it has none.
func inputType(e ct.Node) string {
	t, ok := attrOf(e, "type")
	t = strings.ToLower(strings.TrimSpace(t))
	if !ok || t == "" {
		return "text"
	}
	return t
}

// submitButton reports whether e is a submit button: a button of type
// submit (the default) or an input of type submit or image.
func submitButton(e ct.Node) bool {
	switch e.Tag() {
	case "button":
		t, ok := attrOf(e, "type")
		t = strings.ToLower(strings.TrimSpace(t))
		// An invalid type is the submit state too.
		return !ok || (t != "button" && t != "reset")
	case "input":
		t := inputType(e)
		return t == "submit" || t == "image"
	}
	return false
}

// disabledControl reports whether a form control is disabled: by its own
// disabled attribute, or by a disabled fieldset ancestor unless it is in
// that fieldset's first legend.
func disabledControl(e ct.Node) bool {
	if hasAttr(e, "disabled") {
		return true
	}
	for p := e.Parent(); p != nil; p = p.Parent() {
		if p.Kind() != ct.KindElement || p.Tag() != "fieldset" || !hasAttr(p, "disabled") {
			continue
		}
		if legend := firstLegend(p); legend != nil && isAncestor(legend, e) {
			continue
		}
		return true
	}
	return false
}

func firstLegend(fieldset ct.Node) ct.Node {
	for c := range fieldset.Children() {
		if c.Kind() == ct.KindElement && c.Tag() == "legend" {
			return c
		}
	}
	return nil
}

// isAncestor reports whether a is an ancestor of n.
func isAncestor(a, n ct.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p == a {
			return true
		}
	}
	return false
}

// csrfField reports whether e is a control that submits the CSRF field:
// an input named session.CSRFField that is not disabled and not in a
// datalist (whose controls are never submitted).
func csrfField(e ct.Node) bool {
	return e.Tag() == "input" && AttrIs(e, "name", csrfFieldName) && !disabledControl(e) && inside(e, "datalist") == nil
}
