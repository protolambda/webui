package bs

import (
	"errors"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/section"
)

var (
	// ErrInvalidID indicates an id that is not a valid token: components
	// derive further ids from an id and use it in CSS selectors and id
	// lists, where spaces and punctuation would refer to other elements.
	// See [ValidID].
	//
	// An invalid id is an invalid option: errors wrapping ErrInvalidID also
	// match [ErrInvalidOption], so one errors.Is check for ErrInvalidOption
	// catches both.
	ErrInvalidID error = invalidID{}

	// ErrInvalidOption indicates an option value outside its closed set
	// (such as an unknown [Variant] or [ModalSize]), an id that is not a
	// token ([ErrInvalidID]), or a combination of component fields that
	// cannot render correctly.
	ErrInvalidOption = errors.New("invalid option")

	// ErrNoName indicates a component that would render without an
	// accessible name, such as a [Dialog] without a title or a [Field]
	// without a label.
	ErrNoName = errors.New("missing accessible name")
)

// invalidID is the type of [ErrInvalidID]: an error that also matches
// [ErrInvalidOption].
type invalidID struct{}

func (invalidID) Error() string {
	return "invalid id: want an ASCII letter followed by ASCII letters, digits, '-' or '_'"
}

// Is reports whether target is [ErrInvalidOption], which an invalid id is
// a kind of.
func (invalidID) Is(target error) bool { return target == ErrInvalidOption }

// ValidID reports whether id is safe to use as a component id: an ASCII
// letter followed by ASCII letters, digits, '-' or '_'. Such an id needs no
// escaping in a CSS selector ("#" + id), in an id list (aria-describedby),
// or in the ids that components derive from it ("-title", "-help", ...).
func ValidID(id string) bool {
	if id == "" || !asciiLetter(id[0]) {
		return false
	}
	for i := 1; i < len(id); i++ {
		c := id[i]
		if !asciiLetter(c) && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func asciiLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// Variant is a Bootstrap theme color, used by alerts, badges, buttons and
// other components (alert-{variant}, text-bg-{variant}, btn-{variant}).
// Components fail the render with [ErrInvalidOption] for a value outside
// the constants below.
type Variant string

// The Bootstrap theme colors.
const (
	VariantPrimary   Variant = "primary"
	VariantSecondary Variant = "secondary"
	VariantSuccess   Variant = "success"
	VariantDanger    Variant = "danger"
	VariantWarning   Variant = "warning"
	VariantInfo      Variant = "info"
	VariantLight     Variant = "light"
	VariantDark      Variant = "dark"
)

// Valid reports whether v is one of the Variant constants.
func (v Variant) Valid() bool {
	switch v {
	case VariantPrimary, VariantSecondary, VariantSuccess, VariantDanger,
		VariantWarning, VariantInfo, VariantLight, VariantDark:
		return true
	}
	return false
}

// variantClass returns the class prefix+v, or an attribute that fails the
// render when v is not valid.
func variantClass(component, prefix string, v Variant) attr.Node {
	if !v.Valid() {
		return errAttr{fmt.Errorf("%s: %w: unknown variant", component, ErrInvalidOption)}
	}
	return rawClass(prefix + string(v))
}

// ModalSize is the size of a [Modal] or [Dialog]: the modal-{size} class of
// its .modal-dialog. The zero value is Bootstrap's default width.
type ModalSize string

// The modal sizes of Bootstrap 5.3.
const (
	ModalDefault ModalSize = ""
	ModalSM      ModalSize = "sm"
	ModalLG      ModalSize = "lg"
	ModalXL      ModalSize = "xl"

	// ModalFullscreen covers the whole viewport.
	ModalFullscreen ModalSize = "fullscreen"
	// ModalFullscreenSMDown and the other "Down" sizes cover the whole
	// viewport below the breakpoint, and have the default width above it.
	ModalFullscreenSMDown  ModalSize = "fullscreen-sm-down"
	ModalFullscreenMDDown  ModalSize = "fullscreen-md-down"
	ModalFullscreenLGDown  ModalSize = "fullscreen-lg-down"
	ModalFullscreenXLDown  ModalSize = "fullscreen-xl-down"
	ModalFullscreenXXLDown ModalSize = "fullscreen-xxl-down"
)

// Valid reports whether s is one of the ModalSize constants.
func (s ModalSize) Valid() bool {
	switch s {
	case ModalDefault, ModalSM, ModalLG, ModalXL, ModalFullscreen,
		ModalFullscreenSMDown, ModalFullscreenMDDown, ModalFullscreenLGDown,
		ModalFullscreenXLDown, ModalFullscreenXXLDown:
		return true
	}
	return false
}

// modalDialogClass returns the class list of a .modal-dialog element.
func modalDialogClass(component string, size ModalSize, centered, scrollable bool) (string, error) {
	if !size.Valid() {
		return "", fmt.Errorf("%s: %w: unknown size", component, ErrInvalidOption)
	}
	class := "modal-dialog"
	if size != ModalDefault {
		class += " modal-" + string(size)
	}
	if centered {
		class += " modal-dialog-centered"
	}
	if scrollable {
		class += " modal-dialog-scrollable"
	}
	return class, nil
}

// OffcanvasPlacement is the edge of the viewport that an [Offcanvas] slides
// in from. The zero value is the start edge.
type OffcanvasPlacement string

// The offcanvas placements of Bootstrap 5.3.
const (
	OffcanvasStart  OffcanvasPlacement = "start"
	OffcanvasEnd    OffcanvasPlacement = "end"
	OffcanvasTop    OffcanvasPlacement = "top"
	OffcanvasBottom OffcanvasPlacement = "bottom"
)

// Valid reports whether p is empty or one of the OffcanvasPlacement
// constants.
func (p OffcanvasPlacement) Valid() bool {
	switch p {
	case "", OffcanvasStart, OffcanvasEnd, OffcanvasTop, OffcanvasBottom:
		return true
	}
	return false
}

// HeadingLevel is the level of a component's heading element: 1 to 6 for h1
// to h6. The zero value selects the component's default level (h2 for
// dialog, modal, offcanvas and accordion headings). Components fail the
// render with [ErrInvalidOption] for other values.
type HeadingLevel int

// heading returns the heading element constructor of the level, or def for
// the zero level.
func heading(component string, level, def HeadingLevel, attrs ...attr.Node) (elem.Scope, error) {
	if level == 0 {
		level = def
	}
	switch level {
	case 1:
		return section.H1(attrs...), nil
	case 2:
		return section.H2(attrs...), nil
	case 3:
		return section.H3(attrs...), nil
	case 4:
		return section.H4(attrs...), nil
	case 5:
		return section.H5(attrs...), nil
	case 6:
		return section.H6(attrs...), nil
	}
	return nil, fmt.Errorf("%s: %w: heading level out of range 1 to 6", component, ErrInvalidOption)
}

// InputMode is the virtual keyboard hint of a text field (the inputmode
// attribute). The zero value renders no hint.
type InputMode string

// The inputmode values of HTML.
const (
	InputModeNone    InputMode = "none"
	InputModeText    InputMode = "text"
	InputModeDecimal InputMode = "decimal"
	InputModeNumeric InputMode = "numeric"
	InputModeTel     InputMode = "tel"
	InputModeSearch  InputMode = "search"
	InputModeEmail   InputMode = "email"
	InputModeURL     InputMode = "url"
)

// Valid reports whether m is empty or one of the InputMode constants.
func (m InputMode) Valid() bool {
	switch m {
	case "", InputModeNone, InputModeText, InputModeDecimal, InputModeNumeric,
		InputModeTel, InputModeSearch, InputModeEmail, InputModeURL:
		return true
	}
	return false
}
