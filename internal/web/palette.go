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

// Routes the command palette (assets/palette.js) reads. Each answers JSON
// rather than a screen, which is why they are bound as files.
const (
	RoutePaletteTasks     = "/palette/tasks"
	RoutePaletteProjects  = "/palette/projects"
	RoutePaletteTemplates = "/palette/templates"
)

// PaletteTaskLimit is how many tasks one palette search answers with. The
// palette is for jumping to a task, not for reading a listing; the full list
// behind the same filter is one Enter away on Tasks.
const PaletteTaskLimit = 10

// paletteRoutes are the palette's reads. They add no capability: tasks are
// ListTasks under the filter language, plus GetTask for a typed ref,
// projects are ListProjects and templates are ListTemplates, the same calls
// GET /api/v1/tasks?q=, GET /api/v1/projects and GET /api/v1/templates make.
func (h *handler) paletteRoutes() []route {
	return []route{
		getFile(RoutePaletteTasks, h.paletteTasks, "ListTasks", "GetTask"),
		getFile(RoutePaletteProjects, h.paletteProjects, "ListProjects"),
		getFile(RoutePaletteTemplates, h.paletteTemplates, "ListTemplates"),
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

// paletteTemplate is one task template the palette offers a new task from.
// Title is the title shape a use gives, which the palette matches on too.
type paletteTemplate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Project    string `json:"project,omitempty"`
	TenantWide bool   `json:"tenant_wide,omitempty"`
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

// paletteTemplates answers every template the reader can read, which the
// palette offers as New task from each once something is typed. Using one
// posts the form the Use link on Templates opens, so it adds no new way of
// creating a task.
func (h *handler) paletteTemplates(w http.ResponseWriter, r *http.Request) error {
	items, err := h.allTemplates(r, "")
	if err != nil {
		return err
	}
	out := make([]paletteTemplate, 0, len(items))
	for _, t := range items {
		out = append(out, paletteTemplate{ID: t.ID, Name: t.Name, Title: t.Task.Title,
			Project: t.ProjectKey, TenantWide: t.TenantWide()})
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
	// Value is what a colour scheme entry asks the sidebar switch for.
	Value string `json:"value,omitempty"`
}

// paletteTaskEntry is one thing the palette can do to the task on screen,
// the page's own or the one open in the dialog. Target is the control it
// drives there, a selector in which {p} stands for the screen's id prefix
// and {ref} for the task's ref; assets/palette.js offers the entry only
// when that control is on screen, so a reader who may not change a task is
// offered nothing that would. Do is how it drives it: popover opens a menu,
// focus focuses a field (opening the fold or tab it sits in), click presses
// it, submit sends its form, copy copies its text.
type paletteTaskEntry struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Terms  string `json:"terms,omitempty"`
	Target string `json:"target"`
	Do     string `json:"do"`
}

// paletteNeedsKeys are the entry ids whose target is drawn only for a
// device with a keyboard and a hovering pointer: .needs-keys in app.css.
var paletteNeedsKeys = map[string]bool{
	"screen:" + RouteSettings + "#keyboard": true,
	"shortcuts":                             true,
}

// paletteData is what a page embeds for the palette.
type paletteData struct {
	Screens []paletteEntry     `json:"screens"`
	Actions []paletteEntry     `json:"actions"`
	Themes  []paletteEntry     `json:"themes"`
	Task    []paletteTaskEntry `json:"task"`
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
	{RouteActivity, "Activity", "", false, false},
	{RouteStats, "Statistics", "", false, false},
	{RouteWorkflows, "Workflows", "Admin", true, false},
	{RouteTemplates, "Templates", "Admin", true, false},
	{RouteTenant, "Tenant", "Admin", true, false},
	{RouteDomains, "Domains", "Admin", true, false},
	{RouteUsers, "Users", "Admin", true, false},
	{RouteTokens, "Tokens", "Admin", true, false},
	{RouteSSHKeys, "SSH keys", "Admin", true, false},
	{RouteStatus, "Status", "Admin", true, false},
	{RouteConnections, "Connections", "Admin", true, false},
	{RouteWebhooks, "Webhooks", "Admin", true, false},
	{RouteTransfer, "Import and export", "Admin", true, false},
	{RouteSync, "Sync", "Admin", true, false},
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

// paletteTaskActions are what the palette offers on a task screen, each
// one a control templates/task.html already draws there. A label holding
// %s gets the task's ref. Terms are extra words it is found by, in English
// like the filter language, and are not shown.
var paletteTaskActions = []paletteTaskEntry{
	{ID: "task-move", Label: "Change the status", Terms: "move transition status done", Target: "#{p}flow-{ref}", Do: "popover"},
	{ID: "task-priority", Label: "Change the priority", Terms: "priority urgent", Target: "#{p}prio-{ref}", Do: "popover"},
	{ID: "task-assign", Label: "Assign", Terms: "assignee owner", Target: "#{p}assignee", Do: "focus"},
	{ID: "task-due", Label: "Change the due date", Terms: "due date deadline", Target: "#{p}due", Do: "focus"},
	{ID: "task-tag", Label: "Add a tag", Terms: "tag label", Target: "#{p}tag", Do: "focus"},
	{ID: "task-comment", Label: "Add a comment", Terms: "comment reply", Target: "#{p}comment-body", Do: "focus"},
	{ID: "task-star", Label: "Star %s", Terms: "star favourite", Target: `.star[aria-pressed="false"]`, Do: "click"},
	{ID: "task-unstar", Label: "Unstar %s", Terms: "star unstar favourite", Target: `.star[aria-pressed="true"]`, Do: "click"},
	{ID: "task-snooze", Label: "Snooze…", Terms: "snooze later hide", Target: "#{p}snooze-{ref}", Do: "popover"},
	{ID: "task-unsnooze", Label: "Unsnooze", Terms: "snooze wake", Target: "form.snoozeform", Do: "submit"},
	{ID: "task-copy-ref", Label: "Copy reference", Terms: "copy ref reference clipboard", Target: "span.ref", Do: "copy"},
	{ID: "task-handoff", Label: "Hand to an agent", Terms: "agent ai claude codex prompt mcp delegate handoff", Target: `[data-dlg-open="{p}handoff"]`, Do: "click"},
	{ID: "task-rename", Label: "Rename", Terms: "title edit", Target: "#{p}title", Do: "focus"},
	{ID: "task-describe", Label: "Write the description", Terms: "description body edit", Target: "#{p}body", Do: "focus"},
	{ID: "task-copy-to", Label: "Copy to…", Terms: "copy duplicate project", Target: "a.copyto", Do: "click"},
	{ID: "task-move-to", Label: "Move to…", Terms: "move project", Target: "a.moveto", Do: "click"},
	{ID: "task-save-template", Label: "Save as template", Terms: "template", Target: "a.savetpl", Do: "click"},
	{ID: "task-reclaim", Label: "Reclaim the lease…", Terms: "reclaim lease claim release", Target: "#{p}reclaim-{ref}", Do: "popover"},
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
	for _, a := range paletteTaskActions {
		out = append(out, a.Label)
	}
	for _, t := range Themes {
		out = append(out, ThemeLabel(t))
	}
	return out
}

// paletteDataFor builds the palette's fixed entries for one page.
func paletteDataFor(lang string, advanced, admin bool) paletteData {
	data := paletteData{Screens: []paletteEntry{}, Actions: []paletteEntry{}, Themes: []paletteEntry{}, Task: []paletteTaskEntry{}}
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
	for _, t := range Themes {
		data.Themes = append(data.Themes, paletteEntry{ID: "theme:" + t, Label: i18n.T(lang, ThemeLabel(t)), Value: t})
	}
	for _, a := range paletteTaskActions {
		a.Label = i18n.T(lang, a.Label)
		data.Task = append(data.Task, a)
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
