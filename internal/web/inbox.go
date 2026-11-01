// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
)

// inboxRoutes are the inbox screen, its badge and the forms that mark items.
func (h *handler) inboxRoutes() []route {
	return []route{
		get(RouteInbox, "inbox.html", h.showInbox, "ListInbox", "CountInbox"),
		get(RouteInboxBadge, "inbox.html", h.showInboxBadge, "CountInbox"),
		post(RouteInboxRead, h.readInboxItem, "ReadInboxItem"),
		post(RouteInboxUnread, h.unreadInboxItem, "UnreadInboxItem"),
		post(RouteInboxReadAll, h.readAllInbox, "ReadAllInbox"),
		post(RouteInboxMutes, h.setInboxMute, "GetInboxPrefs", "PutInboxPrefs"),
	}
}

// inboxView is what the inbox screen renders.
type inboxView struct {
	Items  []inboxRow
	Unread bool
	Count  int
	Here   string
	Pager  pager
}

// inboxRow is one item as the screen draws it.
type inboxRow struct {
	core.InboxItem
	// Href is the task's own screen, which the dialog setting may open over
	// the inbox instead.
	Href string
}

// showInbox renders one page of the reader's inbox, newest first.
func (h *handler) showInbox(w http.ResponseWriter, r *http.Request) error {
	unread := r.URL.Query().Get("show") == "unread"
	items, next, err := h.svc.ListInbox(r.Context(), core.InboxFilter{Unread: unread,
		Page: core.Page{Limit: h.rowsPerPage(r), Cursor: r.URL.Query().Get(CursorParam)}})
	if err != nil {
		return err
	}
	rows := make([]inboxRow, len(items))
	for i, it := range items {
		rows[i] = inboxRow{InboxItem: it}
		if !it.TaskGone && it.TaskRef != "" {
			rows[i].Href = "/tasks/" + url.PathEscape(it.TaskRef)
		}
	}
	return h.render(w, r, "inbox.html", "Inbox", inboxView{
		Items: rows, Unread: unread, Count: h.unreadCount(r), Here: here(r),
		Pager: newPager(r, RouteInbox, next, len(rows), "items", "show"),
	})
}

// showInboxBadge renders the sidebar's unread badge alone, for the live
// refresh to swap in.
func (h *handler) showInboxBadge(w http.ResponseWriter, r *http.Request) error {
	v := h.newView(r, "inbox.html", "Inbox", nil)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	return h.templatesFor(r)["inbox.html"].ExecuteTemplate(w, "inbox-badge", v)
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
	redirect(w, r, safeNext(orElse(field(r, "next"), savedNext("Inbox", "inbox"))), "")
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
	Receive bool
}

// inboxKindChoices lists every kind with whether the reader receives it.
func inboxKindChoices(prefs *core.InboxPrefs) []inboxKindChoice {
	labels := map[core.InboxKind][2]string{
		core.InboxMention:  {"Mentions", "Somebody writes @you in a comment or a description."},
		core.InboxAssigned: {"Assignments", "A task is assigned to you."},
		core.InboxStatus:   {"Status changes", "A task assigned to you moves to another status."},
		core.InboxComment:  {"Comments", "Somebody comments on a task you are assigned to or starred."},
	}
	out := make([]inboxKindChoice, 0, len(labels))
	for _, k := range core.InboxKinds() {
		out = append(out, inboxKindChoice{Kind: k, Label: labels[k][0], Help: labels[k][1],
			Receive: prefs == nil || !prefs.Mutes(k)})
	}
	return out
}
