// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/i18n"
)

// Routes the command palette (assets/palette.js) reads. Both answer JSON
// rather than a screen, which is why they are bound as files.
const (
	RoutePaletteTasks    = "/palette/tasks"
	RoutePaletteProjects = "/palette/projects"
)

// PaletteTaskLimit is how many tasks one palette search answers with. The
// palette is for jumping to a task, not for reading a listing; the full list
// behind the same filter is one Enter away on Tasks.
const PaletteTaskLimit = 10

// paletteRoutes are the palette's two reads. They add no capability: tasks
// are ListTasks under the filter language, plus GetTask for a typed ref, and
// projects are ListProjects, the same calls GET /api/v1/tasks?q= and GET
// /api/v1/projects make.
func (h *handler) paletteRoutes() []route {
	return []route{
		getFile(RoutePaletteTasks, h.paletteTasks, "ListTasks", "GetTask"),
		getFile(RoutePaletteProjects, h.paletteProjects, "ListProjects"),
	}
}

// paletteTask is one task the palette offers.
type paletteTask struct {
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Href   string `json:"href"`
}

// paletteProject is one project the palette offers.
type paletteProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Href string `json:"href"`
}

// paletteFilter is the filter one palette search runs: the query read as a
// filter expression, so "is:starred deploy" narrows the way the filter bar
// does, or as plain words when it does not parse, since a half-typed term is
// what a search box holds most of the time.
func paletteFilter(q string) core.TaskFilter {
	f, err := ParseFilter(q)
	if err != nil {
		f = core.TaskFilter{Query: q}
	}
	f.Page = core.Page{Limit: PaletteTaskLimit, Sort: core.SortUpdatedAt, Direction: core.Descending}
	return f
}

// paletteTasks answers a palette search: the task a typed ref names first,
// then the most recently changed tasks the query matches, PaletteTaskLimit
// at most. An empty query searches nothing.
func (h *handler) paletteTasks(w http.ResponseWriter, r *http.Request) error {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []paletteTask{}
	if q == "" {
		return writePaletteJSON(w, out)
	}
	seen := map[string]bool{}
	add := func(t core.Task) {
		if seen[t.Ref] || len(out) >= PaletteTaskLimit {
			return
		}
		seen[t.Ref] = true
		out = append(out, paletteTask{Ref: t.Ref, Title: t.Title, Status: t.Status, Href: RouteTasks + "/" + t.Ref})
	}
	if ref, err := core.ParseTaskRef(q); err == nil && ref.ProjectKey != "" {
		if t, err := h.svc.GetTask(r.Context(), ref); err == nil && t != nil {
			add(*t)
		}
	}
	page, err := h.svc.ListTasks(r.Context(), paletteFilter(q))
	if err != nil {
		return err
	}
	for _, t := range page.Tasks {
		add(t)
	}
	return writePaletteJSON(w, out)
}

// paletteProjects answers every active project, which the palette filters
// as the reader types and offers a "New task in" action for.
func (h *handler) paletteProjects(w http.ResponseWriter, r *http.Request) error {
	projects, _, err := h.allProjects(r, false)
	if err != nil {
		return err
	}
	out := make([]paletteProject, 0, len(projects))
	for _, p := range projects {
		out = append(out, paletteProject{Key: p.Key, Name: p.Name, Href: RouteProjects + "/" + p.Key})
	}
	return writePaletteJSON(w, out)
}

// writePaletteJSON writes one palette answer. It is private to the reader
// who asked and goes stale with the next change, so nothing may keep it.
func writePaletteJSON(w http.ResponseWriter, v any) error {
	hdr := w.Header()
	hdr.Set("Content-Type", "application/json; charset=utf-8")
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Cache-Control", "no-store")
	return json.NewEncoder(w).Encode(v)
}

// paletteEntry is one screen or action the palette offers without asking
// the server: a label in the page's language, a group word beside it, and
// where it goes or what it does.
type paletteEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
	Href  string `json:"href,omitempty"`
	// Keys marks an entry pointing at something only a keyboard uses, which
	// a touch-only device does not draw and assets/palette.js leaves out.
	Keys bool `json:"keys,omitempty"`
}

// paletteNeedsKeys are the entry ids whose target is drawn only for a
// device with a keyboard and a hovering pointer: .needs-keys in app.css.
var paletteNeedsKeys = map[string]bool{
	"screen:" + RouteSettings + "#keyboard": true,
	"shortcuts":                             true,
}

// paletteData is what a page embeds for the palette.
type paletteData struct {
	Screens []paletteEntry `json:"screens"`
	Actions []paletteEntry `json:"actions"`
}

// paletteScreen is one destination: every sidebar entry, and every section
// of Settings. Advanced marks the entries the sidebar shows only with
// Advanced screens on, so the palette offers what the sidebar offers; Admin
// marks the ones drawn only for a tenant administrator.
type paletteScreen struct {
	Href     string
	Label    string
	Hint     string
	Advanced bool
	Admin    bool
}

// paletteScreens follow the sidebar's order (templates/partials.html "nav"
// and layout.html), then the sections of Settings.
var paletteScreens = []paletteScreen{
	{RouteTasks, "Tasks", "", false, false},
	{RouteProjects, "Projects", "", false, false},
	{RouteTemplates, "Templates", "", false, false},
	{RouteActivity, "Activity", "", false, false},
	{RouteStats, "Statistics", "", false, false},
	{RouteWorkflows, "Workflows", "Manage", true, false},
	{RouteTenant, "Tenant", "Manage", true, false},
	{RouteDomains, "Domains", "Manage", true, false},
	{RouteUsers, "Users", "Manage", true, false},
	{RouteTokens, "Tokens", "Manage", true, false},
	{RouteSSHKeys, "SSH keys", "Manage", true, false},
	{RouteStatus, "Status", "Manage", true, false},
	{RouteConnections, "Connections", "Manage", true, false},
	{RouteWebhooks, "Webhooks", "Manage", true, false},
	{RouteTransfer, "Import and export", "Manage", true, false},
	{RouteSync, "Sync", "Manage", true, false},
	{RouteSettings, "Settings", "", false, false},
	{RouteSettings + "#appearance", "Appearance", "Settings", false, false},
	{RouteSettings + "#listings", "Listings", "Settings", false, false},
	{RouteSettings + "#keyboard", "Keyboard", "Settings", false, false},
	{RouteSettings + "#dates", "Dates and times", "Settings", false, false},
	{RouteSettings + "#second-factor", "Second factor", "Settings", false, false},
	{RouteSettings + "#sessions", "Sessions", "Settings", false, false},
	{Href: RouteSettings + "#database", Label: "Database", Hint: "Settings", Admin: true},
}

// paletteActions are what the palette can do besides going somewhere. The
// id is what assets/palette.js dispatches on; each one drives a control the
// interface already has, so the palette adds no way of changing anything.
var paletteActions = []paletteEntry{
	{ID: "new-task", Label: "New task"},
	{ID: "new-project", Label: "New project"},
	{ID: "filter", Label: "Open the filter"},
	{ID: "view-list", Label: "Show tasks as a list"},
	{ID: "view-board", Label: "Show tasks as a board"},
	{ID: "view-lanes", Label: "Show tasks in lanes"},
	{ID: "theme", Label: "Switch the colour scheme"},
	{ID: "shortcuts", Label: "Keyboard shortcuts"},
	{ID: "signout", Label: "Sign out"},
}

// paletteLabels are every fixed word paletteJSON hands a page, for the
// catalog's completeness test.
func paletteLabels() []string {
	var out []string
	for _, s := range paletteScreens {
		out = append(out, s.Label)
		if s.Hint != "" {
			out = append(out, s.Hint)
		}
	}
	for _, a := range paletteActions {
		out = append(out, a.Label)
	}
	return out
}

// paletteDataFor builds the palette's fixed entries for one page.
func paletteDataFor(lang string, advanced, admin bool) paletteData {
	data := paletteData{Screens: []paletteEntry{}, Actions: []paletteEntry{}}
	for _, s := range paletteScreens {
		if (s.Advanced && !advanced) || (s.Admin && !admin) {
			continue
		}
		id := "screen:" + s.Href
		e := paletteEntry{ID: id, Label: i18n.T(lang, s.Label), Href: s.Href, Keys: paletteNeedsKeys[id]}
		if s.Hint != "" {
			e.Hint = i18n.T(lang, s.Hint)
		}
		data.Screens = append(data.Screens, e)
	}
	for _, a := range paletteActions {
		data.Actions = append(data.Actions, paletteEntry{ID: a.ID, Label: i18n.T(lang, a.Label), Keys: paletteNeedsKeys[a.ID]})
	}
	return data
}

// paletteJSON renders the palette's fixed entries as the JSON a page embeds
// in a <script type="application/json"> element, so they draw the moment
// the palette opens with no request behind them.
func paletteJSON(lang string, advanced, admin bool) template.JS {
	raw, err := json.Marshal(paletteDataFor(lang, advanced, admin))
	if err != nil {
		return template.JS("{}") // #nosec G203 -- fixed fallback, not caller input
	}
	// #nosec G203 -- json.Marshal escapes '<', '>' and '&', and every value
	// here is a fixed label, route or catalog translation.
	return template.JS(raw)
}
