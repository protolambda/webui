package page

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
)

var (
	// ErrInvalid indicates a component option that cannot render correctly,
	// such as an unknown color mode or variant, a malformed language tag, an
	// id that is not a token (see [bs.ValidID]), or a URL that is not
	// allowed where it is used. Errors wrapping it name the component and
	// the option, never the value.
	//
	// It is bs.ErrInvalidOption, so one errors.Is check covers the page
	// components and the bs components placed in a page. Errors for an
	// invalid id also wrap bs.ErrInvalidID.
	ErrInvalid = bs.ErrInvalidOption

	// ErrNoName indicates a component that would render without an
	// accessible name, such as a [Document] without a title or a [NavItem]
	// without a label.
	//
	// It is bs.ErrNoName, so one errors.Is check also catches the bs
	// components of a [Layout] without a name, such as a bs.Dialog without
	// a title or a bs.Field without a label.
	ErrNoName = bs.ErrNoName
)

// Ids of the page regions. They are part of the page contract: fragments,
// out-of-band swaps and handlers (see the web package) target them.
const (
	// MainID is the id of the main landmark, the target of the skip link.
	MainID = "main"
	// AlertsID is the id of the alerts region at the top of the main
	// content: a polite live region.
	AlertsID = "alerts"
	// DialogsID is the id of the dialogs region after the page content.
	DialogsID = "dialogs"
	// HeaderID is the id of the page [Header].
	HeaderID = "page-header"
	// NavbarID is the default id of the [Navbar].
	NavbarID = "navbar"
	// SidebarID is the default id of the [Sidebar] navigation.
	SidebarID = "sidebar"
)

// fail returns an error of component wrapping sentinel, with a fixed
// detail that names the option (never its value).
func fail(component string, sentinel error, detail string) error {
	return fmt.Errorf("%s: %w: %s", component, sentinel, detail)
}

// blank reports whether s is empty or only white space.
func blank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// printableASCII reports whether s is non-empty and consists of printable
// ASCII other than space and backslash. Browsers drop tabs and newlines
// inside URLs, strip leading and trailing spaces, and read a backslash as a
// slash, so any of these could change what a URL means.
func printableASCII(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c <= ' ' || c >= 0x7f || c == '\\' {
			return false
		}
	}
	return true
}

// linkURL reports whether href may be the target of a navigation link: a
// relative reference, or an absolute http or https URL, written in
// printable ASCII without spaces or backslashes (escape other characters,
// e.g. with url.PathEscape). Other schemes, such as javascript: and data:,
// are refused.
func linkURL(href string) bool {
	if !printableASCII(href) {
		return false
	}
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "", "http", "https":
		return true
	}
	return false
}

// localPath reports whether href is a path on the page's own origin: it
// starts with a single slash, and is printable ASCII without spaces or
// backslashes.
func localPath(href string) bool {
	if !printableASCII(href) || href[0] != '/' || strings.HasPrefix(href, "//") {
		return false
	}
	u, err := url.Parse(href)
	return err == nil && u.Scheme == "" && u.Host == "" && u.User == nil
}

// iconURL reports whether href may be the favicon of a page: a path on the
// page's own origin, or a data: URL of an image (or the empty "data:,").
// The icon is never loaded from another origin.
func iconURL(href string) bool {
	if strings.HasPrefix(href, "data:") {
		return printableASCII(href) && (href == "data:," || strings.HasPrefix(href, "data:image/"))
	}
	return localPath(href)
}

// validLang reports whether tag has the shape of a BCP 47 language tag: a
// primary subtag of 2 to 8 ASCII letters, then subtags of 1 to 8 ASCII
// letters or digits, separated by hyphens, at most 35 bytes in total.
func validLang(tag string) bool {
	if tag == "" || len(tag) > 35 {
		return false
	}
	for i, sub := range strings.Split(tag, "-") {
		if len(sub) < 1 || len(sub) > 8 || (i == 0 && len(sub) < 2) {
			return false
		}
		for j := 0; j < len(sub); j++ {
			c := sub[j]
			letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
			digit := c >= '0' && c <= '9'
			if !letter && (i == 0 || !digit) {
				return false
			}
		}
	}
	return true
}

// iconText renders an optional decorative icon followed by the label text.
// The icon is spaced with a margin, not a space, so that the accessible
// name is the label alone.
func iconText(icon bi.Icon, margin string, label string) elem.Node {
	if icon == "" {
		return text.Text(label)
	}
	return elem.Bundle{icon.With(attr.Class(margin)), text.Text(label)}
}

// badge renders a navigation badge after a space, so that the accessible
// name of the link reads "Inbox 3" rather than "Inbox3".
func badge(component string, label string, v bs.Variant, extra ...attr.Node) (elem.Node, error) {
	if label == "" {
		return elem.Noop(), nil
	}
	if v == "" {
		v = bs.VariantSecondary
	}
	if !v.Valid() {
		return nil, fail(component, ErrInvalid, "badge variant")
	}
	return elem.Bundle{text.Text(" "), bs.BadgeOf(v, extra...)(text.Text(label))}, nil
}

// checkID returns the id to render: def when id is empty, else id when it
// is a valid token. An invalid id fails with an error wrapping both
// [ErrInvalid] and bs.ErrInvalidID, which is how bs classifies ids.
func checkID(component, id, def string) (string, error) {
	if id == "" {
		return def, nil
	}
	if !bs.ValidID(id) {
		return "", fmt.Errorf("%s: %w: id: %w", component, ErrInvalid, bs.ErrInvalidID)
	}
	return id, nil
}

// optionalAttrs returns a as an attribute list: empty when a is nil.
func optionalAttrs(a attr.Node) []attr.Node {
	if a == nil {
		return nil
	}
	return []attr.Node{a}
}
