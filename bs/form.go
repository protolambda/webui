package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"
)

// FormGroup creates a form field wrapper: div.mb-3, a block with a bottom
// margin (Bootstrap 5 has no .form-group class). [Field], [Textarea] and
// [Select] render this wrapper themselves, with another class when their
// WrapperClass is set.
func FormGroup(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("mb-3"), attrs...))
}

// FormLabel creates a Bootstrap form label (label.form-label). Give it the
// id of its control with label.For.
func FormLabel(attrs ...attr.Node) elem.Scope {
	return label.Label(attr.Cons(rawClass("form-label"), attrs...))
}

// FormCheck creates a Bootstrap form check wrapper (div.form-check), for a
// checkbox or radio input followed by its [FormCheckLabel]. See also
// [Checkbox] and [Radio].
func FormCheck(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("form-check"), attrs...))
}

// FormSwitch creates a Bootstrap switch wrapper (div.form-check.form-switch),
// for a checkbox with role="switch". See also [Checkbox] with Switch.
func FormSwitch(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("form-check form-switch"), attrs...))
}

// FormCheckLabel creates a Bootstrap form check label (label.form-check-label).
func FormCheckLabel(attrs ...attr.Node) elem.Scope {
	return label.Label(attr.Cons(rawClass("form-check-label"), attrs...))
}

// FormText creates a Bootstrap form help text (div.form-text). Give it an
// id and reference it from the control's aria-describedby, as [Field]
// does.
func FormText(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("form-text"), attrs...))
}

// InvalidFeedback creates a Bootstrap validation message
// (div.invalid-feedback). Bootstrap shows it only after a sibling control
// with the is-invalid class: render it after the control, give it an id,
// and reference it from the control's aria-describedby, as [Field] does.
func InvalidFeedback(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("invalid-feedback"), attrs...))
}

// InputGroup creates a Bootstrap input group (div.input-group), a control
// with text or buttons attached. See also [Field] with Before and After.
func InputGroup(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("input-group"), attrs...))
}

// InputGroupText creates a Bootstrap input group text addon
// (span.input-group-text).
func InputGroupText(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("input-group-text"), attrs...))
}
