// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

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
//
// ActorHref is where the row's actor name links: the same feed filtered to
// that actor. An actor is the one thing every row has, and following one was
// the question this screen could not answer at all.
type activityGroup struct {
	Group       historyGroup
	Subject     string
	SubjectHref string
	ActorHref   string
	Accent      projectAccent
}

// activityQuery is the filter the feed is showing, read from the URL so the
// address bar addresses the screen's state and a filtered feed can be
// bookmarked, shared and reloaded.
type activityQuery struct {
	Text   string
	Actor  string
	Kind   string
	Source string
}

// activityQueryFrom reads the filter out of a request's query string.
func activityQueryFrom(r *http.Request) activityQuery {
	q := r.URL.Query()
	return activityQuery{
		Text:   strings.TrimSpace(q.Get("q")),
		Actor:  strings.TrimSpace(q.Get("actor")),
		Kind:   strings.TrimSpace(q.Get("kind")),
		Source: strings.TrimSpace(q.Get("source")),
	}
}

// Active reports whether anything is being filtered out.
func (q activityQuery) Active() bool {
	return q.Text != "" || q.Actor != "" || q.Kind != "" || q.Source != ""
}

// values renders the filter as query parameters, dropping the empty ones so a
// shared link carries only what was actually chosen.
func (q activityQuery) values() url.Values {
	out := url.Values{}
	for name, value := range map[string]string{
		"q": q.Text, "actor": q.Actor, "kind": q.Kind, "source": q.Source,
	} {
		if value != "" {
			out.Set(name, value)
		}
	}
	return out
}

// Href is this filter as a link to the feed, optionally moved to a cursor.
func (q activityQuery) Href(cursor string) string {
	values := q.values()
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	if len(values) == 0 {
		return RouteActivity
	}
	return RouteActivity + "?" + values.Encode()
}

// FeedHref is this filter as a link to the row-list fragment a live page
// re-fetches, so an update re-renders the feed the reader narrowed rather
// than widening it back out to everything.
func (q activityQuery) FeedHref() string {
	values := q.values()
	if len(values) == 0 {
		return RouteActivityFeed
	}
	return RouteActivityFeed + "?" + values.Encode()
}

// WithActor is this filter narrowed to one actor, which is what a row's actor
// name links to.
func (q activityQuery) WithActor(id string) string {
	q.Actor = id
	return q.Href("")
}

// Without is this filter with one field cleared, which is what each active
// filter's own remove control links to.
func (q activityQuery) Without(name string) string {
	switch name {
	case "q":
		q.Text = ""
	case "actor":
		q.Actor = ""
	case "kind":
		q.Kind = ""
	case "source":
		q.Source = ""
	}
	return q.Href("")
}

// activityView is what the activity screen, and its live-update fragment,
// render. Both are built by buildActivityView, so a row that arrives live is
// produced by the exact same grouping and wording as one that was already on
// the page at load.
type activityView struct {
	Groups     []activityGroup
	Names      actorNames
	Cursor     string
	Pager      pager
	Query      activityQuery
	ActorLabel string
	Kinds      []activityChoice
	Sources    []core.Source
	Scanned    int
}

// activityChoice is one option of the subject-kind control: the value the
// filter carries and the words the control shows for it.
type activityChoice struct {
	Value string
	Label string
}

// activityKinds are the subject kinds the filter offers, in the order they
// read. Every one of them is a subject type the service actually records; the
// list is fixed rather than derived from the page on screen, so the control
// does not change shape as a reader pages through.
var activityKinds = []activityChoice{
	{"task", "Tasks"},
	{"comment", "Comments"},
	{"artifact", "Artifacts"},
	{"project", "Projects"},
	{"workflow", "Workflows"},
	{"user", "Users"},
	{"membership", "Memberships"},
	{"session", "Sign-ins"},
	{"api_token", "API tokens"}, // #nosec G101 -- a display label, not a credential
	{"webhook", "Webhooks"},
	{"tenant", "Tenant"},
}

// activitySources are the surfaces a change can arrive from.
var activitySources = []core.Source{
	core.SourceWeb, core.SourceCLI, core.SourceAPI, core.SourceTUI, core.SourceSystem,
}

// showActivity renders recent tenant activity, newest first.
func (h *handler) showActivity(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildActivityView(r, r.URL.Query().Get(CursorParam))
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
// live alike. It reads the same filter from its own query string, which the
// page passes along, so a live update never widens a feed the reader
// narrowed.
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

// activityPageSize is how many audit entries one screen of the feed shows.
const activityPageSize = 50

// activityScanPages bounds how many store pages one screen may read looking
// for matches. A structured filter (actor, kind, source) is answered by the
// store itself, so it needs one page; only the free-text box can leave a
// store page with nothing on it, and this is what stops a search term that
// matches nothing from walking the whole audit table in one request. A screen
// that hits the bound still offers its Older link, so the reader can keep
// going rather than being told the search is over.
const activityScanPages = 8

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
	query := activityQueryFrom(r)
	entries, next, scanned, err := h.scanAudit(r, query, cursor)
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
	actors := make([]string, 0, len(rows)+len(groups)+1)
	for _, row := range rows {
		actors = append(actors, row.Entry.ActorID)
		if row.Entry.SubjectType == "session" || row.Entry.SubjectType == "membership" {
			actors = append(actors, row.Entry.SubjectID)
		}
	}
	if query.Actor != "" {
		actors = append(actors, query.Actor)
	}
	names := h.resolveActors(r, actors...)

	workflows := h.workflowNames(r, groups)
	tenant := h.tenantName(r, groups)
	users := h.userNames(r, groups)
	tasks := taskCache{}

	data := activityView{
		Groups: make([]activityGroup, len(groups)), Cursor: cursor,
		Pager: newPager(r, RouteActivity, next, len(groups), "entries",
			"q", "kind", "actor", "source"),
		Names: names, Query: query, Kinds: activityKinds, Sources: activitySources,
		Scanned: scanned,
	}
	if query.Actor != "" {
		data.ActorLabel = names.Label(query.Actor)
	}
	for i, g := range groups {
		entry := g.Latest().Entry
		subject, href, projectID := h.subjectFor(r, entry, accents, workflows, tenant, users, names, tasks)
		data.Groups[len(groups)-1-i] = activityGroup{
			Group: g, Subject: subject, SubjectHref: href,
			ActorHref: query.WithActor(entry.ActorID), Accent: accents[projectID],
		}
	}
	return data, nil
}

// scanAudit reads audit entries newest first until it has a screenful that
// the filter accepts, and reports the cursor to resume from. The structured
// parts of the filter are handed to the store, which is the only place that
// can apply them without reading rows it then throws away; the free-text box
// is applied here, over entries the store already returned, which is why this
// loops at all and why activityScanPages bounds it.
func (h *handler) scanAudit(r *http.Request, query activityQuery, cursor string) ([]core.AuditEntry, string, int, error) {
	filter := core.AuditFilter{Page: core.Page{
		Cursor: cursor, Sort: "seq", Direction: core.Descending, Limit: activityPageSize,
	}}
	if query.Actor != "" {
		filter.ActorIDs = []string{query.Actor}
	}
	if query.Kind != "" {
		filter.SubjectType = query.Kind
	}
	if query.Source != "" {
		filter.Sources = []core.Source{core.Source(query.Source)}
	}

	out := make([]core.AuditEntry, 0, activityPageSize)
	scanned, next := 0, cursor
	for range activityScanPages {
		filter.Page.Cursor = next
		entries, after, err := h.svc.ListAudit(r.Context(), filter)
		if err != nil {
			return nil, "", 0, err
		}
		scanned += len(entries)
		for _, entry := range entries {
			if matchesText(entry, query.Text) {
				out = append(out, entry)
			}
		}
		next = after
		if next == "" || len(out) >= activityPageSize {
			break
		}
	}
	if len(out) > activityPageSize {
		out = out[:activityPageSize]
	}
	return out, next, scanned, nil
}

// matchesText reports whether one entry answers a free-text search. The text
// is matched against what the entry itself records -- its action, the kind of
// record it touched, the surface it arrived from, and the before and after
// snapshots -- rather than against the sentence the row will eventually read
// as, because the snapshot is where the words a reader actually searches for
// live: a task's reference and title, a project's key, a comment's body.
func matchesText(entry core.AuditEntry, text string) bool {
	if text == "" {
		return true
	}
	needle := strings.ToLower(text)
	for _, hay := range []string{
		entry.Action, entry.SubjectType, string(entry.Source),
		string(entry.Before), string(entry.After),
	} {
		if strings.Contains(strings.ToLower(hay), needle) {
			return true
		}
	}
	return false
}

// subjectKindNouns names the subject types the feed cannot look a specific
// record up for as cheaply as a task, a project or a workflow: naming one of
// these would take a query per row rather than one list call for the whole
// page, or a listing only a tenant admin may call (see workflowNames and
// tenantName for the two admin-gated lookups this screen does make, and why
// a failure there falls back here too rather than breaking the page for a
// viewer or member). Saying what kind of thing changed, without a name, is
// still honest; showing the bare identifier, or inventing a name, is not.
// #nosec G101 -- these are display nouns for audit subject kinds, not
// credentials; the scanner matches on the word "token" in "an API token".
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

// taskCache holds the tasks one screen has already looked up. A feed page
// routinely names the same task several times -- created, moved, commented on
// -- and each of those rows used to cost its own GetTask.
type taskCache map[string]*core.Task

// subjectFor names what an entry happened to, where it links, if anywhere,
// and the id of the project it belongs to, if any. A task is shown, and
// linked, by the reference people quote to each other, and its project id
// comes along for free since finding the task already required loading it.
// A comment or an artifact is reached through the task it hangs off, with a
// fragment that lands on the record itself, because "a comment" with nowhere
// to go was the row readers complained about most. A project or workflow is
// named the same way, from the page's own batched lookup, and links to its
// own screen. A session or membership names the actor it belongs to, since
// that is literally what its subject id is. Everything else names its kind
// (subjectKindNouns) rather than an identifier nobody recognises or a name
// nobody wrote down for it.
func (h *handler) subjectFor(r *http.Request, entry core.AuditEntry, accents map[string]projectAccent, workflows map[string]core.Workflow, tenant string, users map[string]core.User, names actorNames, tasks taskCache) (label, href, projectID string) {
	switch entry.SubjectType {
	case "task":
		if entry.SubjectID == "" {
			break
		}
		if task := h.taskByID(r, tasks, entry.SubjectID); task != nil && task.Ref != "" {
			return task.Ref, RouteTasks + "/" + task.Ref, task.ProjectID
		}
	case "comment", "artifact":
		if task := h.taskOfAttachment(r, tasks, entry); task != nil && task.Ref != "" {
			noun := subjectKindNouns[entry.SubjectType]
			anchor := "#" + entry.SubjectType + "-" + entry.SubjectID
			return noun + " on " + task.Ref, RouteTasks + "/" + task.Ref + anchor, task.ProjectID
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
				return "the user " + name, RouteUsers + "#user-" + u.ID, ""
			}
		}
	case "session", "membership":
		if label := names.Label(entry.SubjectID); label != "" {
			return label, RouteActivity + "?actor=" + url.QueryEscape(entry.SubjectID), ""
		}
	}
	if noun, ok := subjectKindNouns[entry.SubjectType]; ok {
		return noun, "", ""
	}
	return shortID(entry.SubjectID), "", ""
}

// taskByID loads one task, remembering it for the rest of the page. A task
// the caller may not read is cached as absent, so a feed carrying twenty rows
// about it does not retry the refusal twenty times.
func (h *handler) taskByID(r *http.Request, tasks taskCache, id string) *core.Task {
	if cached, seen := tasks[id]; seen {
		return cached
	}
	tasks[id] = nil
	task, err := h.svc.GetTask(r.Context(), core.TaskRef{ID: id})
	if err == nil && task != nil {
		tasks[id] = task
	}
	return tasks[id]
}

// taskOfAttachment finds the task a comment or artifact entry hangs off. The
// audit entry's own snapshot carries the task identifier -- a comment row is
// written with the whole comment as its after image, and a deleted one with
// it as its before image -- so this needs no extra listing, only the lookup
// the task rows of the same page already make.
func (h *handler) taskOfAttachment(r *http.Request, tasks taskCache, entry core.AuditEntry) *core.Task {
	id := taskIDOf(entry.After)
	if id == "" {
		id = taskIDOf(entry.Before)
	}
	if id == "" {
		return nil
	}
	return h.taskByID(r, tasks, id)
}

// taskIDOf reads the task identifier out of an audit snapshot.
func taskIDOf(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var snapshot struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ""
	}
	return snapshot.TaskID
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
