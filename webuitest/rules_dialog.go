package webuitest

import (
	"fmt"
	"strings"

	"github.com/protolambda/chord/ct"
)

// ModalDialogs checks the markup of modal dialogs that work without script
// (bs.Dialog): a dialog element opened and closed by buttons with invoker
// commands (commandfor and command), which browsers without invoker
// commands replace by the buttons' popovertarget:
//
//	<dialog id="ID" class="modal" popover aria-labelledby="ID-title"> ... </dialog>
//	<button type="button" commandfor="ID" command="show-modal" popovertarget="ID">
//	<button type="button" commandfor="ID" command="close" popovertarget="ID" popovertargetaction="hide">
//
// Every dialog element has an id, the class modal, the popover attribute,
// an accessible name (aria-labelledby or aria-label), and a control with
// autofocus (a popover moves the focus only to such a control); it is
// never rendered open (which shows it without modality and makes
// show-modal fail), and it is not inside a form (forms do not nest, and a
// dialog often holds a form).
//
// Every element with commandfor is a button of type button that targets
// a dialog with the command show-modal, close or request-close, and has a
// popovertarget naming the same dialog; close buttons sit inside their
// dialog and carry popovertargetaction="hide", and openers do not hide.
// No popovertarget names a dialog without commandfor, which would open it
// as a non-modal popover in every browser. Every dialog has an opener and
// a close button.
//
// In a fragment (a subject without an html element) dialogs and their
// openers may be in different responses, so a commandfor or popovertarget
// whose dialog is missing, and a dialog without an opener, are not
// reported there; parse a fragment answer with
// [cthttp.Result.HTMLFragment] for that, as Result.HTML always makes the
// subject a document (see [Rules]). On a complete page, the dialogs whose
// ids are among shared need no opener either: dialogs that the document
// renders on every page for the openers that later fragments bring (see
// [WithSharedDialogs]). Every other check applies to them.
func ModalDialogs(shared ...string) ct.Rule {
	isShared := make(map[string]bool, len(shared))
	quoted := make([]string, len(shared))
	for i, id := range shared {
		isShared[id] = true
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return ct.CustomRule("modalDialogs("+strings.Join(quoted, ", ")+")", func(root ct.Node) []string {
		frag := isFragment(root)
		ids := idIndex(root)
		var out []string
		var dialogs []ct.Node
		for d := range elementsByTag(root, "dialog") {
			dialogs = append(dialogs, d)
			where := "dialog at " + loc(d)
			if id, _ := attrOf(d, "id"); id == "" {
				out = append(out, where+" has no id")
			}
			if !HasClass(d, "modal") {
				out = append(out, where+" lacks the class modal")
			}
			if !hasAttr(d, "popover") {
				out = append(out, where+" has no popover attribute, which browsers without invoker commands need")
			}
			if l, ok := attrOf(d, "aria-labelledby"); !ok || strings.TrimSpace(l) == "" {
				if l, ok := attrOf(d, "aria-label"); !ok || strings.TrimSpace(l) == "" {
					out = append(out, where+" has no aria-labelledby or aria-label")
				}
			}
			if hasAttr(d, "open") {
				out = append(out, where+" is rendered open")
			}
			if inside(d, "form") != nil {
				out = append(out, where+" is inside a form")
			}
			focus := false
			for c := range ct.Elements(d) {
				if hasAttr(c, "autofocus") {
					focus = true
					break
				}
			}
			if !focus {
				out = append(out, where+" has no autofocus control")
			}
		}
		openers := make(map[string]bool)
		closers := make(map[string]bool)
		for e := range ct.Elements(root) {
			target, hasCommand := attrOf(e, "commandfor")
			popover, hasPopover := attrOf(e, "popovertarget")
			if !hasCommand {
				if hasPopover {
					t := ids[popover]
					switch {
					case t == nil && !frag:
						out = append(out, fmt.Sprintf("popovertarget at %s targets missing id %q", loc(e), popover))
					case t != nil && t.Tag() == "dialog":
						out = append(out, fmt.Sprintf("popovertarget at %s opens dialog %q without commandfor, as a non-modal popover", loc(e), popover))
					}
				}
				continue
			}
			where := fmt.Sprintf("commandfor at %s", loc(e))
			if t, _ := attrOf(e, "type"); e.Tag() != "button" || !strings.EqualFold(strings.TrimSpace(t), "button") {
				out = append(out, where+" is not on a button of type button")
			}
			t := ids[target]
			switch {
			case t == nil:
				if !frag {
					out = append(out, fmt.Sprintf("%s targets missing id %q", where, target))
				}
			case t.Tag() != "dialog":
				out = append(out, fmt.Sprintf("%s targets a %s, not a dialog", where, t.Tag()))
				continue
			}
			if !hasPopover || popover != target {
				out = append(out, where+" has no popovertarget naming the same dialog")
			}
			action, _ := attrOf(e, "popovertargetaction")
			action = strings.ToLower(strings.TrimSpace(action))
			switch command, _ := attrOf(e, "command"); strings.ToLower(strings.TrimSpace(command)) {
			case "show-modal":
				openers[target] = true
				if action == "hide" {
					out = append(out, where+" opens the dialog but its popovertargetaction hides it")
				}
			case "close", "request-close":
				closers[target] = true
				if t != nil && !isAncestor(t, e) {
					out = append(out, where+" closes a dialog it is not inside")
				}
				if action != "hide" {
					out = append(out, where+" closes the dialog without popovertargetaction=\"hide\"")
				}
			default:
				out = append(out, fmt.Sprintf("%s has command %q, not show-modal, close or request-close", where, command))
			}
		}
		for _, d := range dialogs {
			id, _ := attrOf(d, "id")
			if id == "" {
				continue
			}
			if !openers[id] && !frag && !isShared[id] {
				out = append(out, "dialog at "+loc(d)+" has no opener (command=\"show-modal\")")
			}
			if !closers[id] {
				out = append(out, "dialog at "+loc(d)+" has no close button (command=\"close\")")
			}
		}
		return out
	})
}

// NoBootstrapJSHooks forbids the data-bs-* attributes of Bootstrap's
// script (data-bs-toggle, data-bs-dismiss, data-bs-target and the others),
// which do nothing on pages without it: a modal, collapse, dropdown or
// dismiss button that needs them stays dead. data-bs-theme, which
// Bootstrap's CSS reads, is allowed. Use the no-JS components of the bs
// package instead (bs.Dialog, bs.DetailsAccordion, bs.DetailsDropdown), or
// drop this rule with [WithBootstrapJS] on pages that load the script.
func NoBootstrapJSHooks() ct.Rule {
	return ct.CustomRule("noBootstrapJSHooks()", func(root ct.Node) []string {
		var out []string
		for e := range ct.Elements(root) {
			for key := range e.Attrs() {
				lk := strings.ToLower(key)
				if strings.HasPrefix(lk, "data-bs-") && lk != "data-bs-theme" {
					out = append(out, fmt.Sprintf("%s at %s needs Bootstrap's script", key, loc(e)))
				}
			}
		}
		return out
	})
}
