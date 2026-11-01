// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// The inbox of the fake: one reader's items, newest first, held in memory.

func (f *fakeService) ListInbox(_ context.Context, filter core.InboxFilter) ([]core.InboxItem, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inboxReads++
	if f.inboxErr != nil {
		return nil, "", f.inboxErr
	}
	out := []core.InboxItem{}
	for _, it := range f.inbox {
		if filter.Unread && it.Read() {
			continue
		}
		out = append(out, it)
	}
	return out, "", nil
}

func (f *fakeService) CountInbox(context.Context) (*core.InboxCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, it := range f.inbox {
		if !it.Read() {
			n++
		}
	}
	return &core.InboxCount{Unread: n}, nil
}

func (f *fakeService) markInbox(id string, read bool) (*core.InboxItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.inbox {
		if f.inbox[i].ID != id {
			continue
		}
		f.inbox[i].ReadAt = nil
		if read {
			now := time.Unix(0, 0)
			f.inbox[i].ReadAt = &now
		}
		it := f.inbox[i]
		return &it, nil
	}
	return nil, core.NotFound("inbox item %q", id)
}

func (f *fakeService) ReadInboxItem(_ context.Context, id string) (*core.InboxItem, error) {
	return f.markInbox(id, true)
}

func (f *fakeService) UnreadInboxItem(_ context.Context, id string) (*core.InboxItem, error) {
	return f.markInbox(id, false)
}

func (f *fakeService) ReadAllInbox(context.Context) (*core.InboxReadAll, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := &core.InboxReadAll{}
	now := time.Unix(0, 0)
	for i := range f.inbox {
		if !f.inbox[i].Read() {
			f.inbox[i].ReadAt = &now
			out.Marked++
		}
	}
	return out, nil
}

func (f *fakeService) GetInboxPrefs(context.Context) (*core.InboxPrefs, error) {
	return &core.InboxPrefs{Muted: []core.InboxKind{}}, nil
}

func (f *fakeService) PutInboxPrefs(_ context.Context, in core.InboxPrefsInput) (*core.InboxPrefs, error) {
	kinds, err := core.ParseInboxKinds(in.Muted)
	return &core.InboxPrefs{Muted: kinds}, err
}
