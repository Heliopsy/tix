// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"slices"
	"strings"
	"time"
)

// InboxKind names why an inbox item reached its recipient.
type InboxKind string

// Inbox kinds, strongest first: when one change concerns a recipient for
// several reasons, the item carries the first of these that applies.
const (
	// InboxMention is a comment or a description naming the recipient as @handle.
	InboxMention InboxKind = "mention"
	// InboxAssigned is a task assigned to the recipient.
	InboxAssigned InboxKind = "assigned"
	// InboxStatus is a task assigned to the recipient changing status.
	InboxStatus InboxKind = "status"
	// InboxComment is a comment on a task the recipient is assigned to or starred.
	InboxComment InboxKind = "comment"
)

// inboxKinds is the kind vocabulary, strongest first.
var inboxKinds = []InboxKind{InboxMention, InboxAssigned, InboxStatus, InboxComment}

// InboxKinds returns every inbox kind, strongest first.
func InboxKinds() []InboxKind { return slices.Clone(inboxKinds) }

// Valid reports whether k is a known kind.
func (k InboxKind) Valid() bool { return slices.Contains(inboxKinds, k) }

// Rank orders kinds by strength: a lower rank wins.
func (k InboxKind) Rank() int {
	if i := slices.Index(inboxKinds, k); i >= 0 {
		return i
	}
	return len(inboxKinds)
}

// ParseInboxKinds reads kind names, refusing an unknown one and dropping
// repeats, in the order given.
func ParseInboxKinds(names []string) ([]InboxKind, error) {
	out := []InboxKind{}
	for _, name := range names {
		k := InboxKind(strings.ToLower(strings.TrimSpace(name)))
		if k == "" {
			continue
		}
		if !k.Valid() {
			return nil, Invalid("unknown inbox kind %q: use mention, assigned, status or comment", name).
				WithDetail(DetailField, "muted")
		}
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out, nil
}

// MaxInboxExcerpt bounds the text an item quotes from a comment or description.
const MaxInboxExcerpt = 200

// InboxItem is one thing that concerned its recipient: a mention, an
// assignment, a comment on a task they follow, or a status change.
type InboxItem struct {
	ID          string    `json:"id" yaml:"id"`
	RecipientID string    `json:"recipient_id" yaml:"recipient_id"`
	Kind        InboxKind `json:"kind" yaml:"kind"`
	TaskID      string    `json:"task_id" yaml:"task_id"`
	// TaskRef and TaskTitle name the task when the item was read back.
	TaskRef   string `json:"task_ref,omitempty" yaml:"task_ref,omitempty"`
	TaskTitle string `json:"task_title,omitempty" yaml:"task_title,omitempty"`
	// TaskGone is set when the task has since been deleted.
	TaskGone    bool   `json:"task_gone,omitempty" yaml:"task_gone,omitempty"`
	ActorID     string `json:"actor_id" yaml:"actor_id"`
	ActorHandle string `json:"actor_handle,omitempty" yaml:"actor_handle,omitempty"`
	CommentID   string `json:"comment_id,omitempty" yaml:"comment_id,omitempty"`
	// FromStatus and ToStatus are set on a status item.
	FromStatus string     `json:"from_status,omitempty" yaml:"from_status,omitempty"`
	ToStatus   string     `json:"to_status,omitempty" yaml:"to_status,omitempty"`
	Excerpt    string     `json:"excerpt,omitempty" yaml:"excerpt,omitempty"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	ReadAt     *time.Time `json:"read_at,omitempty" yaml:"read_at,omitempty"`
}

// Read reports whether the recipient has read the item.
func (i InboxItem) Read() bool { return i.ReadAt != nil }

// InboxFilter selects a page of the caller's own inbox, newest first.
type InboxFilter struct {
	Unread bool `json:"unread,omitempty" yaml:"unread,omitempty"`
	Page   Page `json:"page,omitempty" yaml:"page,omitempty"`
}

// InboxCount is how many of the caller's items are unread.
type InboxCount struct {
	Unread int `json:"unread" yaml:"unread"`
}

// InboxReadAll reports how many items marking everything read changed.
type InboxReadAll struct {
	Marked int `json:"marked" yaml:"marked"`
}

// InboxPrefs is the caller's choice of kinds that create no item for them.
type InboxPrefs struct {
	Muted []InboxKind `json:"muted" yaml:"muted"`
}

// Mutes reports whether the kind is muted.
func (p InboxPrefs) Mutes(k InboxKind) bool { return slices.Contains(p.Muted, k) }

// InboxPrefsInput replaces the caller's muted kinds.
type InboxPrefsInput struct {
	Muted []string `json:"muted" yaml:"muted"`
}

// InboxService covers each actor's own inbox.
type InboxService interface {
	// ListInbox returns one page of the caller's items, newest first, and
	// the cursor for the next.
	ListInbox(ctx context.Context, f InboxFilter) ([]InboxItem, string, error)
	// CountInbox counts the caller's unread items.
	CountInbox(ctx context.Context) (*InboxCount, error)
	// ReadInboxItem marks one of the caller's items read.
	ReadInboxItem(ctx context.Context, id string) (*InboxItem, error)
	// UnreadInboxItem marks one of the caller's items unread.
	UnreadInboxItem(ctx context.Context, id string) (*InboxItem, error)
	// ReadAllInbox marks every one of the caller's items read.
	ReadAllInbox(ctx context.Context) (*InboxReadAll, error)
	// GetInboxPrefs returns the kinds the caller muted.
	GetInboxPrefs(ctx context.Context) (*InboxPrefs, error)
	// PutInboxPrefs replaces the kinds the caller muted.
	PutInboxPrefs(ctx context.Context, in InboxPrefsInput) (*InboxPrefs, error)
}
