package bs

import (
	"context"

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

	// TitleLevel is the heading level of the title; zero means h5.
	TitleLevel HeadingLevel
	// SubtitleLevel is the heading level of the subtitle; zero means h6.
	SubtitleLevel HeadingLevel
}

// Eval builds the card body structure as a fragment. A heading level out of
// range fails with [ErrInvalidOption].
func (cb CardBody) Eval(ctx context.Context) (elem.Obj, error) {
	var children []elem.Node

	if cb.Title != nil {
		h, err := heading("bs.CardBody", cb.TitleLevel, 5, rawClass("card-title"))
		if err != nil {
			return elem.Obj{}, err
		}
		children = append(children, h(cb.Title))
	}
	if cb.Subtitle != nil {
		h, err := heading("bs.CardBody", cb.SubtitleLevel, 6, rawClass("card-subtitle mb-2 text-body-secondary"))
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

// CardImgOverlay creates a card image overlay wrapper (div.card-img-overlay).
func CardImgOverlay(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("card-img-overlay"), attrs...))
}
