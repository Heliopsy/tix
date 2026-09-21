package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// activityRoutes are the tenant-wide activity feed and the fragment its live
// update reloads.
func (h *handler) activityRoutes() []route {
	services := []string{
		"ListAudit", "GetActor", "GetTask", "ListProjects", "ListWorkflows", "GetTenant", "ListUsers",
	}
	return []route{
		get(RouteActivity, "activity.html", h.showActivity, services...),
		// The feed fragment renders through the same template set as the full
		// screen -- see showActivityFeed -- so it is bound to the same
		// template file here, even though the handler executes only one of
		// its named blocks rather than the whole page.
		get(RouteActivityFeed, "activity.html", h.showActivityFeed, services...),
	}
}

// activityGroup is one row the tenant-wide feed renders: a history group
// (history.go) plus the human name of the record it happened to, where it
// links, and the list it belongs to -- none of which the task-scoped history
// view needs, since its whole page is already about one task in one list.
// Accent is the zero value when the entry names no task, or names one whose
// project could not be resolved, and the row then carries no project marking
// at all rather than an empty badge.
type activityGroup struct {
	Group       historyGroup
	Subject     string
	SubjectHref string
	Accent      projectAccent
}

// activityView is what the activity screen, and its live-update fragment,
// render. Both are built by buildActivityView, so a row that arrives live is
// produced by the exact same grouping and wording as one that was already on
// the page at load.
type activityView struct {
	Groups     []activityGroup
	Names      actorNames
	Cursor     string
	NextCursor string
}

// showActivity renders recent tenant activity, newest first.
func (h *handler) showActivity(w http.ResponseWriter, r *http.Request) error {
	cursor := r.URL.Query().Get("cursor")
	data, err := h.buildActivityView(r, cursor)
	if err != nil {
		return err
	}
	return h.render(w, r, "activity.html", "Activity", data)
}

// showActivityFeed renders only the row list, newest first, with no cursor:
// the fragment a page with live.js loaded fetches after its WebSocket
// connection says something changed. It shares buildActivityView with the
// full screen and executes the very block that screen uses for its rows, so
// there is exactly one implementation of what a row looks like, on load and
// live alike.
func (h *handler) showActivityFeed(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildActivityView(r, "")
	if err != nil {
		return err
	}
	v := h.newView(r, "activity.html", "Activity", data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	return h.templates["activity.html"].ExecuteTemplate(w, "activity-rows", v)
}

// buildActivityView loads a page of tenant audit entries and folds them into
// the same grouped, sentence-rendered rows history.go builds for one task's
// history, generalised across every subject the entries name. See
// groupHistory and continuesChain for the grouping rule, which is otherwise
// unchanged: the only addition continuesChain needed is that two hops must
// share a subject before they may chain, since a tenant-wide stream, unlike
// one task's own trail, interleaves many subjects.
//
// The project every row carries is resolved from one ListProjects call for
// the whole page, not one lookup per row: a mixed feed can have dozens of
// rows naming a handful of lists, and this is the same map, built the same
// way, that tasks.go hands the task list for its own accent stripes.
func (h *handler) buildActivityView(r *http.Request, cursor string) (activityView, error) {
	filter := core.AuditFilter{Page: core.Page{
		Cursor: cursor, Sort: "seq", Direction: core.Descending, Limit: 50,
	}}
	entries, next, err := h.svc.ListAudit(r.Context(), filter)
	if err != nil {
		return activityView{}, err
	}
	projects, _, err := h.svc.ListProjects(r.Context(), core.ProjectFilter{
		IncludeArchived: true, Page: core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		return activityView{}, err
	}
	accents := projectAccents(projects)

	// entries arrive newest first; chaining reads left to right in time, so
	// it runs oldest first and the result is reversed back to newest first,
	// exactly as attachHistory does for one task's own trail.
	rows := make([]historyRow, len(entries))
	for i, entry := range entries {
		rows[len(entries)-1-i] = historyRow{Entry: entry, Changed: changedFields(entry)}
	}
	groups := groupHistory(rows)

	// Actor names are resolved once, up front, because a "session" or
	// "membership" entry's subject is itself an actor id (the person who
	// signed in, or who was added to the tenant) -- exactly the id this
	// same lookup already resolves for the row's own actor, so a second,
	// differently-shaped cache would only duplicate it.
	actors := make([]string, 0, len(rows)+len(groups))
	for _, row := range rows {
		actors = append(actors, row.Entry.ActorID)
		if row.Entry.SubjectType == "session" || row.Entry.SubjectType == "membership" {
			actors = append(actors, row.Entry.SubjectID)
		}
	}
	names := h.resolveActors(r, actors...)

	workflows := h.workflowNames(r, groups)
	tenant := h.tenantName(r, groups)
	users := h.userNames(r, groups)

	data := activityView{Groups: make([]activityGroup, len(groups)), Cursor: cursor, NextCursor: next, Names: names}
	for i, g := range groups {
		subject, href, projectID := h.subjectFor(r, g.Latest().Entry, accents, workflows, tenant, users, names)
		data.Groups[len(groups)-1-i] = activityGroup{
			Group: g, Subject: subject, SubjectHref: href, Accent: accents[projectID],
		}
	}
	return data, nil
}

// subjectKindNouns names the subject types the feed cannot look a specific
// record up for as cheaply as a task, a project or a workflow: naming one of
// these would take a query per row rather than one list call for the whole
// page, or a listing only a tenant admin may call (see workflowNames and
// tenantName for the two admin-gated lookups this screen does make, and why
// a failure there falls back here too rather than breaking the page for a
// viewer or member). Saying what kind of thing changed, without a name, is
// still honest; showing the bare identifier, or inventing a name, is not.
var subjectKindNouns = map[string]string{
	"comment":          "a comment",
	"artifact":         "an artifact",
	"api_token":        "an API token",
	"field_def":        "a custom field",
	"field":            "a custom field",
	"domain":           "a domain",
	"sync_source":      "a sync source",
	"webhook":          "a webhook",
	"webhook_delivery": "a webhook delivery",
	"tag":              "a tag",
	"session":          "a session",
	"membership":       "a membership",
}

// subjectFor names what an entry happened to, where it links, if anywhere,
// and the id of the project it belongs to, if any. A task is shown, and
// linked, by the reference people quote to each other, and its project id
// comes along for free since finding the task already required loading it.
// A project or workflow is named the same way, from the page's own batched
// lookup, and links to its own screen. A session or membership names the
// actor it belongs to, since that is literally what its subject id is.
// Everything else names its kind (subjectKindNouns) rather than an
// identifier nobody recognises or a name nobody wrote down for it.
func (h *handler) subjectFor(r *http.Request, entry core.AuditEntry, accents map[string]projectAccent, workflows map[string]core.Workflow, tenant string, users map[string]core.User, names actorNames) (label, href, projectID string) {
	switch entry.SubjectType {
	case "task":
		if entry.SubjectID == "" {
			break
		}
		task, err := h.svc.GetTask(r.Context(), core.TaskRef{ID: entry.SubjectID})
		if err == nil && task != nil && task.Ref != "" {
			return task.Ref, RouteTasks + "/" + task.Ref, task.ProjectID
		}
	case "project":
		if a, ok := accents[entry.SubjectID]; ok && a.Name != "" {
			return "the project " + a.Name, RouteProjects + "/" + a.Key, entry.SubjectID
		}
	case "workflow":
		if wf, ok := workflows[entry.SubjectID]; ok && wf.Name != "" {
			return "the workflow " + wf.Name, RouteWorkflows + "/" + wf.Key, ""
		}
	case "tenant":
		if tenant != "" {
			return "the tenant " + tenant, RouteTenant, ""
		}
	case "user":
		if u, ok := users[entry.SubjectID]; ok {
			if name := userLabel(u); name != "" {
				return "the user " + name, RouteUsers, ""
			}
		}
	case "session", "membership":
		if label := names.Label(entry.SubjectID); label != "" {
			return label, "", ""
		}
	}
	if noun, ok := subjectKindNouns[entry.SubjectType]; ok {
		return noun, "", ""
	}
	return shortID(entry.SubjectID), "", ""
}

// userLabel is what a user record is called on the feed: the name they
// gave, or their email when they gave none, never the account id.
func userLabel(u core.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Email
}

// workflowNames resolves every workflow name the page's rows need in one
// call, the same shape as accents resolves projects, but only when a row
// actually names one: ListWorkflows needs no elevated scope, so this always
// succeeds for anyone who could reach the feed at all, but there is no
// reason to make the call on a page with no workflow rows on it.
func (h *handler) workflowNames(r *http.Request, groups []historyGroup) map[string]core.Workflow {
	if !anySubject(groups, "workflow") {
		return nil
	}
	workflows, err := h.svc.ListWorkflows(r.Context())
	if err != nil {
		return nil
	}
	out := make(map[string]core.Workflow, len(workflows))
	for _, wf := range workflows {
		out[wf.ID] = wf
	}
	return out
}

// tenantName resolves the tenant's own display name for a "tenant" subject
// row, when the page has one. GetTenant requires the tenant-admin scope, so
// a viewer or member calling it fails; that failure is not a bug here, it is
// the same reason those roles get no "Tenant" link in the nav, so it is
// swallowed rather than breaking the whole feed, and the row falls back to
// subjectKindNouns instead.
func (h *handler) tenantName(r *http.Request, groups []historyGroup) string {
	if !anySubject(groups, "tenant") {
		return ""
	}
	tenant, err := h.svc.GetTenant(r.Context(), "")
	if err != nil || tenant == nil {
		return ""
	}
	return tenant.Name
}

// userNames resolves every user account a "user" subject row needs in one
// call. ListUsers, like GetTenant, requires the tenant-admin scope: the same
// reasoning in tenantName's comment applies here, and a lookup failure falls
// back the same way.
func (h *handler) userNames(r *http.Request, groups []historyGroup) map[string]core.User {
	if !anySubject(groups, "user") {
		return nil
	}
	users, _, err := h.svc.ListUsers(r.Context(), core.Page{Limit: core.MaxPageLimit})
	if err != nil {
		return nil
	}
	out := make(map[string]core.User, len(users))
	for _, u := range users {
		out[u.ID] = u
	}
	return out
}

// anySubject reports whether any group's latest hop names a subject of the
// given type.
func anySubject(groups []historyGroup, subjectType string) bool {
	for _, g := range groups {
		if g.Latest().Entry.SubjectType == subjectType {
			return true
		}
	}
	return false
}
