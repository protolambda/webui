package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// FormControl creates a Bootstrap form-control class attribute, followed by
// opts. Use it on input and textarea elements.
func FormControl(opts ...attr.Node) attr.Node {
	return attr.Cons(rawClass("form-control"), opts...)
}

// FormControlSM creates a Bootstrap form-control-sm class: a small form
// control. Combine it with [FormControl].
func FormControlSM() attr.Node { return rawClass("form-control-sm") }

// FormControlLG creates a Bootstrap form-control-lg class: a large form
// control. Combine it with [FormControl].
func FormControlLG() attr.Node { return rawClass("form-control-lg") }

// FormSelect creates a Bootstrap form-select class attribute, followed by
// opts. Use it on select elements.
func FormSelect(opts ...attr.Node) attr.Node {
	return attr.Cons(rawClass("form-select"), opts...)
}

// FormSelectSM creates a Bootstrap form-select-sm class: a small select.
// Combine it with [FormSelect].
func FormSelectSM() attr.Node { return rawClass("form-select-sm") }

// FormSelectLG creates a Bootstrap form-select-lg class: a large select.
// Combine it with [FormSelect].
func FormSelectLG() attr.Node { return rawClass("form-select-lg") }

// FormCheckInput creates a Bootstrap form-check-input class attribute,
// followed by opts. Use it on checkbox and radio inputs.
func FormCheckInput(opts ...attr.Node) attr.Node {
	return attr.Cons(rawClass("form-check-input"), opts...)
}

// IsInvalid creates a Bootstrap is-invalid class, which marks a form control
// as failing server-side validation and shows the [InvalidFeedback] next to
// it. Also set aria-invalid="true" on the control, and point its
// aria-describedby at the feedback's id, so that assistive technology
// announces the problem.
func IsInvalid() attr.Node { return rawClass("is-invalid") }

// IsValid creates a Bootstrap is-valid class, which marks a form control as
// accepted and shows the [ValidFeedback] next to it.
func IsValid() attr.Node { return rawClass("is-valid") }

// InvalidFeedback creates a Bootstrap invalid-feedback class for the error
// message of a form control. The message is hidden unless a preceding
// sibling control is marked [IsInvalid] (or the form is [WasValidated] and
// the control fails browser validation).
func InvalidFeedback() attr.Node { return rawClass("invalid-feedback") }

// ValidFeedback creates a Bootstrap valid-feedback class for the
// confirmation message of a form control, shown next to an [IsValid]
// control.
func ValidFeedback() attr.Node { return rawClass("valid-feedback") }

// WasValidated creates a Bootstrap was-validated class for a form: its
// controls are styled by the browser's own constraint validation (:valid
// and :invalid) instead of IsValid and IsInvalid.
func WasValidated() attr.Node { return rawClass("was-validated") }
