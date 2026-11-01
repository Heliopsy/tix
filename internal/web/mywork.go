// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/i18n"
)

// My work: the task screen narrowed to the reader's own tasks and drawn as a
// list under one heading per deadline bucket.
const (
	RouteMine     = "/mine"
	RouteMineLive = "/mine/live"

	// HeldParam takes in the tasks the reader holds a claim on, as well as
	// the ones assigned to them.
	HeldParam = "held"

	// ListGroupDue groups the list by deadline. Only My work draws it.
	ListGroupDue = "due"

	// myWorkQuery is what My work answers, in the filter language, so the
	// filter bar can show it and Save can keep it.
	myWorkQuery = "assignee:" + core.MeRef
	// myWorkLimit is how many tasks My work draws. It is one page, because
	// buckets split a page and a pager across buckets reads as a bug; past
	// it the screen says so and links the full listing.
	myWorkLimit = 200
)

// dueBucketLabels name each deadline bucket on its heading.
var dueBucketLabels = map[string]string{
	core.DueBucketOverdue: "Overdue",
	core.DueBucketToday:   "Today",
	core.DueBucketWeek:    "This week",
	core.DueBucketLater:   "Later",
	core.DueBucketNone:    "No date",
}

// myWorkRoutes are My work and its live fragment.
func (h *handler) myWorkRoutes() []route {
	return []route{
		get(RouteMine, "tasks.html", h.showMyWork, "ListTasks", "ListGroupPrefs"),
		get(RouteMineLive, "tasks.html", h.showMyWorkLive, "ListTasks", "ListGroupPrefs"),
	}
}

// showMyWork renders the reader's own work.
func (h *handler) showMyWork(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildTaskScreen(r, true)
	if err != nil {
		return err
	}
	return h.render(w, r, "tasks.html", "My work", data)
}

// showMyWorkLive renders the regions a live update of My work replaces.
func (h *handler) showMyWorkLive(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildTaskScreen(r, true)
	if err != nil {
		return err
	}
	return h.renderFragment(w, r, "tasks.html", "tasks-live", "My work", data)
}

// withHeld adds the tasks the reader holds to the ones assigned to them,
// each once, under the same narrowing.
func (h *handler) withHeld(r *http.Request, assigned core.TaskFilter, page core.TaskPage) (core.TaskPage, error) {
	f := assigned
	f.AssigneeIDs, f.ClaimedBy = nil, []string{core.MeRef}
	f.Page.Cursor = ""
	held, err := h.svc.ListTasks(r.Context(), f)
	if err != nil {
		return page, err
	}
	seen := make(map[string]bool, len(page.Tasks))
	for _, t := range page.Tasks {
		seen[t.ID] = true
	}
	for _, t := range held.Tasks {
		if !seen[t.ID] {
			page.Tasks = append(page.Tasks, t)
		}
	}
	if held.NextCursor != "" {
		page.NextCursor = held.NextCursor
	}
	return page, nil
}

// readerZone is where "today" ends for this reader: the zone they chose,
// or the deployment's.
func (h *handler) readerZone(r *http.Request) *time.Location {
	if zone := timezoneOf(r); zone != "" {
		if loc, err := time.LoadLocation(zone); err == nil {
			return loc
		}
	}
	return h.style.Location()
}

// buildDueGroups splits tasks into the deadline buckets, in bucket order,
// leaving out the empty ones. A task keeps its place in the sort.
func buildDueGroups(tasks []core.Task, now time.Time, loc *time.Location, label func(string) string) []listGroup {
	byBucket := map[string][]core.Task{}
	for _, t := range tasks {
		b := core.DueBucketOf(t.DueAt, now, loc)
		byBucket[b] = append(byBucket[b], t)
	}
	var out []listGroup
	for _, b := range core.DueBuckets {
		if len(byBucket[b]) == 0 {
			continue
		}
		out = append(out, listGroup{
			Head:  groupHead{Key: b, Name: label(dueBucketLabels[b]), Count: len(byBucket[b]), Kind: groupDue},
			Tasks: byBucket[b],
		})
	}
	return out
}

// arrangeMine groups My work by deadline and arranges the buckets the way the
// reader arranged the mine view.
func (h *handler) arrangeMine(r *http.Request, tasks []core.Task, self string) ([]listGroup, arrangement) {
	lang := langOf(r)
	label := func(text string) string { return i18n.T(lang, text) }
	groups := buildDueGroups(tasks, time.Now(), h.readerZone(r), label)
	prefs := h.groupPrefs(r, core.ViewMine)
	groups, _ = arrangeGroups(groups, listHead, groupPrefIndex(prefs))
	name := func(key string) (string, bool) {
		text, ok := dueBucketLabels[key]
		return label(text), ok
	}
	a := newArrangement(r, core.ViewMine, self, tasksRegion, drawnOrder(groups, listHead), hiddenOf(prefs, name), prefs)
	a.Vertical, a.CanCreate = true, false
	return groups, a
}

// myWorkRequest is the request My work asks its question with: its own
// address with the filter set to assignee:me and the deadline window and
// cursor dropped, so listingFilter answers it like any typed filter.
func myWorkRequest(r *http.Request) *http.Request {
	q := r.URL.Query()
	q.Set("q", myWorkQuery)
	q.Del(DueParam)
	q.Del(CursorParam)
	out := r.Clone(r.Context())
	u := *r.URL
	u.RawQuery = q.Encode()
	out.URL = &u
	return out
}

// heldHref is My work with the held tasks taken in, or left out.
func heldHref(r *http.Request, on bool) string {
	if on {
		return RouteMine + "?" + HeldParam + "=1"
	}
	return RouteMine
}
