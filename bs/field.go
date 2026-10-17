package bs

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	selectel "github.com/protolambda/chord/html/form/select"
	"github.com/protolambda/chord/html/form/textarea"
	"github.com/protolambda/chord/html/group/div"
)

// Field is a labelled form input with optional help text and a server-side
// validation error, wired for assistive technology:
//
//	<div class="mb-3">
//	  <label class="form-label" for="ID">Label</label>
//	  <input class="form-control is-invalid" type="email" id="ID" name="email" value="..."
//	    aria-invalid="true" aria-describedby="ID-error ID-help"/>
//	  <div class="invalid-feedback" id="ID-error">Error</div>
//	  <div class="form-text" id="ID-help">Help</div>
//	</div>
//
// With Error set, the input gets the is-invalid class (which also shows
// the feedback) and aria-invalid="true", and the error comes first in
// aria-describedby, so that screen readers read it right after the label.
// An aria-describedby among Attrs is appended to the field's own ids.
// With Before or After set, the input and the error sit in a
// div.input-group (with has-validation when there is an error).
//
// Value and Placeholder are escaped by chord. Never put a secret in Value:
// it is sent to the browser, and stays in the page source, history and
// caches. A password field must not have a value, in Value or in Attrs
// (Eval fails), and the same holds for any other secret: render the field
// empty and let the user type it.
type Field struct {
	// ID is the input's id: a token, see ValidID. The help and error ids
	// derive from it (ID-help, ID-error).
	ID string
	// Name is the form field name; empty renders no name, so that the
	// value is not submitted.
	Name string
	// Label is the content of the label, the input's accessible name.
	// Without it, Attrs must give an aria-label or aria-labelledby.
	Label elem.Node
	// LabelHidden makes the label visually hidden (but still the
	// accessible name), e.g. for a search box with a visible button.
	LabelHidden bool
	// Type is the input type; zero means text. Supported are the text-like
	// types (text, password, email, number, tel, url, search), the date and
	// time types, color, file and range. Checkboxes and radios are
	// [Checkbox] and [Radio]; other types fail with [ErrInvalidOption].
	Type input.InputType

	Value        string    // current value, escaped; never a secret
	Placeholder  string    // placeholder text, escaped; not a substitute for the label
	Autocomplete string    // autocomplete tokens, e.g. "email" or "current-password"; empty renders none
	InputMode    InputMode // virtual keyboard hint; zero renders none
	Required     bool
	Disabled     bool
	ReadOnly     bool

	Help  elem.Node // help text (div.form-text#ID-help); nil leaves it out
	Error elem.Node // validation error (div.invalid-feedback#ID-error); nil means valid

	Before []elem.Node // input group addons before the input, e.g. InputGroupText or buttons
	After  []elem.Node // input group addons after the input

	// Attrs are added to the input: the escape hatch for other attributes
	// (min, max, step, pattern, maxlength, autofocus, hx-* ...). They may
	// replace the aria-invalid default, but must not set an attribute that
	// a field above renders (such as id, name, type or value).
	Attrs attr.Node
	// Wrapper holds additional attributes of the wrapping div.mb-3, e.g. a
	// grid column class.
	Wrapper attr.Node
}

// fieldClass returns the control class of an input type, and false for a
// type that Field does not render.
func fieldClass(t input.InputType) (string, bool) {
	switch t {
	case input.Text, input.Password, input.Email, input.Number, input.Tel,
		input.URL, input.Search, input.Date, input.Time, input.DatetimeLocal,
		input.Month, input.Week, input.File:
		return "form-control", true
	case input.Color:
		return "form-control form-control-color", true
	case input.Range:
		return "form-range", true
	}
	return "", false
}

// Eval builds the field. It fails with [ErrInvalidID] for an invalid ID,
// with [ErrNoName] without a label, and with [ErrInvalidOption] for an
// unsupported Type or InputMode, or a password field with a value (in
// Value or in Attrs).
func (f Field) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Field"
	if err := checkID(component, f.ID); err != nil {
		return elem.Obj{}, err
	}
	typ := f.Type
	if typ == "" {
		typ = input.Text
	}
	class, ok := fieldClass(typ)
	if !ok {
		return elem.Obj{}, fmt.Errorf("%s: %w: unsupported input type", component, ErrInvalidOption)
	}
	if typ == input.Password && f.Value != "" {
		return elem.Obj{}, fmt.Errorf("%s: %w: a password field must not carry a value", component, ErrInvalidOption)
	}
	if !f.InputMode.Valid() {
		return elem.Obj{}, fmt.Errorf("%s: %w: unknown input mode", component, ErrInvalidOption)
	}
	c, err := newControl(ctx, component, f.ID, f.Label, f.Help, f.Error, f.Attrs)
	if err != nil {
		return elem.Obj{}, err
	}
	// Without a Value the field renders no value attribute of its own, so a
	// value in Attrs is not a duplicate: check it here.
	if typ == input.Password && sets(c.objs, "value") {
		return elem.Obj{}, fmt.Errorf("%s: %w: a password field must not carry a value", component, ErrInvalidOption)
	}

	own := []attr.Node{
		c.class(class),
		input.Type(typ),
		attr.ID(f.ID),
	}
	if f.Name != "" {
		own = append(own, input.Name(f.Name))
	}
	if f.Value != "" {
		own = append(own, input.Value(f.Value))
	}
	if f.Placeholder != "" {
		own = append(own, input.Placeholder(f.Placeholder))
	}
	if f.Autocomplete != "" {
		own = append(own, input.Autocomplete(f.Autocomplete))
	}
	if f.InputMode != "" {
		own = append(own, attr.Name("inputmode").Raw(string(f.InputMode)))
	}
	own = appendStates(own, f.Required, f.Disabled, f.ReadOnly)
	own = append(own, c.aria()...)
	control := input.Input(compose(c.user, own...))

	children := []elem.Node{c.label(f.LabelHidden, "form-label")}
	if len(f.Before) > 0 || len(f.After) > 0 {
		groupClass := "input-group"
		if f.Error != nil {
			groupClass += " has-validation"
		}
		group := make([]elem.Node, 0, len(f.Before)+len(f.After)+2)
		group = append(group, f.Before...)
		group = append(group, control)
		group = append(group, f.After...)
		group = append(group, c.feedback()...)
		children = append(children, div.Div(rawClass(groupClass))(group...))
	} else {
		children = append(children, control)
		children = append(children, c.feedback()...)
	}
	children = append(children, c.helpText()...)
	return wrapper(rawClass("mb-3"), f.Wrapper)(children...).Eval(ctx)
}

// Textarea is a labelled multi-line text field, with help text and a
// server-side validation error wired like those of a [Field]. Value is the
// content of the textarea, escaped by chord (which also keeps a leading
// newline); never put a secret in it.
type Textarea struct {
	ID           string    // the textarea's id: a token, see ValidID
	Name         string    // form field name; empty renders no name
	Label        elem.Node // label content; without it, Attrs must give an aria-label
	LabelHidden  bool      // visually hidden label
	Value        string    // current value, escaped; never a secret
	Rows         int       // visible text lines; zero renders no rows attribute
	Placeholder  string    // placeholder text, escaped
	Autocomplete string    // autocomplete tokens; empty renders none
	Required     bool
	Disabled     bool
	ReadOnly     bool
	Help         elem.Node // help text (div.form-text#ID-help); nil leaves it out
	Error        elem.Node // validation error (div.invalid-feedback#ID-error); nil means valid
	Attrs        attr.Node // additional attributes of the textarea; see Field.Attrs
	Wrapper      attr.Node // additional attributes of the wrapping div.mb-3
}

// Eval builds the textarea field. It fails with [ErrInvalidID] for an
// invalid ID, with [ErrNoName] without a label, and with
// [ErrInvalidOption] for negative Rows.
func (t Textarea) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Textarea"
	if err := checkID(component, t.ID); err != nil {
		return elem.Obj{}, err
	}
	if t.Rows < 0 {
		return elem.Obj{}, fmt.Errorf("%s: %w: negative rows", component, ErrInvalidOption)
	}
	c, err := newControl(ctx, component, t.ID, t.Label, t.Help, t.Error, t.Attrs)
	if err != nil {
		return elem.Obj{}, err
	}

	own := []attr.Node{c.class("form-control"), attr.ID(t.ID)}
	if t.Name != "" {
		own = append(own, textarea.Name(t.Name))
	}
	if t.Rows > 0 {
		own = append(own, attr.Name("rows").Raw(strconv.Itoa(t.Rows)))
	}
	if t.Placeholder != "" {
		own = append(own, textarea.Placeholder(t.Placeholder))
	}
	if t.Autocomplete != "" {
		own = append(own, textarea.Autocomplete(t.Autocomplete))
	}
	own = appendStates(own, t.Required, t.Disabled, t.ReadOnly)
	own = append(own, c.aria()...)

	var content []elem.Node
	if t.Value != "" {
		content = append(content, elem.Text(t.Value))
	}
	children := []elem.Node{
		c.label(t.LabelHidden, "form-label"),
		textarea.Textarea(compose(c.user, own...))(content...),
	}
	children = append(children, c.feedback()...)
	children = append(children, c.helpText()...)
	return wrapper(rawClass("mb-3"), t.Wrapper)(children...).Eval(ctx)
}

// SelectOption is an option of a [Select]. The value attribute is always
// rendered, also when empty (e.g. for a "Choose..." option), so that the
// submitted value never falls back to the label text.
type SelectOption struct {
	Value    string // submitted value, escaped
	Label    string // option text, escaped
	Selected bool
	Disabled bool
}

// Select is a labelled select box, with help text and a server-side
// validation error wired like those of a [Field].
type Select struct {
	ID          string         // the select's id: a token, see ValidID
	Name        string         // form field name; empty renders no name
	Label       elem.Node      // label content; without it, Attrs must give an aria-label
	LabelHidden bool           // visually hidden label
	Options     []SelectOption // the options, in order
	Multiple    bool           // allow several selected options
	Required    bool
	Disabled    bool
	Help        elem.Node // help text (div.form-text#ID-help); nil leaves it out
	Error       elem.Node // validation error (div.invalid-feedback#ID-error); nil means valid
	Attrs       attr.Node // additional attributes of the select; see Field.Attrs
	Wrapper     attr.Node // additional attributes of the wrapping div.mb-3
}

// Eval builds the select field. It fails with [ErrInvalidID] for an
// invalid ID, with [ErrNoName] without a label, and with
// [ErrInvalidOption] when a select without Multiple has more than one
// selected option.
func (s Select) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Select"
	if err := checkID(component, s.ID); err != nil {
		return elem.Obj{}, err
	}
	selected := 0
	options := make([]elem.Node, 0, len(s.Options))
	for _, o := range s.Options {
		optAttrs := []attr.Node{selectel.Value(o.Value)}
		if o.Selected {
			selected++
			optAttrs = append(optAttrs, selectel.Selected())
		}
		if o.Disabled {
			optAttrs = append(optAttrs, selectel.Disabled())
		}
		options = append(options, selectel.Option(optAttrs...)(elem.Text(o.Label)))
	}
	if !s.Multiple && selected > 1 {
		return elem.Obj{}, fmt.Errorf("%s: %w: %d options selected without Multiple", component, ErrInvalidOption, selected)
	}
	c, err := newControl(ctx, component, s.ID, s.Label, s.Help, s.Error, s.Attrs)
	if err != nil {
		return elem.Obj{}, err
	}

	own := []attr.Node{c.class("form-select"), attr.ID(s.ID)}
	if s.Name != "" {
		own = append(own, selectel.Name(s.Name))
	}
	if s.Multiple {
		own = append(own, selectel.Multiple())
	}
	if s.Required {
		own = append(own, selectel.Required())
	}
	if s.Disabled {
		own = append(own, selectel.Disabled())
	}
	own = append(own, c.aria()...)

	children := []elem.Node{
		c.label(s.LabelHidden, "form-label"),
		selectel.Select(compose(c.user, own...))(options...),
	}
	children = append(children, c.feedback()...)
	children = append(children, c.helpText()...)
	return wrapper(rawClass("mb-3"), s.Wrapper)(children...).Eval(ctx)
}

// Checkbox is a labelled checkbox (or switch), with help text and a
// server-side validation error wired like those of a [Field]:
//
//	<div class="form-check">
//	  <input class="form-check-input" type="checkbox" id="ID" name="..." value="..."/>
//	  <label class="form-check-label" for="ID">Label</label>
//	</div>
//
// Checkboxes that share a Name submit one value each when checked.
type Checkbox struct {
	ID       string    // the input's id: a token, see ValidID
	Name     string    // form field name; empty renders no name
	Value    string    // submitted value when checked; empty renders none (the browser submits "on")
	Label    elem.Node // label content; without it, Attrs must give an aria-label
	Checked  bool
	Required bool
	Disabled bool
	Switch   bool      // render as a switch (form-switch, role="switch")
	Inline   bool      // lay out in a row with other inline checks (form-check-inline)
	Help     elem.Node // help text (div.form-text#ID-help); nil leaves it out
	Error    elem.Node // validation error (div.invalid-feedback#ID-error); nil means valid
	Attrs    attr.Node // additional attributes of the input; see Field.Attrs
	Wrapper  attr.Node // additional attributes of the wrapping div.form-check
}

// Eval builds the checkbox. It fails with [ErrInvalidID] for an invalid ID
// and with [ErrNoName] without a label.
func (cb Checkbox) Eval(ctx context.Context) (elem.Obj, error) {
	return check{
		component: "bs.Checkbox",
		typ:       input.Checkbox,
		id:        cb.ID,
		name:      cb.Name,
		value:     cb.Value,
		label:     cb.Label,
		checked:   cb.Checked,
		required:  cb.Required,
		disabled:  cb.Disabled,
		isSwitch:  cb.Switch,
		inline:    cb.Inline,
		help:      cb.Help,
		err:       cb.Error,
		attrs:     cb.Attrs,
		wrapper:   cb.Wrapper,
	}.eval(ctx)
}

// Radio is a labelled radio button, with help text and a server-side
// validation error wired like those of a [Field]. Radios that share a Name
// form a group of which one can be checked; group them in a fieldset with
// a legend.
type Radio struct {
	ID       string    // the input's id: a token, see ValidID
	Name     string    // group name
	Value    string    // submitted value when checked
	Label    elem.Node // label content; without it, Attrs must give an aria-label
	Checked  bool
	Required bool
	Disabled bool
	Inline   bool      // lay out in a row with other inline checks (form-check-inline)
	Help     elem.Node // help text (div.form-text#ID-help); nil leaves it out
	Error    elem.Node // validation error (div.invalid-feedback#ID-error); nil means valid
	Attrs    attr.Node // additional attributes of the input; see Field.Attrs
	Wrapper  attr.Node // additional attributes of the wrapping div.form-check
}

// Eval builds the radio button. It fails with [ErrInvalidID] for an
// invalid ID and with [ErrNoName] without a label.
func (r Radio) Eval(ctx context.Context) (elem.Obj, error) {
	return check{
		component: "bs.Radio",
		typ:       input.Radio,
		id:        r.ID,
		name:      r.Name,
		value:     r.Value,
		label:     r.Label,
		checked:   r.Checked,
		required:  r.Required,
		disabled:  r.Disabled,
		inline:    r.Inline,
		help:      r.Help,
		err:       r.Error,
		attrs:     r.Attrs,
		wrapper:   r.Wrapper,
	}.eval(ctx)
}

// check renders a Checkbox or a Radio.
type check struct {
	component                   string
	typ                         input.InputType
	id, name, value             string
	label                       elem.Node
	checked, required, disabled bool
	isSwitch, inline            bool
	help, err                   elem.Node
	attrs, wrapper              attr.Node
}

func (k check) eval(ctx context.Context) (elem.Obj, error) {
	if err := checkID(k.component, k.id); err != nil {
		return elem.Obj{}, err
	}
	c, err := newControl(ctx, k.component, k.id, k.label, k.help, k.err, k.attrs)
	if err != nil {
		return elem.Obj{}, err
	}
	own := []attr.Node{c.class("form-check-input"), input.Type(k.typ)}
	if k.isSwitch {
		own = append(own, fallbackRaw("role", "switch"))
	}
	own = append(own, attr.ID(k.id))
	if k.name != "" {
		own = append(own, input.Name(k.name))
	}
	if k.value != "" {
		own = append(own, input.Value(k.value))
	}
	if k.checked {
		own = append(own, input.Checked())
	}
	own = appendStates(own, k.required, k.disabled, false)
	own = append(own, c.aria()...)

	wrapClass := "form-check"
	if k.isSwitch {
		wrapClass += " form-switch"
	}
	if k.inline {
		wrapClass += " form-check-inline"
	}
	children := []elem.Node{
		input.Input(compose(c.user, own...)),
		c.label(false, "form-check-label"),
	}
	children = append(children, c.feedback()...)
	children = append(children, c.helpText()...)
	return wrapper(rawClass(wrapClass), k.wrapper)(children...).Eval(ctx)
}

// control holds what the form field components share: the label, the help
// and error parts, and the caller's evaluated attributes.
type control struct {
	id          string
	labelNode   elem.Node
	help, err   elem.Node
	describedBy string      // the ids of aria-describedby, the caller's included
	user        []attr.Node // the caller's evaluated attributes, without aria-describedby
	objs        []attr.Obj  // the caller's evaluated attributes, aria-describedby included
}

// newControl evaluates the caller's attributes of a form control once,
// checks that the control has an accessible name, and merges a caller's
// aria-describedby into the control's own ids.
func newControl(ctx context.Context, component, id string, labelNode, help, errNode elem.Node, a attr.Node) (control, error) {
	objs, err := flatten(ctx, userAttrs(a), nil)
	if err != nil {
		return control{}, fmt.Errorf("%s: %w", component, err)
	}
	if labelNode == nil && !sets(objs, "aria-label") && !sets(objs, "aria-labelledby") {
		return control{}, fmt.Errorf("%s: %w: set a Label, or an aria-label in Attrs", component, ErrNoName)
	}
	var ids []string
	if errNode != nil {
		ids = append(ids, id+"-error")
	}
	if help != nil {
		ids = append(ids, id+"-help")
	}
	user := make([]attr.Node, 0, len(objs))
	for _, o := range objs {
		if asciiEqualFold(o.Key, "aria-describedby") {
			ids = append(ids, strings.Fields(logicalValue(o))...)
			continue
		}
		user = append(user, o)
	}
	return control{
		id:          id,
		labelNode:   labelNode,
		help:        help,
		err:         errNode,
		describedBy: strings.Join(ids, " "),
		user:        user,
		objs:        objs,
	}, nil
}

// logicalValue returns the value of an evaluated attribute as the browser
// reads it.
func logicalValue(o attr.Obj) string {
	switch o.Kind {
	case attr.KindValue:
		return o.Val
	case attr.KindRawValue:
		return html.UnescapeString(o.Val)
	}
	return ""
}

// class returns the control's class attribute, with is-invalid when it has
// an error.
func (c control) class(class string) attr.Node {
	if c.err != nil {
		class += " is-invalid"
	}
	return rawClass(class)
}

// aria returns the control's aria-invalid and aria-describedby attributes.
func (c control) aria() []attr.Node {
	var out []attr.Node
	if c.err != nil {
		out = append(out, fallbackRaw("aria-invalid", "true"))
	}
	if c.describedBy != "" {
		out = append(out, attr.Name("aria-describedby").Value(c.describedBy))
	}
	return out
}

// label returns the control's label, or no node without a label.
func (c control) label(hidden bool, class string) elem.Node {
	if c.labelNode == nil {
		return elem.Noop()
	}
	if hidden {
		class += " visually-hidden"
	}
	return label.Label(rawClass(class), label.For(c.id))(c.labelNode)
}

// feedback returns the control's error part.
func (c control) feedback() []elem.Node {
	if c.err == nil {
		return nil
	}
	return []elem.Node{div.Div(rawClass("invalid-feedback"), attr.ID(c.id+"-error"))(c.err)}
}

// helpText returns the control's help part.
func (c control) helpText() []elem.Node {
	if c.help == nil {
		return nil
	}
	return []elem.Node{div.Div(rawClass("form-text"), attr.ID(c.id+"-help"))(c.help)}
}

// appendStates appends the required, disabled and readonly attributes.
func appendStates(own []attr.Node, required, disabled, readOnly bool) []attr.Node {
	if required {
		own = append(own, attr.Name("required").Bool())
	}
	if disabled {
		own = append(own, attr.Name("disabled").Bool())
	}
	if readOnly {
		own = append(own, attr.Name("readonly").Bool())
	}
	return own
}

// wrapper returns a div with the class and the caller's wrapper attributes.
func wrapper(class attr.Node, a attr.Node) elem.Scope {
	return div.Div(append([]attr.Node{class}, userAttrs(a)...)...)
}
