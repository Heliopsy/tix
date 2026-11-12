// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// inboxRoutes are the inbox screen, its badge and the forms that mark items.
func (h *handler) inboxRoutes() []route {
	return []route{
		get(RouteInbox, "inbox.html", h.showInbox, "ListInbox", "CountInbox", "GetInboxPrefs", "ListTasks", "Stats"),
		get(RouteInboxBadge, "inbox.html", h.showInboxBadge, "CountInbox"),
		post(RouteInboxRead, h.readInboxItem, "ReadInboxItem"),
		post(RouteInboxUnread, h.unreadInboxItem, "UnreadInboxItem"),
		post(RouteInboxReadAll, h.readAllInbox, "ReadAllInbox"),
		post(RouteInboxMutes, h.setInboxMute, "GetInboxPrefs", "PutInboxPrefs"),
	}
}

// InboxKindsParam names the kinds the inbox screen keeps, comma separated.
const InboxKindsParam = "kinds"

// inboxView is what the inbox screen renders.
type inboxView struct {
	Live    liveScreen
	Items   []inboxRow
	Groups  []inboxGroup
	Kinds   []inboxKindFilter
	Summary *inboxSummary
	Unread  bool
	// Filtered is true when the screen keeps only some kinds.
	Filtered bool
	Count    int
	Here     string
	// AllHref and UnreadHref are the two segments, each keeping the kinds.
	AllHref    string
	UnreadHref string
	Pager      pager
}

// inboxRow is one item as the screen draws it.
type inboxRow struct {
	core.InboxItem
	// Href is the task's own screen, which the dialog setting may open over
	// the inbox instead.
	Href string
	// Stop is the label of the action that stops items of this kind, empty
	// when the reader already stopped it or the service would not say.
	Stop string
}

// inboxGroup is one day heading of the inbox and the items under it.
type inboxGroup struct {
	Key   string
	Label string
	Items []inboxRow
}

// inboxKindFilter is one kind's chip on the inbox bar: a link that adds the
// kind to the ones kept, or takes it away.
type inboxKindFilter struct {
	Kind  core.InboxKind
	Label string
	On    bool
	Href  string
}

// inboxSummary is the card at the top of the inbox: the reader's overdue and
// due-today tasks, the tenant's open work and what was finished this week.
type inboxSummary struct {
	Overdue   string
	DueToday  string
	Open      string
	DoneWeek  string
	HasDue    bool
	HasTenant bool
}

// showInbox renders one page of the reader's inbox, newest first.
func (h *handler) showInbox(w http.ResponseWriter, r *http.Request) error {
	unread := r.URL.Query().Get("show") == "unread"
	kinds := inboxKindsOf(r)
	size := h.rowsPerPage(r)
	read := func(cursor string, limit int) ([]core.InboxItem, string, error) {
		return h.svc.ListInbox(r.Context(), core.InboxFilter{Unread: unread, Kinds: kinds,
			Page: core.Page{Limit: limit, Cursor: cursor}})
	}
	pos, err := locate(r, size, walkOf(read))
	if err != nil {
		return err
	}
	items, next, err := read(pos.Cursor, size)
	if err != nil {
		return err
	}
	prefs := h.inboxPrefs(r)
	rows := make([]inboxRow, len(items))
	for i, it := range items {
		rows[i] = inboxRow{InboxItem: it}
		if !it.TaskGone && it.TaskRef != "" {
			rows[i].Href = "/tasks/" + url.PathEscape(it.TaskRef)
		}
		if prefs != nil && !prefs.Mutes(it.Kind) {
			rows[i].Stop = inboxStopLabels[it.Kind]
		}
	}
	return h.render(w, r, "inbox.html", "Inbox", inboxView{
		Live: liveScreen{Events: h.eventsPath, Href: fragmentHref(r, RouteInbox),
			Filter:  liveFilter(nil, []core.EventType{core.EventInboxUpdated}),
			Regions: "inbox-summary inbox-bar inbox-list", Window: liveListWindow},
		Items: rows, Groups: groupInboxByDay(rows, time.Now(), h.readerZone(r)),
		Kinds: inboxKindFilters(unread, kinds), Summary: h.inboxSummary(r),
		Unread: unread, Filtered: len(kinds) > 0, Count: h.unreadCount(r), Here: here(r),
		AllHref: inboxHref(false, kinds), UnreadHref: inboxHref(true, kinds),
		Pager: newPager(r, RouteInbox, next, len(rows), "items", "show", InboxKindsParam).counted(pos, size),
	})
}

// inboxKindsOf reads the kinds the screen keeps from its address, leaving
// out a name tix does not have: the value arrives from a link.
func inboxKindsOf(r *http.Request) []core.InboxKind {
	var out []core.InboxKind
	for _, name := range strings.Split(r.URL.Query().Get(InboxKindsParam), ",") {
		k := core.InboxKind(strings.ToLower(strings.TrimSpace(name)))
		if k.Valid() && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// inboxHref is the inbox address showing everything or unread items, of
// the kinds given, in the order tix lists kinds.
func inboxHref(unread bool, kinds []core.InboxKind) string {
	q := url.Values{}
	if unread {
		q.Set("show", "unread")
	}
	var names []string
	for _, k := range core.InboxKinds() {
		if slices.Contains(kinds, k) {
			names = append(names, string(k))
		}
	}
	if len(names) > 0 {
		q.Set(InboxKindsParam, strings.Join(names, ","))
	}
	if len(q) == 0 {
		return RouteInbox
	}
	return RouteInbox + "?" + q.Encode()
}

// inboxKindFilters are the bar's kind chips, each linking to the screen with
// its kind toggled and everything else kept.
func inboxKindFilters(unread bool, kinds []core.InboxKind) []inboxKindFilter {
	out := make([]inboxKindFilter, 0, len(core.InboxKinds()))
	for _, k := range core.InboxKinds() {
		on := slices.Contains(kinds, k)
		next := slices.DeleteFunc(slices.Clone(kinds), func(x core.InboxKind) bool { return x == k })
		if !on {
			next = append(next, k)
		}
		out = append(out, inboxKindFilter{Kind: k, Label: inboxKindLabels[k], On: on, Href: inboxHref(unread, next)})
	}
	return out
}

// inboxKindLabels name each kind, plural, as the filters and settings say it.
var inboxKindLabels = map[core.InboxKind]string{
	core.InboxMention:  "Mentions",
	core.InboxAssigned: "Assignments",
	core.InboxStatus:   "Status changes",
	core.InboxComment:  "Comments",
}

// inboxStopLabels are the row action stopping each kind.
var inboxStopLabels = map[core.InboxKind]string{
	core.InboxMention:  "Stop mentions",
	core.InboxAssigned: "Stop assignments",
	core.InboxStatus:   "Stop status changes",
	core.InboxComment:  "Stop comments",
}

// inboxStoppedFlashes are the toasts a stop from the inbox answers with.
var inboxStoppedFlashes = map[core.InboxKind]string{
	core.InboxMention:  "Mentions no longer reach your inbox",
	core.InboxAssigned: "Assignments no longer reach your inbox",
	core.InboxStatus:   "Status changes no longer reach your inbox",
	core.InboxComment:  "Comments no longer reach your inbox",
}

// inboxDayLabels name the day headings.
var inboxDayLabels = map[string]string{"today": "Today", "yesterday": "Yesterday", "earlier": "Earlier"}

// groupInboxByDay splits rows, newest first, under Today, Yesterday and
// Earlier as days are counted in loc, leaving out an empty heading.
func groupInboxByDay(rows []inboxRow, now time.Time, loc *time.Location) []inboxGroup {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)
	var out []inboxGroup
	for _, row := range rows {
		key := "earlier"
		switch {
		case !row.CreatedAt.Before(today):
			key = "today"
		case !row.CreatedAt.Before(yesterday):
			key = "yesterday"
		}
		if len(out) == 0 || out[len(out)-1].Key != key {
			out = append(out, inboxGroup{Key: key, Label: inboxDayLabels[key]})
		}
		out[len(out)-1].Items = append(out[len(out)-1].Items, row)
	}
	return out
}

// summaryCap is the largest count the summary card spells out.
const summaryCap = 99

// inboxSummary reads the summary card's numbers. A half it cannot read is
// left out, and a reader the service tells nothing gets no card: a summary
// is never a reason to refuse the inbox.
func (h *handler) inboxSummary(r *http.Request) *inboxSummary {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return nil
	}
	s := &inboxSummary{}
	if overdue, today, ok := h.dueSoon(r); ok {
		s.Overdue, s.DueToday, s.HasDue = capped(overdue), capped(today), true
	}
	if stats, err := h.svc.Stats(r.Context(), core.StatsInput{Window: core.Duration(7 * 24 * time.Hour)}); err == nil && stats != nil {
		open := 0
		for _, c := range stats.ByCategory {
			if c.Category != core.CategoryDone && c.Category != core.CategoryCancelled {
				open += c.Count
			}
		}
		s.Open, s.DoneWeek, s.HasTenant = strconv.Itoa(open), strconv.Itoa(stats.Completed), true
	}
	if !s.HasDue && !s.HasTenant {
		return nil
	}
	return s
}

// dueSoon counts the reader's unfinished assigned tasks past their deadline
// and those falling due later today, in the reader's zone.
func (h *handler) dueSoon(r *http.Request) (overdue, today int, ok bool) {
	filter, err := ParseFilter(myWorkQuery)
	if err != nil {
		return 0, 0, false
	}
	open, err := h.unfinishedStatuses(r.Context())
	if err != nil || len(open) == 0 {
		return 0, 0, false
	}
	now := time.Now()
	loc := h.readerZone(r)
	local := now.In(loc)
	end := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
	filter.Statuses, filter.DueBefore, filter.OmitBody = open, &end, true
	filter.Page = core.Page{Limit: 2*summaryCap + 2, Sort: core.SortDueAt, Direction: core.Ascending}
	page, err := h.svc.ListTasks(r.Context(), filter)
	if err != nil {
		return 0, 0, false
	}
	for _, t := range page.Tasks {
		switch core.DueBucketOf(t.DueAt, now, loc) {
		case core.DueBucketOverdue:
			overdue++
		case core.DueBucketToday:
			today++
		}
	}
	return overdue, today, true
}

// capped spells a count, or 99+ past summaryCap.
func capped(n int) string {
	if n > summaryCap {
		return strconv.Itoa(summaryCap) + "+"
	}
	return strconv.Itoa(n)
}

// showInboxBadge renders the sidebar's two badges, the unread count and My
// work's overdue count, for the live refresh to swap in.
func (h *handler) showInboxBadge(w http.ResponseWriter, r *http.Request) error {
	v := h.newView(r, "inbox.html", "Inbox", nil)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	return h.templatesFor(r)["inbox.html"].ExecuteTemplate(w, "side-badges", v)
}

// unreadCount is the reader's unread count, or nothing for a reader the
// service will not tell: a badge is never a reason to refuse a screen.
func (h *handler) unreadCount(r *http.Request) int {
	if _, ok := core.ActorFrom(r.Context()); !ok {
		return 0
	}
	n, err := h.svc.CountInbox(r.Context())
	if err != nil {
		return 0
	}
	return n.Unread
}

func (h *handler) readInboxItem(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.ReadInboxItem(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirectTo(w, r, inboxNext(r), "", "")
	return nil
}

func (h *handler) unreadInboxItem(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.UnreadInboxItem(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirectTo(w, r, inboxNext(r), "", "")
	return nil
}

func (h *handler) readAllInbox(w http.ResponseWriter, r *http.Request) error {
	if _, err := h.svc.ReadAllInbox(r.Context()); err != nil {
		return err
	}
	redirect(w, r, inboxNext(r), "inbox marked read")
	return nil
}

// inboxNext is where an inbox form returns to: the page it was posted from,
// or the inbox.
func inboxNext(r *http.Request) string {
	next := field(r, "next")
	if next == "" {
		return RouteInbox
	}
	return safeNext(next)
}

// setInboxMute turns one kind of inbox item on or off for the reader.
func (h *handler) setInboxMute(w http.ResponseWriter, r *http.Request) error {
	kinds, err := core.ParseInboxKinds([]string{field(r, "kind")})
	if err != nil {
		return err
	}
	if len(kinds) != 1 {
		return core.Invalid("name the kind of inbox item to turn on or off").WithDetail(core.DetailField, "kind")
	}
	prefs, err := h.svc.GetInboxPrefs(r.Context())
	if err != nil {
		return err
	}
	muted := []string{}
	for _, k := range prefs.Muted {
		if k != kinds[0] {
			muted = append(muted, string(k))
		}
	}
	if field(r, "receive") != "on" {
		muted = append(muted, string(kinds[0]))
	}
	if _, err := h.svc.PutInboxPrefs(r.Context(), core.InboxPrefsInput{Muted: muted}); err != nil {
		return err
	}
	flash := ""
	if field(r, "stop") == "1" {
		flash = inboxStoppedFlashes[kinds[0]]
	}
	redirect(w, r, safeNext(orElse(field(r, "next"), savedNext("Inbox", "inbox"))), flash)
	return nil
}

// inboxPrefs is the reader's mutes, or none for a reader the service will
// not tell.
func (h *handler) inboxPrefs(r *http.Request) *core.InboxPrefs {
	prefs, err := h.svc.GetInboxPrefs(r.Context())
	if err != nil {
		return nil
	}
	return prefs
}

// inboxKindChoice is one switch of the settings screen's Inbox section.
type inboxKindChoice struct {
	Kind    core.InboxKind
	Label   string
	Help    string
	Info    string
	Receive bool
}

// inboxKindChoices lists every kind with whether the reader receives it.
func inboxKindChoices(prefs *core.InboxPrefs) []inboxKindChoice {
	labels := map[core.InboxKind][3]string{
		core.InboxMention: {inboxKindLabels[core.InboxMention], "Somebody writes @you in a comment or a description.",
			"An item when somebody writes @ and your handle in a comment or a task's description. Editing the text mentions only handles it did not name before. Off, a mention creates no item; items already in your inbox stay."},
		core.InboxAssigned: {"Assignments", "A task is assigned to you.",
			"An item when somebody else assigns a task to you. Off, an assignment creates no item; items already in your inbox stay."},
		core.InboxStatus: {"Status changes", "A task assigned to you moves to another status.",
			"An item when somebody else moves a task assigned to you to another workflow state. Off, a move creates no item; items already in your inbox stay."},
		core.InboxComment: {"Comments", "Somebody comments on a task you are assigned to or starred.",
			"An item when somebody else comments on a task assigned to you or one you starred. Off, a comment creates no item; items already in your inbox stay."},
	}
	out := make([]inboxKindChoice, 0, len(labels))
	for _, k := range core.InboxKinds() {
		out = append(out, inboxKindChoice{Kind: k, Label: labels[k][0], Help: labels[k][1], Info: labels[k][2],
			Receive: prefs == nil || !prefs.Mutes(k)})
	}
	return out
}
