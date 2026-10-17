package bs

import (
	"context"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
)

// Offcanvas represents a Bootstrap offcanvas component, a panel that slides
// in from an edge of the viewport.
//
// An Offcanvas needs Bootstrap's script (the theme package's opt-in
// Bootstrap JS asset): without it the panel never shows. Without script,
// prefer a [Dialog] or always-visible content.
//
// The markup follows Bootstrap 5.3: div.offcanvas.offcanvas-{placement}#ID
// tabindex="-1" aria-labelledby="ID-title", holding .offcanvas-header (the
// title heading with id ID-title and a close button) and .offcanvas-body.
// Attrs are added to the div.offcanvas; they may replace the tabindex and
// aria-labelledby defaults, but must not set an id.
type Offcanvas struct {
	ID         string             // required id: a token, see ValidID
	Title      elem.Node          // .offcanvas-title content, the panel's accessible name; nil leaves out the title
	TitleLevel HeadingLevel       // heading level of the title; zero means h2
	CloseLabel string             // accessible name of the close button; empty means "Close"
	Body       elem.Node          // .offcanvas-body content; nil leaves out the body
	Placement  OffcanvasPlacement // edge to slide in from; zero means OffcanvasStart
	// Backdrop shows Bootstrap's backdrop behind the open panel. Unlike
	// Bootstrap's own default, the zero value has no backdrop
	// (data-bs-backdrop="false"), as in earlier versions of this type.
	Backdrop bool
	Scroll   bool      // allow the page to scroll while the panel is open
	Attrs    attr.Node // additional attributes of the div.offcanvas
}

// Eval builds the offcanvas structure with Bootstrap markup. It fails with
// [ErrInvalidID] for an invalid ID and with [ErrInvalidOption] for an
// unknown Placement or TitleLevel.
func (o Offcanvas) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Offcanvas"
	if err := checkID(component, o.ID); err != nil {
		return elem.Obj{}, err
	}
	if !o.Placement.Valid() {
		return elem.Obj{}, fmt.Errorf("%s: %w: unknown placement", component, ErrInvalidOption)
	}
	placement := o.Placement
	if placement == "" {
		placement = OffcanvasStart
	}
	titleID := o.ID + "-title"

	own := []attr.Node{
		rawClass("offcanvas offcanvas-" + string(placement)),
		attr.ID(o.ID),
		fallbackRaw("tabindex", "-1"),
	}
	if o.Title != nil {
		own = append(own, fallbackValue("aria-labelledby", titleID))
	}
	if o.Scroll {
		own = append(own, attr.Name("data-bs-scroll").Raw("true"))
	}
	if !o.Backdrop {
		own = append(own, attr.Name("data-bs-backdrop").Raw("false"))
	}

	var headerChildren []elem.Node
	if o.Title != nil {
		h, err := heading(component, o.TitleLevel, 2, rawClass("offcanvas-title h5"), attr.ID(titleID))
		if err != nil {
			return elem.Obj{}, err
		}
		headerChildren = append(headerChildren, h(o.Title))
	}
	headerChildren = append(headerChildren, dismissButton("offcanvas", o.CloseLabel))

	children := []elem.Node{div.Div(rawClass("offcanvas-header"))(headerChildren...)}
	if o.Body != nil {
		children = append(children, div.Div(rawClass("offcanvas-body"))(o.Body))
	}

	return div.Div(compose(userAttrs(o.Attrs), own...))(children...).Eval(ctx)
}

// OffcanvasTrigger creates a button that opens the [Offcanvas] with the
// given id through Bootstrap's script: <button type="button"
// data-bs-toggle="offcanvas" data-bs-target="#id" aria-controls="id">. It
// has no classes; add them among attrs. A type among attrs replaces the
// default. An invalid id fails the render with [ErrInvalidID].
func OffcanvasTrigger(targetID string, attrs ...attr.Node) elem.Scope {
	return button.Button(compose(attrs,
		typeButton,
		attr.Name("data-bs-toggle").Raw("offcanvas"),
		idTarget("bs.OffcanvasTrigger", "data-bs-target", targetID),
		fallbackValue("aria-controls", targetID),
	))
}
