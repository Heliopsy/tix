// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// RouteLiveEvents is the Live events screen: the tenant's raw event stream,
// followed in the browser over the WebSocket, as `tix watch` prints it.
const RouteLiveEvents = "/events"

// LiveEventsCap is how many events the screen keeps, and how many it holds
// while paused. Older ones are dropped, as the app's Live events screen does.
const LiveEventsCap = 500

// Why a filter cannot be followed, each with the value at fault in its %s.
const (
	liveEventsNoProject = "No project has the key %s."
	liveEventsBadSince  = "The sequence number %s is not a whole number above zero."
)

// liveEventsRoutes serve the screen. The events themselves arrive over the
// WebSocket (Subscribe, exempted from the browser), so the only call the page
// makes is the project listing behind its picker.
func (h *handler) liveEventsRoutes() []route {
	return []route{
		get(RouteLiveEvents, "events.html", h.showLiveEvents, "ListProjects"),
	}
}

// liveEventsQuery is what the screen follows, read from the address so a
// narrowed stream is a link somebody can send.
type liveEventsQuery struct {
	Types   string
	Project string
	Actor   string
	Since   string
}

// liveEventsQueryFrom reads the filter out of a request's query string.
func liveEventsQueryFrom(r *http.Request) liveEventsQuery {
	q := r.URL.Query()
	return liveEventsQuery{
		Types:   strings.TrimSpace(q.Get("type")),
		Project: strings.TrimSpace(q.Get("project")),
		Actor:   strings.TrimSpace(q.Get("actor")),
		Since:   strings.TrimSpace(q.Get("since")),
	}
}

// Active reports whether anything narrows or replays the stream.
func (q liveEventsQuery) Active() bool {
	return q.Types != "" || q.Project != "" || q.Actor != "" || q.Since != ""
}

// Href is this filter as a link to the screen, dropping the empty fields.
func (q liveEventsQuery) Href() string {
	values := url.Values{}
	for _, p := range [][2]string{{"type", q.Types}, {"project", q.Project}, {"actor", q.Actor}, {"since", q.Since}} {
		if p[1] != "" {
			values.Set(p[0], p[1])
		}
	}
	if len(values) == 0 {
		return RouteLiveEvents
	}
	return RouteLiveEvents + "?" + values.Encode()
}

// Without is this filter with one field cleared, what each chip links to.
func (q liveEventsQuery) Without(name string) string {
	switch name {
	case "type":
		q.Types = ""
	case "project":
		q.Project = ""
	case "actor":
		q.Actor = ""
	case "since":
		q.Since = ""
	}
	return q.Href()
}

// liveEventsView is what events.html is executed against.
type liveEventsView struct {
	Query    liveEventsQuery
	Projects []core.Project
	// ProjectsWhole is false when the picker could not list every project.
	ProjectsWhole bool
	// Problem is why the filter cannot be followed, which stops the page
	// from subscribing and offers the way back to the whole stream.
	Problem string
	// ProblemArg fills Problem's one %s.
	ProblemArg string
	// Filter is the subscription's server half as JSON, empty for none.
	Filter string
	// Actors is the actor filter as a JSON array, applied in the browser.
	Actors string
	// Since is the sequence number to replay after, zero for from now.
	Since int64
	// Keys names each project's key by its id, as JSON, for the rows.
	Keys string
	Cap  int
}

// showLiveEvents renders the screen. The rows are drawn by
// assets/events.js as events arrive; the server draws the filter, the
// connection state and the empty list.
func (h *handler) showLiveEvents(w http.ResponseWriter, r *http.Request) error {
	query := liveEventsQueryFrom(r)
	projects, whole, err := h.allProjects(r, true)
	if err != nil {
		return err
	}
	data := liveEventsView{Query: query, ProjectsWhole: whole, Cap: LiveEventsCap}
	keys := map[string]string{}
	filter := core.EventFilter{}
	for _, p := range projects {
		keys[p.ID] = p.Key
		if !p.Archived() {
			data.Projects = append(data.Projects, p)
		}
		if query.Project != "" && strings.EqualFold(p.Key, query.Project) {
			filter.ProjectIDs = []string{p.ID}
		}
	}
	for _, t := range splitList(query.Types) {
		filter.Types = append(filter.Types, core.EventType(t))
	}
	switch {
	case query.Project != "" && len(filter.ProjectIDs) == 0:
		data.Problem, data.ProblemArg = liveEventsNoProject, query.Project
	case query.Since != "":
		n, err := strconv.ParseInt(query.Since, 10, 64)
		if err != nil || n < 1 {
			data.Problem, data.ProblemArg = liveEventsBadSince, query.Since
		}
		data.Since = n
	}
	if len(filter.Types) > 0 || len(filter.ProjectIDs) > 0 {
		data.Filter = jsonText(filter)
	}
	data.Actors = jsonText(append([]string{}, splitList(query.Actor)...))
	data.Keys = jsonText(keys)
	return h.render(w, r, "events.html", "Live events", data)
}

// jsonText marshals a value the page carries in an attribute. Every value
// here is a slice, a map of strings or a filter, which always marshal.
func jsonText(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}
