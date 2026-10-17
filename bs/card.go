package bs

import (
	"context"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"
)

// Card represents a Bootstrap card component (div.card). Every field is
// optional: a nil part is left out. Attrs are rendered on the div.card,
// before its class, so a class in Attrs comes first ("x card").
type Card struct {
	Header elem.Node // optional .card-header content
	Img    elem.Node // optional image, rendered as is: give it the card-img-top class
	Body   elem.Node // .card-body content (use CardBody for a structured body)
	Footer elem.Node // optional .card-footer content
	Attrs  attr.Node // additional attributes of the div.card
}

// Eval builds the card structure with Bootstrap markup.
func (c Card) Eval(ctx context.Context) (elem.Obj, error) {
	var cardAttrs []attr.Node
	if c.Attrs != nil {
		cardAttrs = append(cardAttrs, c.Attrs)
	}
	cardAttrs = append(cardAttrs, rawClass("card"))

	var children []elem.Node
	if c.Header != nil {
		children = append(children, div.Div(rawClass("card-header"))(c.Header))
	}
	if c.Img != nil {
		children = append(children, c.Img)
	}
	if c.Body != nil {
		children = append(children, div.Div(rawClass("card-body"))(c.Body))
	}
	if c.Footer != nil {
		children = append(children, div.Div(rawClass("card-footer"))(c.Footer))
	}

	return div.Div(cardAttrs...)(children...).Eval(ctx)
}

// CardBody represents a structured Bootstrap card body, rendered as a
// fragment to put in [Card.Body]. Every field is optional.
type CardBody struct {
	Title    elem.Node // .card-title heading
	Subtitle elem.Node // .card-subtitle heading
	Text     elem.Node // p.card-text content
	Content  elem.Node // additional body content

	// TitleLevel is the heading level of the title; zero means h5. The
	// level places the title in the page's outline; the title keeps the
	// size of an h5, which another level gets from the h5 class, as the
	// titles of a [Dialog] and a [Modal] do.
	TitleLevel HeadingLevel
	// SubtitleLevel is the heading level of the subtitle; zero means h6.
	// Another level gets the h6 class, so that the size stays that of an
	// h6.
	SubtitleLevel HeadingLevel
}

// Eval builds the card body structure as a fragment. A heading level out of
// range fails with [ErrInvalidOption].
func (cb CardBody) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.CardBody"
	var children []elem.Node

	if cb.Title != nil {
		h, err := heading(component, cb.TitleLevel, 5, rawClass(sizedHeadingClass("card-title", cb.TitleLevel, 5)))
		if err != nil {
			return elem.Obj{}, err
		}
		children = append(children, h(cb.Title))
	}
	if cb.Subtitle != nil {
		class := sizedHeadingClass("card-subtitle", cb.SubtitleLevel, 6) + " mb-2 text-body-secondary"
		h, err := heading(component, cb.SubtitleLevel, 6, rawClass(class))
		if err != nil {
			return elem.Obj{}, err
		}
		children = append(children, h(cb.Subtitle))
	}
	if cb.Text != nil {
		children = append(children, text.P(rawClass("card-text"))(cb.Text))
	}
	if cb.Content != nil {
		children = append(children, cb.Content)
	}

	return elem.Bundle(children).Eval(ctx)
}

// sizedHeadingClass returns class, followed by the size class of the default
// level def (h5 for 5) when level is another one, so that the heading keeps
// the size of the default level. An invalid level is left to heading.
func sizedHeadingClass(class string, level, def HeadingLevel) string {
	if level == 0 || level == def {
		return class
	}
	return class + " h" + strconv.Itoa(int(def))
}

// CardImgOverlay creates a card image overlay wrapper (div.card-img-overlay).
func CardImgOverlay(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("card-img-overlay"), attrs...))
}
