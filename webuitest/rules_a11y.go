package webuitest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/protolambda/chord/ct"

	"github.com/protolambda/webui/page"
)

// IconsHidden requires that every Bootstrap Icons glyph (an element with
// the class bi) is hidden from assistive technology, by aria-hidden="true"
// on itself or an ancestor, or is an image with a name: role="img" and an
// aria-label or aria-labelledby. The glyph is a private-use character of
// the icon font that screen readers would otherwise read out. The bi
// package renders icons this way; an icon-only control names the control
// instead ([ControlsNamed]).
func IconsHidden() ct.Rule {
	return ct.CustomRule("iconsHidden()", func(root ct.Node) []string {
		var out []string
		for e := range ct.Elements(root) {
			if !HasClass(e, "bi") {
				continue
			}
			hidden := ct.Closest(e, func(n ct.Node) bool { return AttrIs(n, "aria-hidden", "true") }) != nil
			if hidden || namedImage(e) {
				continue
			}
			out = append(out, "icon at "+loc(e)+" is neither aria-hidden=\"true\" nor role=\"img\" with a label")
		}
		return out
	})
}

func namedImage(e ct.Node) bool {
	role, _ := attrOf(e, "role")
	if f := strings.Fields(role); len(f) == 0 || !strings.EqualFold(f[0], "img") {
		return false
	}
	for _, name := range []string{"aria-label", "aria-labelledby"} {
		if v, ok := attrOf(e, name); ok && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// controlRoles are the explicit widget roles that need an accessible name.
var controlRoles = map[string]bool{
	"button": true, "link": true, "checkbox": true, "radio": true, "switch": true,
	"tab": true, "menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"option": true, "combobox": true, "textbox": true, "searchbox": true,
	"slider": true, "spinbutton": true, "treeitem": true,
}

// ControlsNamed requires an accessible name ([ct.AccessibleName]) on every
// control: buttons, links (a and area with href), inputs other than
// hidden ones, selects, textareas, and elements with an explicit widget
// role. An icon-only button needs an aria-label, as the icon glyph is no
// text. A name that cannot be computed, because aria-labelledby names a
// missing id, is reported too.
func ControlsNamed() ct.Rule {
	return ct.CustomRule("controlsNamed()", func(root ct.Node) []string {
		var out []string
		for e := range ct.Elements(root) {
			if !control(e) {
				continue
			}
			name, err := ct.AccessibleName(e)
			switch {
			case err != nil:
				out = append(out, fmt.Sprintf("%s at %s has a name that cannot be computed: %v", e.Tag(), loc(e), err))
			case name == "":
				out = append(out, fmt.Sprintf("%s at %s has no accessible name", e.Tag(), loc(e)))
			}
		}
		return out
	})
}

func control(e ct.Node) bool {
	if role, ok := attrOf(e, "role"); ok {
		if f := strings.Fields(role); len(f) > 0 {
			return controlRoles[strings.ToLower(f[0])]
		}
	}
	switch e.Tag() {
	case "button", "select", "textarea":
		return true
	case "a", "area":
		return hasAttr(e, "href")
	case "input":
		return inputType(e) != "hidden"
	}
	return false
}

// interactive reports whether e is interactive content of HTML, or has a
// tabindex, which neither links nor buttons may contain.
func interactive(e ct.Node) bool {
	if hasAttr(e, "tabindex") {
		return true
	}
	switch e.Tag() {
	case "a":
		return hasAttr(e, "href")
	case "button", "details", "embed", "iframe", "label", "select", "textarea":
		return true
	case "input":
		return inputType(e) != "hidden"
	case "audio", "video":
		return hasAttr(e, "controls")
	case "img", "object":
		return hasAttr(e, "usemap")
	}
	return false
}

// NoNestedInteractive forbids interactive content (links, buttons, form
// controls, labels, details, frames, media with controls, and any element
// with tabindex) inside a link or a button. HTML does not allow it:
// browsers handle the nested control inconsistently, and its name merges
// into the outer control's accessible name.
func NoNestedInteractive() ct.Rule {
	return ct.CustomRule("noNestedInteractive()", func(root ct.Node) []string {
		var out []string
		for e := range ct.Elements(root) {
			if !interactive(e) {
				continue
			}
			outer := ct.Closest(e.Parent(), func(n ct.Node) bool {
				return n.Tag() == "button" || (n.Tag() == "a" && hasAttr(n, "href"))
			})
			if outer != nil {
				out = append(out, fmt.Sprintf("%s at %s is inside the %s at %s", e.Tag(), loc(e), outer.Tag(), loc(outer)))
			}
		}
		return out
	})
}

// RectangularTables requires that every row of a table spans as many
// columns as its first row (the first header row, when the table has a
// thead), counting colspan, and rowspan within each row group, as the
// HTML table model does. A ragged table misaligns cells with their headers
// for everyone, and breaks the header association that screen readers
// announce.
func RectangularTables() ct.Rule {
	return ct.CustomRule("rectangularTables()", func(root ct.Node) []string {
		var out []string
		for t := range elementsByTag(root, "table") {
			out = append(out, tableProblems(t)...)
		}
		return out
	})
}

// tableProblems checks the row widths of one table.
func tableProblems(table ct.Node) []string {
	type group struct{ rows []ct.Node }
	var groups []group
	var head *group
	var direct group
	for c := range table.Children() {
		if c.Kind() != ct.KindElement {
			continue
		}
		switch c.Tag() {
		case "thead", "tbody", "tfoot":
			var g group
			for r := range c.Children() {
				if r.Kind() == ct.KindElement && r.Tag() == "tr" {
					g.rows = append(g.rows, r)
				}
			}
			groups = append(groups, g)
			if c.Tag() == "thead" && head == nil {
				head = &groups[len(groups)-1]
			}
		case "tr":
			direct.rows = append(direct.rows, c)
		}
	}
	if len(direct.rows) > 0 {
		groups = append(groups, direct)
	}
	widths := make(map[ct.Node]int)
	for _, g := range groups {
		for r, w := range groupWidths(g.rows) {
			widths[r] = w
		}
	}
	var first ct.Node
	if head != nil && len(head.rows) > 0 {
		first = head.rows[0]
	} else {
		for _, g := range groups {
			if len(g.rows) > 0 {
				first = g.rows[0]
				break
			}
		}
	}
	if first == nil {
		return nil
	}
	want := widths[first]
	var out []string
	for _, g := range groups {
		for _, r := range g.rows {
			if w := widths[r]; w != want {
				out = append(out, fmt.Sprintf("row at %s spans %d columns, the first row at %s spans %d", loc(r), w, loc(first), want))
			}
		}
	}
	return out
}

// groupWidths returns the number of columns each row of a row group
// covers, with the cells of earlier rows that span into it.
func groupWidths(rows []ct.Node) map[ct.Node]int {
	out := make(map[ct.Node]int, len(rows))
	// above holds, per column, how many more rows a cell from an earlier
	// row covers.
	above := make(map[int]int)
	for i, r := range rows {
		width := 0
		for c, n := range above {
			if n > 0 && c+1 > width {
				width = c + 1
			}
		}
		x := 0
		placed := make(map[int]int)
		for cell := range r.Children() {
			if cell.Kind() != ct.KindElement || (cell.Tag() != "td" && cell.Tag() != "th") {
				continue
			}
			for above[x] > 0 {
				x++
			}
			colspan := spanAttr(cell, "colspan", 1, 1000)
			rowspan := spanAttr(cell, "rowspan", 1, 65534)
			if v, ok := attrOf(cell, "rowspan"); ok && strings.TrimSpace(v) == "0" {
				rowspan = len(rows) - i // to the end of the group
			}
			for k := 0; k < colspan; k++ {
				if rowspan > 1 {
					placed[x+k] = rowspan - 1
				}
			}
			x += colspan
		}
		if x > width {
			width = x
		}
		out[r] = width
		for c := range above {
			if above[c]--; above[c] <= 0 {
				delete(above, c)
			}
		}
		for c, n := range placed {
			above[c] = n
		}
	}
	return out
}

// spanAttr reads a colspan or rowspan value as HTML does: a non-negative
// integer, def when missing or invalid, clamped to max.
func spanAttr(cell ct.Node, name string, def, max int) int {
	v, ok := attrOf(cell, name)
	if !ok {
		return def
	}
	v = strings.TrimSpace(v)
	end := 0
	for end < len(v) && v[end] >= '0' && v[end] <= '9' {
		end++
	}
	if end > 6 {
		return max
	}
	n, err := strconv.Atoi(v[:end])
	if err != nil || n < 1 {
		return def
	}
	return min(n, max)
}

// ExternalLinksSafe requires rel="noopener" (or noreferrer, which implies
// it) on every link, area and form that opens a new browsing context
// (target="_blank" or a named target): without it, the opened page gets a
// handle on this window and can navigate it elsewhere. Current browsers
// apply noopener to target="_blank" by themselves; the rule keeps pages
// safe in older ones and makes the intent explicit.
func ExternalLinksSafe() ct.Rule {
	return ct.CustomRule("externalLinksSafe()", func(root ct.Node) []string {
		var out []string
		for e := range ct.Elements(root) {
			switch e.Tag() {
			case "a", "area", "form":
			default:
				continue
			}
			target, ok := attrOf(e, "target")
			target = strings.ToLower(strings.TrimSpace(target))
			if !ok || target == "" || target == "_self" || target == "_parent" || target == "_top" {
				continue
			}
			rel, _ := attrOf(e, "rel")
			if !hasFieldFold(rel, "noopener") && !hasFieldFold(rel, "noreferrer") {
				out = append(out, fmt.Sprintf("%s at %s opens a new browsing context without rel=\"noopener\"", e.Tag(), loc(e)))
			}
		}
		return out
	})
}

// LayoutRegions checks the regions of webui's page layout (page.Layout) in
// a full document:
//
//   - one main element with id [page.MainID];
//   - a region with id [page.AlertsID] inside it that announces changes
//     (aria-live, or role alert or status);
//   - the page header ([page.HeaderID]), when present, inside main;
//   - the navbar ([page.NavbarID]) and the sidebar ([page.SidebarID]), when
//     present, nav elements outside main;
//   - a region with id [page.DialogsID] outside main and outside every
//     form, which holds every dialog element.
//
// Fragments are not checked. The rule is not part of [Rules], since pages
// may use other layouts.
func LayoutRegions() ct.Rule {
	return ct.CustomRule("layoutRegions()", func(root ct.Node) []string {
		if isFragment(root) {
			return nil
		}
		ids := idIndex(root)
		var out []string
		main := ids[page.MainID]
		switch {
		case main == nil:
			out = append(out, fmt.Sprintf("no element with id %q", page.MainID))
		case main.Tag() != "main":
			out = append(out, fmt.Sprintf("element with id %q at %s is a %s, not main", page.MainID, loc(main), main.Tag()))
		}
		if alerts := ids[page.AlertsID]; alerts == nil {
			out = append(out, fmt.Sprintf("no element with id %q", page.AlertsID))
		} else {
			if main != nil && !isAncestor(main, alerts) {
				out = append(out, fmt.Sprintf("%q region at %s is outside main", page.AlertsID, loc(alerts)))
			}
			role, _ := attrOf(alerts, "role")
			if !hasAttr(alerts, "aria-live") && !hasFieldFold(role, "alert") && !hasFieldFold(role, "status") {
				out = append(out, fmt.Sprintf("%q region at %s has no aria-live", page.AlertsID, loc(alerts)))
			}
		}
		if header := ids[page.HeaderID]; header != nil && main != nil && !isAncestor(main, header) {
			out = append(out, fmt.Sprintf("%q at %s is outside main", page.HeaderID, loc(header)))
		}
		for _, id := range []string{page.NavbarID, page.SidebarID} {
			nav := ids[id]
			switch {
			case nav == nil:
			case nav.Tag() != "nav":
				out = append(out, fmt.Sprintf("element with id %q at %s is a %s, not nav", id, loc(nav), nav.Tag()))
			case main != nil && isAncestor(main, nav):
				out = append(out, fmt.Sprintf("%q at %s is inside main", id, loc(nav)))
			}
		}
		dialogs := ids[page.DialogsID]
		if dialogs == nil {
			out = append(out, fmt.Sprintf("no element with id %q", page.DialogsID))
		} else {
			if main != nil && (isAncestor(main, dialogs) || main == dialogs) {
				out = append(out, fmt.Sprintf("%q region at %s is inside main", page.DialogsID, loc(dialogs)))
			}
			if inside(dialogs, "form") != nil {
				out = append(out, fmt.Sprintf("%q region at %s is inside a form", page.DialogsID, loc(dialogs)))
			}
		}
		for d := range elementsByTag(root, "dialog") {
			if dialogs == nil || !isAncestor(dialogs, d) {
				out = append(out, fmt.Sprintf("dialog at %s is outside the %q region", loc(d), page.DialogsID))
			}
		}
		return out
	})
}
