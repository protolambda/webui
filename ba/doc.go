// Package ba provides Bootstrap 5.3 utility classes as chord attributes.
//
// Every helper returns an [attr.Node], usually a class attribute, to apply to
// any element: the html element packages of chord or the components of the
// bs package. Package bs creates elements (bs.Row() is a <div class="row">),
// while ba only adds attributes (ba.Row() is the class attribute, for an
// element of your choice).
//
// Several class attributes on one element merge into a single class
// attribute:
//
//	div.Div(ba.DFlex(), ba.AlignItemsCenter(), ba.Gap(2), ba.MB(3))(
//		text.Span(ba.FWSemibold())(text.Text("Tasks")),
//		text.Span(ba.MSAuto(), ba.TextMuted())(text.Text("3 open")),
//	)
//
// renders
//
//	<div class="d-flex align-items-center gap-2 mb-3"><span class="fw-semibold">Tasks</span><span class="ms-auto text-body-secondary">3 open</span></div>
//
// # Validated values
//
// Helpers that take a value accept only values that have a Bootstrap class:
// spacing ([M], [P], [MT], ... and [Gap]) 0..5, [FS] 1..6, column spans
// ([ColMD], [ColAt], ...) 1..12, [Offset] 1..11 and responsive offsets
// ([OffsetMD], [OffsetAt], ...) 0..11, [BorderWidth] and [RoundedSize] 0..5,
// the named weights of [FW] and line heights of [LH], and the breakpoints
// [SM] to [XXL]. Any other value never becomes a class that Bootstrap does
// not define: the helper returns an attribute that fails the render with an
// error wrapping [ErrInvalidValue], so the mistake shows up in tests and as
// a render error instead of as a silently unstyled page.
//
// Auto margins have helpers of their own: [MAuto], [MXAuto], [MSAuto], and
// so on.
//
// # Responsive variants
//
// A [Breakpoint] selects the screen width from which a responsive utility
// applies. For example, a sidebar that only shows from the md breakpoint up,
// and a toolbar that stacks on small screens:
//
//	div.Div(ba.DNone(), ba.DBlockAt(ba.MD))(sidebar)
//	div.Div(ba.DFlex(), ba.FlexColumn(), ba.FlexRowAt(ba.LG), ba.Gap(2))(tools...)
//
// # Accessibility and forms
//
// [VisuallyHidden] keeps text for assistive technology while hiding it on
// screen; [VisuallyHiddenFocusable] reveals it on keyboard focus (skip
// links). [IsInvalid] and [InvalidFeedback] show server-side validation
// errors; pair them with aria-invalid and aria-describedby on the control.
//
// # JavaScript-dependent helpers
//
// Every helper is a plain CSS class and works without JavaScript, except
// [Tooltip] and [Popover]: they need the opt-in Bootstrap JavaScript bundle
// and application code that initializes each element, which webui does not
// provide. Prefer the title attribute or the native HTML popover attribute.
//
// # Escaping
//
// Classes are written without escaping only when they are literals of this
// package or are built from validated digits and breakpoints. Values from
// callers ([FW], [LH]) are checked against a fixed list first. Combining ba
// classes with escaped classes from attr.Class is safe.
//
// Reference: https://getbootstrap.com/docs/5.3/utilities/
package ba
