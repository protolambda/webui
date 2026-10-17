package app

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/ba"
	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
)

// Ids of the htmx page that requests target.
const (
	taskListID      = "task-list"
	taskFilterID    = "task-filter"
	taskCountID     = "task-count"
	addTaskID       = "add-task"
	failFormID      = "fail-form"
	searchResultsID = "search-results"
)

// errDemoFailure is the cause of the request that fails on purpose.
var errDemoFailure = errors.New("gallery: the demo request fails on purpose")

func (a *App) htmx(w http.ResponseWriter, r *http.Request) error {
	token, err := a.formToken(w, r)
	if err != nil {
		return err
	}
	return web.Render(w, r, http.StatusOK, a.htmxPage(r, token, filterAll, ""))
}

// htmxPage renders the htmx page with the task list filtered by f and the
// search for q.
func (a *App) htmxPage(r *http.Request, token string, f filter, q string) elem.Node {
	sidebarItems := []page.NavItem{
		{Label: "Swaps", Href: "#swaps"}, {Label: "Errors", Href: "#errors"},
		{Label: "Search", Href: "#search"}, {Label: "Polling", Href: "#polling"},
	}
	return a.page(r, view{
		Title:   "htmx",
		Icon:    bi.ArrowRepeat,
		Actions: []elem.Node{a.taskCount(false)},
		Sidebar: page.Sidebar{Label: "htmx demos", Sections: []page.SideSection{{Title: "On this page", Items: sidebarItems}}},
		Main: elem.Bundle{
			para("Every control here also works without script: links and forms fall back to full pages, which the same handlers render (web.Choose)."),
			demo("swaps", "Fragment and out-of-band swaps",
				para("The filter links swap the task list and push the URL. Adding or deleting a task swaps the list too, and updates the count in the header, the filter, the form and the alerts out of band."),
				taskFilter(f, false),
				a.taskList(token, f),
				addTaskForm(token, false),
			),
			demo("errors", "Error answers",
				para("This request fails on purpose. The server answers htmx with an alert and HX-Retarget, so the message shows in the alerts region at the top, with the real status (409)."),
				failForm(token, false),
			),
			demo("search", "Search as you type",
				form.Form(form.Method(form.MethodGet), form.Action("/htmx/search"), aria.Role("search"), attr.Class("mb-3"))(
					bs.Field{
						ID: "search-q", Name: "q", Type: input.Search, Label: text.Text("Search tasks"), Value: q,
						Autocomplete: "off",
						Attrs: attr.Bundle{
							input.Maxlength(strconv.Itoa(maxQuery)),
							hx2.Get("/htmx/search"),
							hx2.Trigger("input changed delay:300ms, search"),
							hx2.Target("#" + searchResultsID),
						},
						After: []elem.Node{submit(bs.VariantSecondary, "Search")},
					},
				),
				div.Div(attr.ID(searchResultsID), aria.Live("polite"))(a.searchResults(q)),
			),
			demo("polling", "Polling",
				para("This line asks the server every five seconds. Polls do not count as use of a session, so an open page does not keep a login alive."),
				text.P(hx2.Get("/htmx/poll"), hx2.Trigger("every 5s"))(a.uptime()),
			),
		},
	})
}

// taskCount renders the count badge of the page header; oob renders it for
// an out-of-band swap.
func (a *App) taskCount(oob bool) elem.Node {
	open, all := a.tasks.counts()
	attrs := []attr.Node{attr.ID(taskCountID)}
	if oob {
		attrs = append(attrs, hx2.SwapOOB("true"))
	}
	return bs.BadgeSecondary(attrs...)(text.Text(fmt.Sprintf("%d open of %d", open, all)))
}

// taskFilter renders the filter links of the task list; oob renders it for
// an out-of-band swap, to mark the current filter.
func taskFilter(current filter, oob bool) elem.Node {
	items := make([]elem.Node, 0, len(filters))
	for _, f := range filters {
		href := "/htmx/tasks?filter=" + string(f)
		attrs := []attr.Node{
			text.Href(href), hx2.Get(href), hx2.Target("#" + taskListID), hx2.Swap(hx2.SwapOuterHTML), hx2.PushURL("true"),
		}
		label := text.Text(capitalize(string(f)))
		if f == current {
			items = append(items, bs.NavItem()(bs.NavLinkActive(attrs...)(label)))
		} else {
			items = append(items, bs.NavItem()(bs.NavLink(attrs...)(label)))
		}
	}
	attrs := []attr.Node{attr.ID(taskFilterID), aria.Label("Task filter"), attr.Class("mb-2")}
	if oob {
		attrs = append(attrs, hx2.SwapOOB("true"))
	}
	return section.Nav(attrs...)(bs.NavPills()(items...))
}

// taskList renders the task list, the target of the swaps: each task has a
// delete form, which asks for confirmation with htmx.
func (a *App) taskList(token string, f filter) elem.Node {
	del := func(t task) elem.Node {
		action := "/htmx/tasks/" + strconv.Itoa(t.ID) + "/delete"
		return postForm(action, token,
			hx2.Post(action), hx2.Confirm("Delete this task?"),
			hx2.Target("#"+taskListID), hx2.Swap(hx2.SwapOuterHTML),
		)(
			bs.BtnOutlineDanger(button.Type(button.TypeSubmit), ba.BtnSM(), aria.Label("Delete "+t.Title))(bi.Trash),
		)
	}
	caption := "Tasks: " + string(f)
	return div.Div(attr.ID(taskListID))(taskTable(caption, a.tasks.list(f), del))
}

// addTaskForm renders the form that adds a task; oob renders it empty for
// an out-of-band swap after an added task.
func addTaskForm(token string, oob bool) elem.Node {
	attrs := []attr.Node{
		attr.ID(addTaskID), form.Novalidate(),
		hx2.Post("/htmx/tasks"), hx2.Target("#" + taskListID), hx2.Swap(hx2.SwapOuterHTML),
	}
	if oob {
		attrs = append(attrs, hx2.SwapOOB("true"))
	}
	return postForm("/htmx/tasks", token, attrs...)(
		bs.Field{
			ID: "new-task", Name: "title", Label: text.Text("New task"), Required: true, Attrs: input.Maxlength("80"),
			After: []elem.Node{submit(bs.VariantPrimary, "Add")},
		},
	)
}

// failForm renders the form of the request that fails on purpose; oob
// renders it for an out-of-band swap, with a renewed token.
func failForm(token string, oob bool) elem.Node {
	attrs := []attr.Node{attr.ID(failFormID), hx2.Post("/htmx/fail")}
	if oob {
		attrs = append(attrs, hx2.SwapOOB("true"))
	}
	return postForm("/htmx/fail", token, attrs...)(
		bs.BtnOutlineWarning(button.Type(button.TypeSubmit))(bi.ExclamationTriangle, text.Text(" Send a failing request")),
	)
}

// htmxTasks answers the filter links: the list for htmx, with the filter
// marked out of band; the page for others. When the token for the list's
// delete forms renewed the pre-authentication cookie, the fragment also
// renews the page's other forms out of band, as the new cookie refuses
// their tokens.
func (a *App) htmxTasks(w http.ResponseWriter, r *http.Request) error {
	token, renewed, err := a.renewedToken(w, r)
	if err != nil {
		return err
	}
	f := parseFilter(r.URL.Query().Get("filter"))
	fragment := elem.Bundle{a.taskList(token, f), taskFilter(f, true)}
	if renewed {
		fragment = append(fragment, addTaskForm(token, true), failForm(token, true))
	}
	return web.Choose(w, r, http.StatusOK, fragment, a.htmxPage(r, token, f, ""))
}

// changedList is the htmx answer to an added or deleted task: the list,
// and out of band the filter, the count, a fresh form and a notice.
func (a *App) changedList(token string, notice elem.Node) elem.Node {
	return elem.Bundle{
		a.taskList(token, filterAll),
		taskFilter(filterAll, true),
		a.taskCount(true),
		addTaskForm(token, true),
		page.AlertsOOB(notice),
	}
}

func (a *App) htmxAddTask(w http.ResponseWriter, r *http.Request) error {
	f, err := a.readForm(w, r)
	if err != nil {
		return err
	}
	defer f.Zero()
	title, ok := cleanLine(f.String("title"), maxTitle)
	if !ok {
		return web.StatusError(http.StatusUnprocessableEntity, "Give the task a title of 1 to 80 characters, on one line.", nil)
	}
	if _, err := a.tasks.add(task{Title: title, Priority: priorityNormal}); err != nil {
		return web.Conflict("The task list is full. Delete a task first.", err)
	}
	if !web.IsHTMX(r) {
		notices.Redirect(w, r, "/htmx", "task-added")
		return nil
	}
	return web.Render(w, r, http.StatusOK, a.changedList(f.String(session.CSRFField), page.Notice("", text.Text("Task added."))))
}

func (a *App) htmxDeleteTask(w http.ResponseWriter, r *http.Request) error {
	f, err := a.readForm(w, r)
	if err != nil {
		return err
	}
	defer f.Zero()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 || !a.tasks.remove(id) {
		return web.NotFound("There is no such task. It may have been deleted already.", nil)
	}
	if !web.IsHTMX(r) {
		notices.Redirect(w, r, "/htmx", "task-deleted")
		return nil
	}
	return web.Render(w, r, http.StatusOK, a.changedList(f.String(session.CSRFField), page.Notice(bs.VariantInfo, text.Text("Task deleted."))))
}

// htmxFail is the request that fails on purpose, after the CSRF check.
func (a *App) htmxFail(w http.ResponseWriter, r *http.Request) error {
	f, err := a.readForm(w, r)
	if err != nil {
		return err
	}
	f.Zero()
	return web.Conflict("This request failed on purpose: the server answered htmx with this alert, retargeted into the alerts region.", errDemoFailure)
}

func (a *App) htmxSearch(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query().Get("q")
	if len(q) > maxQuery*utf8.UTFMax {
		return web.BadRequest("The search is too long.", nil)
	}
	if strings.TrimSpace(q) == "" {
		q = ""
	} else if clean, ok := cleanLine(q, maxQuery); ok {
		q = clean
	} else {
		return web.BadRequest("Search for 1 to 100 characters on one line.", nil)
	}
	return a.choosePage(w, r, a.searchResults(q), func(token string) elem.Node {
		return a.htmxPage(r, token, filterAll, q)
	})
}

// searchResults renders the tasks that match q.
func (a *App) searchResults(q string) elem.Node {
	if q == "" {
		return para("Type to search the tasks.", attr.Class("text-body-secondary"))
	}
	found := a.tasks.search(q)
	if len(found) == 0 {
		return para("No task matches.", attr.Class("text-body-secondary"))
	}
	items := make([]elem.Node, 0, len(found))
	for _, t := range found {
		items = append(items, bs.ListGroupItem()(text.Text(t.Title)))
	}
	return elem.Bundle{
		text.P(attr.Class("small text-body-secondary"))(text.Text(fmt.Sprintf("%d found:", len(found)))),
		bs.ListGroup()(items...),
	}
}

func (a *App) htmxPoll(w http.ResponseWriter, r *http.Request) error {
	return a.choosePage(w, r, a.uptime(), func(token string) elem.Node {
		return a.htmxPage(r, token, filterAll, "")
	})
}

// uptime renders how long the gallery has been running.
func (a *App) uptime() elem.Node {
	d := a.now().Sub(a.started).Truncate(time.Second)
	return elem.Bundle{bi.Clock, text.Text(" The gallery has been running for " + d.String() + ".")}
}
