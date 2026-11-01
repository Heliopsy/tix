// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// Audit actions of the inbox.
const (
	auditInboxRead    = "inbox.read"
	auditInboxUnread  = "inbox.unread"
	auditInboxReadAll = "inbox.read_all"
	auditInboxPrefs   = "inbox.prefs"
	inboxSubject      = "inbox"
)

// inboxNote is one reason a change may concern one actor. A mutation queues
// them as it goes and resolves them all at flush.
type inboxNote struct {
	recipient string
	kind      core.InboxKind
	task      *core.Task
	commentID string
	from, to  string
	excerpt   string
}

// notify queues inbox notes for every recipient named.
func (m *mutation) notify(n inboxNote, recipients ...string) {
	for _, r := range recipients {
		if r == "" {
			continue
		}
		n.recipient = r
		m.notes = append(m.notes, n)
	}
}

// notifyMentions queues a mention for every handle in text that the earlier
// text did not already name.
func (m *mutation) notifyMentions(ctx context.Context, n inboxNote, text, earlier string) error {
	handles := newMentions(text, earlier)
	if len(handles) == 0 {
		return nil
	}
	n.kind = core.InboxMention
	n.excerpt = excerpt(text)
	for _, h := range handles {
		a, err := m.tx.GetActorByHandle(ctx, h)
		if err != nil {
			if core.IsKind(err, core.KindNotFound) {
				continue
			}
			return err
		}
		m.notify(n, a.ID)
	}
	return nil
}

// notifyComment queues what one new comment on a task means to the people
// following it: the mentions in it, and a comment for the assignee and
// everyone who starred the task.
func (m *mutation) notifyComment(ctx context.Context, task *core.Task, c *core.Comment) error {
	n := inboxNote{task: task, commentID: c.ID}
	if err := m.notifyMentions(ctx, n, c.Body, ""); err != nil {
		return err
	}
	watchers, err := m.tx.TaskStarrers(ctx, task.ID)
	if err != nil {
		return err
	}
	n.kind, n.excerpt = core.InboxComment, excerpt(c.Body)
	m.notify(n, append([]string{task.AssigneeActorID}, watchers...)...)
	return nil
}

// notifyStatus queues a status change for the task's assignee.
func (m *mutation) notifyStatus(task *core.Task, from, to string) {
	if from == to {
		return
	}
	m.notify(inboxNote{kind: core.InboxStatus, task: task, from: from, to: to}, task.AssigneeActorID)
}

// notifyAssigned queues an assignment for the task's assignee.
func (m *mutation) notifyAssigned(task *core.Task) {
	m.notify(inboxNote{kind: core.InboxAssigned, task: task, excerpt: excerpt(task.Title)}, task.AssigneeActorID)
}

// deliverInbox writes the queued notes as inbox items: never to the actor who
// made the change, never of a kind the recipient muted, never to an
// identifier that names no actor, and at most one per recipient and task,
// the strongest reason winning. One inbox.updated event names everybody who
// received something.
func (m *mutation) deliverInbox(ctx context.Context) error {
	if len(m.notes) == 0 {
		return nil
	}
	notes, err := m.eligibleNotes(ctx)
	if err != nil || len(notes) == 0 {
		return err
	}
	type key struct{ recipient, task string }
	best := map[key]int{}
	var order []key
	for i, n := range notes {
		k := key{n.recipient, n.task.ID}
		at, ok := best[k]
		if !ok {
			order = append(order, k)
			best[k] = i
			continue
		}
		if n.kind.Rank() < notes[at].kind.Rank() {
			best[k] = i
		}
	}
	var recipients []string
	var created []*core.InboxItem
	seen := map[string]bool{}
	for _, k := range order {
		n := notes[best[k]]
		item := &core.InboxItem{
			RecipientID: n.recipient, Kind: n.kind, TaskID: n.task.ID, ActorID: m.actor.ID,
			CommentID: n.commentID, FromStatus: n.from, ToStatus: n.to, Excerpt: n.excerpt,
			CreatedAt: m.now,
		}
		if err := m.tx.CreateInboxItem(ctx, item); err != nil {
			return err
		}
		created = append(created, item)
		if !seen[n.recipient] {
			seen[n.recipient] = true
			recipients = append(recipients, n.recipient)
		}
	}
	if err := m.queueChannels(ctx, created, recipients); err != nil {
		return err
	}
	first := notes[best[order[0]]].task
	m.Event(core.EventInboxUpdated, inboxSubject, first.ID, first.ProjectID,
		map[string]any{"recipients": recipients, "ref": first.Ref, "task_id": first.ID})
	return nil
}

// eligibleNotes drops the notes nobody should receive.
func (m *mutation) eligibleNotes(ctx context.Context) ([]inboxNote, error) {
	known := map[string]bool{}
	var ids []string
	for _, n := range m.notes {
		if n.recipient == m.actor.ID || n.recipient == systemActorID || n.task == nil {
			continue
		}
		if _, done := known[n.recipient]; done {
			continue
		}
		_, err := m.tx.GetActor(ctx, n.recipient)
		switch {
		case err == nil:
			known[n.recipient] = true
			ids = append(ids, n.recipient)
		case core.IsKind(err, core.KindNotFound):
			known[n.recipient] = false
		default:
			return nil, err
		}
	}
	mutes, err := m.tx.InboxMutesOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	var out []inboxNote
	for _, n := range m.notes {
		if !known[n.recipient] {
			continue
		}
		if (core.InboxPrefs{Muted: mutes[n.recipient]}).Mutes(n.kind) {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// mentionPattern finds @handle not preceded by a word character, so an email
// address is not a mention.
var mentionPattern = regexp.MustCompile(`(?:^|[^\w@.])@([A-Za-z0-9][A-Za-z0-9._-]*)`)

// mentions lists the handles text names, lowercased, each once, in order.
func mentions(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mentionPattern.FindAllStringSubmatch(text, -1) {
		h := strings.ToLower(strings.TrimRight(m[1], ".-_"))
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// newMentions lists the handles text names that earlier did not.
func newMentions(text, earlier string) []string {
	old := map[string]bool{}
	for _, h := range mentions(earlier) {
		old[h] = true
	}
	var out []string
	for _, h := range mentions(text) {
		if !old[h] {
			out = append(out, h)
		}
	}
	return out
}

// excerpt is text on one line, cut to MaxInboxExcerpt runes.
func excerpt(text string) string {
	line := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(line) <= core.MaxInboxExcerpt {
		return line
	}
	runes := []rune(line)
	return strings.TrimSpace(string(runes[:core.MaxInboxExcerpt-1])) + "…"
}

// ListInbox returns one page of the caller's items, newest first.
func (l *Local) ListInbox(ctx context.Context, f core.InboxFilter) ([]core.InboxItem, string, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	f.Page.Sort, f.Page.Direction = core.SortCreatedAt, core.Descending
	page, err := f.Page.Normalize()
	if err != nil {
		return nil, "", err
	}
	if err := mustCursorMatch(page); err != nil {
		return nil, "", err
	}
	f.Page = page
	out := []core.InboxItem{}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		items, err := tx.ListInbox(ctx, actor.ID, f)
		if err != nil {
			return err
		}
		out, err = nameInboxItems(ctx, tx, items)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return out, nextInboxCursor(page, out), nil
}

// nextInboxCursor returns the cursor for the page after these items.
func nextInboxCursor(page core.Page, items []core.InboxItem) string {
	if len(items) == 0 || len(items) < page.Limit {
		return ""
	}
	last := items[len(items)-1]
	return core.Cursor{
		SortValue: sqlb.TimeText(last.CreatedAt),
		ID:        last.ID,
		Sort:      page.Sort,
		Direction: page.Direction,
	}.Encode()
}

// nameInboxItems fills in each item's task reference and title and the
// handle of the actor who caused it, one lookup per distinct task and actor.
func nameInboxItems(ctx context.Context, tx store.Tx, items []core.InboxItem) ([]core.InboxItem, error) {
	tasks := map[string]*core.Task{}
	handles := map[string]string{systemActorID: systemActorID}
	for i := range items {
		it := &items[i]
		task, ok := tasks[it.TaskID]
		if !ok {
			t, err := tx.GetTask(ctx, core.TaskRef{ID: it.TaskID})
			if err != nil && !core.IsKind(err, core.KindNotFound) {
				return nil, err
			}
			tasks[it.TaskID], task = t, t
		}
		if task != nil {
			it.TaskRef, it.TaskTitle, it.TaskGone = task.Ref, task.Title, task.Deleted()
		} else {
			it.TaskGone = true
		}
		handle, ok := handles[it.ActorID]
		if !ok {
			a, err := tx.GetActor(ctx, it.ActorID)
			if err != nil && !core.IsKind(err, core.KindNotFound) {
				return nil, err
			}
			if a != nil {
				handle = a.Handle
			}
			handles[it.ActorID] = handle
		}
		it.ActorHandle = handle
	}
	return items, nil
}

// CountInbox counts the caller's unread items.
func (l *Local) CountInbox(ctx context.Context) (*core.InboxCount, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := &core.InboxCount{}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		n, err := tx.CountUnreadInbox(ctx, actor.ID)
		out.Unread = n
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReadInboxItem marks one of the caller's items read.
func (l *Local) ReadInboxItem(ctx context.Context, id string) (*core.InboxItem, error) {
	return l.markInboxItem(ctx, id, true)
}

// UnreadInboxItem marks one of the caller's items unread.
func (l *Local) UnreadInboxItem(ctx context.Context, id string) (*core.InboxItem, error) {
	return l.markInboxItem(ctx, id, false)
}

func (l *Local) markInboxItem(ctx context.Context, id string, read bool) (*core.InboxItem, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, core.Invalid("inbox item identifier is required")
	}
	var out *core.InboxItem
	err = l.write(ctx, actor, func(m *mutation) error {
		item, err := m.tx.GetInboxItem(ctx, actor.ID, id)
		if err != nil {
			return err
		}
		before := *item
		at, action := &m.now, auditInboxRead
		if !read {
			at, action = nil, auditInboxUnread
		}
		changed, err := m.tx.SetInboxRead(ctx, actor.ID, id, at)
		if err != nil {
			return err
		}
		item.ReadAt = at
		named, err := nameInboxItems(ctx, m.tx, []core.InboxItem{*item})
		if err != nil {
			return err
		}
		out = &named[0]
		if !changed {
			return nil
		}
		return m.Record(action, core.EventInboxUpdated, inboxSubject, item.ID, "", before, *item,
			map[string]any{"recipients": []string{actor.ID}, "read": read, "task_id": item.TaskID, "ref": out.TaskRef})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReadAllInbox marks every one of the caller's items read.
func (l *Local) ReadAllInbox(ctx context.Context) (*core.InboxReadAll, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := &core.InboxReadAll{}
	err = l.write(ctx, actor, func(m *mutation) error {
		n, err := m.tx.ReadAllInbox(ctx, actor.ID, m.now)
		if err != nil || n == 0 {
			return err
		}
		out.Marked = int(n)
		return m.Record(auditInboxReadAll, core.EventInboxUpdated, inboxSubject, actor.ID, "", nil,
			map[string]any{"marked": n}, map[string]any{"recipients": []string{actor.ID}, "read": true, "marked": n})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetInboxPrefs returns the kinds the caller muted.
func (l *Local) GetInboxPrefs(ctx context.Context) (*core.InboxPrefs, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.InboxPrefs
	err = l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		out, err = inboxPrefs(ctx, tx, actor.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PutInboxPrefs replaces the kinds the caller muted. A set equal to the one
// held writes nothing.
func (l *Local) PutInboxPrefs(ctx context.Context, in core.InboxPrefsInput) (*core.InboxPrefs, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	kinds, err := core.ParseInboxKinds(in.Muted)
	if err != nil {
		return nil, err
	}
	after := &core.InboxPrefs{Muted: canonicalKinds(kinds)}
	err = l.write(ctx, actor, func(m *mutation) error {
		before, err := inboxPrefs(ctx, m.tx, actor.ID)
		if err != nil {
			return err
		}
		if sameKinds(before.Muted, after.Muted) {
			return nil
		}
		if err := m.tx.SetInboxMutes(ctx, actor.ID, after.Muted); err != nil {
			return err
		}
		return m.Record(auditInboxPrefs, core.EventInboxUpdated, inboxSubject, actor.ID, "", before, after,
			map[string]any{"recipients": []string{actor.ID}, "muted": after.Muted})
	})
	if err != nil {
		return nil, err
	}
	return after, nil
}

func inboxPrefs(ctx context.Context, tx store.Tx, actorID string) (*core.InboxPrefs, error) {
	mutes, err := tx.InboxMutesOf(ctx, []string{actorID})
	if err != nil {
		return nil, err
	}
	return &core.InboxPrefs{Muted: canonicalKinds(mutes[actorID])}, nil
}

// canonicalKinds orders kinds strongest first, so two equal sets compare equal.
func canonicalKinds(kinds []core.InboxKind) []core.InboxKind {
	out := []core.InboxKind{}
	for _, k := range core.InboxKinds() {
		for _, have := range kinds {
			if have == k {
				out = append(out, k)
				break
			}
		}
	}
	return out
}

func sameKinds(a, b []core.InboxKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
