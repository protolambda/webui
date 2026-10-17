package app

import (
	"net/http"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/web"
)

const jsModal = "js-modal"

// bootstrapJS renders the opt-in page: it loads the second theme asset set,
// which adds Bootstrap's script, and shows the components that need it.
// Every other page works without that script.
func (a *App) bootstrapJS(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, a.page(r, view{
		Title:       "Bootstrap JS (opt-in)",
		Icon:        bi.CodeSquare,
		BootstrapJS: true,
		Alerts: []elem.Node{bs.AlertWarning(attr.Class("d-flex gap-2"))(
			bi.ExclamationTriangle,
			text.Span()(
				text.Strong()(text.Text("Opt-in. ")),
				text.Text("Only this page loads Bootstrap's script, from a second theme asset set built with theme.Options{BootstrapJS: true}. The components below need it; the rest of the gallery uses the script-free components of package bs instead (bs.Dialog, bs.DetailsDropdown, server-rendered toasts)."),
			),
		)},
		Main: elem.Bundle{
			demo("js-modal-demo", "Modal (bs.Modal)",
				para("Opened by Bootstrap's script through data-bs-toggle; compare with the script-free bs.Dialog on the dialogs page."),
				bs.ModalTrigger(jsModal, attr.Class("btn btn-primary"))(text.Text("Open the modal")),
			),
			demo("js-dropdown-demo", "Dropdown (bs.Dropdown)",
				para("Positioned by Popper, closed by a click elsewhere or Escape, with arrow keys between the items."),
				bs.Dropdown{
					Toggle: text.Text("Actions"),
					Items: []elem.Node{
						bs.DropdownItem(text.Href("/components"))(text.Text("Components")),
						bs.DropdownItem(text.Href("/dialogs"))(text.Text("Dialogs")),
						bs.DropdownDivider(),
						bs.DropdownItem(text.Href("/"))(text.Text("Gallery")),
					},
				},
			),
			demo("js-toast-demo", "Toast and dismissible alert",
				para("Bootstrap's close buttons (data-bs-dismiss) work with its script."),
				bs.Toast{
					Header: elem.Bundle{bi.Bell, text.Strong(attr.Class("ms-2 me-auto"))(text.Text("Bootstrap JS"))},
					Body:   text.Text("Close me with Bootstrap's own close button."),
					Shown:  true,
					Polite: true,
					Attrs:  attr.Class("mb-3"),
				},
				bs.AlertDismissible(attr.Class("alert-info"))(text.Text("A dismissible alert.")),
			),
		},
		Dialogs: []elem.Node{bs.Modal{
			ID:     jsModal,
			Title:  text.Text("A Bootstrap modal"),
			Body:   para("Bootstrap's script shows this modal, traps the focus and closes it on Escape."),
			Footer: bs.BtnSecondary(attr.KV("data-bs-dismiss", "modal"))(text.Text("Close")),
		}},
	}))
}
