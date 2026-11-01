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

// ChannelFormat names how a personal channel's messages are shaped.
type ChannelFormat string

// Channel formats.
const (
	// ChannelGeneric is signed JSON carrying the item and a message.
	ChannelGeneric ChannelFormat = "generic"
	// ChannelSlack is a Slack incoming webhook.
	ChannelSlack ChannelFormat = "slack"
	// ChannelDiscord is a Discord webhook.
	ChannelDiscord ChannelFormat = "discord"
	// ChannelNtfy is an ntfy topic URL.
	ChannelNtfy ChannelFormat = "ntfy"
)

// channelFormats is the format vocabulary, in the order surfaces offer it.
var channelFormats = []ChannelFormat{ChannelGeneric, ChannelSlack, ChannelDiscord, ChannelNtfy}

// ChannelFormats returns every channel format.
func ChannelFormats() []ChannelFormat { return slices.Clone(channelFormats) }

// Valid reports whether f is a known format.
func (f ChannelFormat) Valid() bool { return slices.Contains(channelFormats, f) }

// Channel limits.
const (
	MaxChannelName       = 80
	MaxChannelsPerPerson = 20
)

// InboxChannel is one person's own delivery endpoint for their inbox items:
// a URL tix posts to, shaped for the service behind it, for the kinds the
// person chose. It is not a tenant webhook and nobody else's items reach it.
type InboxChannel struct {
	ID        string        `json:"id" yaml:"id"`
	ActorID   string        `json:"actor_id" yaml:"actor_id"`
	Name      string        `json:"name" yaml:"name"`
	URL       string        `json:"url" yaml:"url"`
	Secret    string        `json:"-" yaml:"-"`
	Format    ChannelFormat `json:"format" yaml:"format"`
	Kinds     []InboxKind   `json:"kinds" yaml:"kinds"`
	Active    bool          `json:"active" yaml:"active"`
	CreatedAt time.Time     `json:"created_at" yaml:"created_at"`
}

// Receives reports whether the channel takes items of a kind.
func (c InboxChannel) Receives(k InboxKind) bool { return c.Active && slices.Contains(c.Kinds, k) }

// IssuedInboxChannel is a channel as it is created: the only answer that
// carries its signing secret, which no later read returns.
type IssuedInboxChannel struct {
	InboxChannel
	SigningSecret string `json:"secret" yaml:"secret"`
}

// InboxChannelInput creates a channel. Kinds empty means every kind.
type InboxChannelInput struct {
	Name   string   `json:"name" yaml:"name"`
	URL    string   `json:"url" yaml:"url"`
	Format string   `json:"format,omitempty" yaml:"format,omitempty"`
	Kinds  []string `json:"kinds,omitempty" yaml:"kinds,omitempty"`
	Active *bool    `json:"active,omitempty" yaml:"active,omitempty"`
}

// InboxChannelUpdate changes a channel. A nil field is left as it is.
type InboxChannelUpdate struct {
	Name   *string   `json:"name,omitempty" yaml:"name,omitempty"`
	URL    *string   `json:"url,omitempty" yaml:"url,omitempty"`
	Format *string   `json:"format,omitempty" yaml:"format,omitempty"`
	Kinds  *[]string `json:"kinds,omitempty" yaml:"kinds,omitempty"`
	Active *bool     `json:"active,omitempty" yaml:"active,omitempty"`
}

// ChannelDelivery is one attempt queue entry for a channel: an inbox item,
// or a test when ItemID is empty.
type ChannelDelivery struct {
	ID             string         `json:"id" yaml:"id"`
	ChannelID      string         `json:"channel_id" yaml:"channel_id"`
	ItemID         string         `json:"item_id,omitempty" yaml:"item_id,omitempty"`
	Attempts       int            `json:"attempts" yaml:"attempts"`
	NextAttemptAt  time.Time      `json:"next_attempt_at" yaml:"next_attempt_at"`
	Status         DeliveryStatus `json:"status" yaml:"status"`
	LastError      string         `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	LastStatusCode int            `json:"last_status_code,omitempty" yaml:"last_status_code,omitempty"`
	CreatedAt      time.Time      `json:"created_at" yaml:"created_at"`
}

// Test reports whether the delivery is a test rather than an inbox item.
func (d ChannelDelivery) Test() bool { return d.ItemID == "" }

// ChannelDeliveryFilter selects a page of one channel's deliveries, newest first.
type ChannelDeliveryFilter struct {
	ChannelID string `json:"channel_id" yaml:"channel_id"`
	Page      Page   `json:"page,omitempty" yaml:"page,omitempty"`
}

// InboxChannelService covers each person's own notification channels.
type InboxChannelService interface {
	// ListInboxChannels returns the caller's channels.
	ListInboxChannels(ctx context.Context) ([]InboxChannel, error)
	// CreateInboxChannel adds a channel and returns its secret, once.
	CreateInboxChannel(ctx context.Context, in InboxChannelInput) (*IssuedInboxChannel, error)
	// UpdateInboxChannel changes one of the caller's channels.
	UpdateInboxChannel(ctx context.Context, id string, in InboxChannelUpdate) (*InboxChannel, error)
	// DeleteInboxChannel removes one of the caller's channels and its deliveries.
	DeleteInboxChannel(ctx context.Context, id string) error
	// TestInboxChannel queues a test message to one of the caller's channels
	// and returns the delivery, attempted when this process drains inline.
	TestInboxChannel(ctx context.Context, id string) (*ChannelDelivery, error)
	// ListChannelDeliveries returns a page of one channel's deliveries.
	ListChannelDeliveries(ctx context.Context, f ChannelDeliveryFilter) ([]ChannelDelivery, string, error)
	// RedeliverChannelDelivery queues another attempt of a delivery.
	RedeliverChannelDelivery(ctx context.Context, id string) error
}
