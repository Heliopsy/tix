// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// liveScreen is what a screen hands assets/live.js so it can keep itself
// current: where to re-fetch its own fragment, which events to ask the stream
// for, and which regions of the page the fragment replaces. The activity feed
// established the shape (activity.go, templates/activity.html); this is the
// same arrangement for a screen whose update touches more than one place.
//
// Href carries the screen's state, because the server wrote it from the
// request that drew the page. A refresh therefore re-renders the listing the
// reader narrowed, sorted and paged to, rather than the first page of
// everything. The preferences that are cookies rather than query parameters
// -- the list/board switch, the column picker, the project visibility choice
// -- ride along on the fetch.
type liveScreen struct {
	Events  string
	Href    string
	Filter  string
	Subject string
	Regions string
	Window  int
}

// How long a screen coalesces events before re-rendering, in milliseconds.
// The first event is drawn at once -- the trigger is leading-edge, see
// decide.js coalescer -- so this is only how long the screen then waits for
// the rest of a burst.
//
// A listing waits longer because a burst there is routine: a multi-hop
// transition writes a hop at a time, an agent sweep touches a page of rows,
// and re-rendering fifty rows for each of them is work nobody sees. One
// task's own screen waits half as long, because on it a burst is a handful
// of hops at most and the wait is the whole of the delay a reader perceives
// between a change and the pill changing.
const (
	liveListWindow   = 300
	liveDetailWindow = 150
)

// liveTaskTypes are the events that can change anything the task list or a
// board draws. Narrowing the subscription is what keeps an unrelated
// tenant-wide change -- a webhook delivery, a sign-in, a token -- from
// rebuilding a screen it cannot appear on.
var liveTaskTypes = []core.EventType{
	"task.*", core.EventLabelAdded, core.EventDependencyAdded,
}

// liveDetailTypes add the records one task's own screen draws beside it.
var liveDetailTypes = append([]core.EventType{
	core.EventCommentAdded, core.EventArtifactAdded,
}, liveTaskTypes...)

// liveFilter renders a subscription filter the way the event stream's
// subscribe message takes it. A filter that cannot be marshalled is no
// filter: the screen then subscribes to everything its credential can see,
// which is stale-free but noisier, rather than failing to connect at all.
func liveFilter(projectIDs []string, types []core.EventType) string {
	raw, err := json.Marshal(core.EventFilter{ProjectIDs: projectIDs, Types: types})
	if err != nil {
		return ""
	}
	return string(raw)
}

// fragmentHref is a fragment route carrying this request's own query string,
// minus the flash a redirect left behind, so the fragment answers the same
// question the page did.
func fragmentHref(r *http.Request, route string) string {
	query := r.URL.Query()
	query.Del("flash")
	if len(query) == 0 {
		return route
	}
	return route + "?" + query.Encode()
}

// screenHref is where a row action returns to: this request's own query
// string under the screen's own route.
//
// The route is named rather than taken from the request, because the request
// may have arrived at the live fragment instead. A row rendered by a refresh
// has to carry the same "where to come back to" as one rendered by the page;
// without this, ticking a row that arrived live sent the reader to the
// fragment route and they got a listing with no page around it.
//
// The flash goes. It is a message about the action just taken, not part of
// the view, and carrying it into the return URL made every tick append
// another: three clicks left the reader on "?flash=...&flash=...&flash=..."
// and the string grew for as long as they kept working down the list.
func screenHref(r *http.Request, route string) string {
	u := *r.URL
	u.Path = route
	query := u.Query()
	query.Del("flash")
	u.RawQuery = query.Encode()
	return u.RequestURI()
}

// taskLiveRoute is the fragment route for one task, with its reference in
// place of the wildcard.
func taskLiveRoute(ref string) string {
	return strings.Replace(RouteTaskLive, "{ref}", url.PathEscape(ref), 1)
}

// boardLiveRoute is the fragment route for one project's board.
func boardLiveRoute(key string) string {
	return strings.Replace(RouteBoardLive, "{key}", url.PathEscape(key), 1)
}

// liveTasks is the binding the task screen renders, for either drawing: the
// list and the board are one selection of tasks, so they refresh as one.
func (h *handler) liveTasks(r *http.Request) liveScreen {
	return liveScreen{
		Events:  h.eventsPath,
		Href:    fragmentHref(r, RouteTasksLive),
		Filter:  liveFilter(nil, liveTaskTypes),
		Regions: "tasks-summary tasks-listing",
		Window:  liveListWindow,
	}
}

// liveBoard is the binding one project's board renders. The board shows that
// project and nothing else, so the subscription says so and an event from
// another project never reaches it.
func (h *handler) liveBoard(project core.Project) liveScreen {
	return liveScreen{
		Events:  h.eventsPath,
		Href:    boardLiveRoute(project.Key),
		Filter:  liveFilter([]string{project.ID}, liveTaskTypes),
		Regions: "project-board",
		Window:  liveListWindow,
	}
}

// liveTask is the binding one task's screen renders. The stream has no filter
// for a single record, so the subscription narrows to the task's project and
// the screen narrows the rest of the way itself, on the subject the event
// names (decide.js wantsEvent).
func (h *handler) liveTask(task core.Task) liveScreen {
	return liveScreen{
		Events:  h.eventsPath,
		Href:    taskLiveRoute(task.Ref),
		Filter:  liveFilter([]string{task.ProjectID}, liveDetailTypes),
		Subject: task.ID,
		Regions: "task-pills task-comments task-history",
		Window:  liveDetailWindow,
	}
}

// renderFragment writes one named block of a screen's template set, executed
// against the same view the whole page is. It is how a live update stays
// honest: the markup a refresh writes is produced by the very block the page
// load used, so there is no second copy of a row, a card or a status pill to
// keep in step.
func (h *handler) renderFragment(w http.ResponseWriter, r *http.Request, name, block, title string, data any) error {
	tmpl, ok := h.templatesFor(r)[name]
	if !ok {
		return core.Internal("no template named %q", name)
	}
	v := h.newView(r, name, title, data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	return tmpl.ExecuteTemplate(w, block, v)
}
