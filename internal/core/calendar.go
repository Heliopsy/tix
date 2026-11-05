// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"strings"
	"time"
)

// MaxCalendarFilterLength bounds the filter expression a feed stores.
const MaxCalendarFilterLength = 1000

// MaxCalendarEntries bounds how many tasks one feed document carries.
const MaxCalendarEntries = 2000

// CalendarFeed is one actor's subscribable feed of due dates. The secret that
// addresses it is never part of it: only IssuedCalendarFeed carries that, and
// only on the call that minted it.
type CalendarFeed struct {
	// ID identifies the feed.
	ID string `json:"id" yaml:"id"`
	// TenantID identifies the tenant the feed reads.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ActorID is the feed's owner, whose access it reads with.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// Filter is the task filter expression the feed selects with. Empty means
	// the tasks assigned to the owner that are not finished.
	Filter string `json:"filter" yaml:"filter"`
	// CreatedAt is when this secret was issued, RFC 3339.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// RevokedAt is when the feed stopped answering, RFC 3339, absent while it is live.
	RevokedAt *time.Time `json:"revoked_at,omitempty" yaml:"revoked_at,omitempty"`
}

// IssuedCalendarFeed carries a feed together with its freshly minted secret.
// Path is the route the secret is read at; URL is that route made absolute by
// a surface that knows the address it was reached at, and is empty otherwise.
type IssuedCalendarFeed struct {
	CalendarFeed
	// Token is the secret that addresses the feed, returned only by the call
	// that minted it.
	Token string `json:"token" yaml:"token"`
	// Path is the route a calendar app reads the feed at, with no credential:
	// /api/v1/calendar/feeds/TOKEN.ics.
	Path string `json:"path" yaml:"path"`
	// URL is Path made absolute, empty when the surface does not know its own address.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

// CalendarFeedInput sets a feed's filter.
type CalendarFeedInput struct {
	// Filter is a task filter expression of at most 1000 characters. Empty
	// selects the caller's assigned tasks that are not finished.
	Filter string `json:"filter" yaml:"filter"`
}

// Normalize trims the filter and checks its length.
func (in CalendarFeedInput) Normalize() (CalendarFeedInput, error) {
	in.Filter = strings.TrimSpace(in.Filter)
	if len(in.Filter) > MaxCalendarFilterLength {
		return in, Invalid("a calendar filter may be at most %d characters", MaxCalendarFilterLength).
			WithDetail(DetailField, "filter")
	}
	return in, nil
}

// CalendarEntry is one task a feed draws, with what its workflow says about
// the state it is in.
type CalendarEntry struct {
	// Task is the task drawn.
	Task Task `json:"task" yaml:"task"`
	// Category is the category of the task's state in its own workflow.
	Category StateCategory `json:"category" yaml:"category"`
	// State is the state's label, falling back to its key.
	State string `json:"state" yaml:"state"`
}

// CalendarEntries is what a feed answers: whose it is and the tasks it draws,
// soonest due last.
type CalendarEntries struct {
	// Feed is the feed answered.
	Feed CalendarFeed `json:"feed" yaml:"feed"`
	// OwnerHandle is the handle of the feed's owner.
	OwnerHandle string `json:"owner_handle" yaml:"owner_handle"`
	// Entries are the tasks with a due date the feed selects, at most 2000.
	Entries []CalendarEntry `json:"entries" yaml:"entries"`
}

// CalendarService manages the caller's calendar feed and answers a feed by its
// secret.
type CalendarService interface {
	// GetCalendarFeed returns the caller's live feed, without its secret.
	// It is not_found when the caller has none.
	GetCalendarFeed(ctx context.Context) (*CalendarFeed, error)
	// CreateCalendarFeed issues the caller a feed. One that is live already
	// is a conflict.
	CreateCalendarFeed(ctx context.Context, in CalendarFeedInput) (*IssuedCalendarFeed, error)
	// RotateCalendarFeed replaces the caller's feed secret, keeping the filter.
	// The old address stops answering in the same transaction and the new
	// secret is returned once; not_found when there is no live feed.
	RotateCalendarFeed(ctx context.Context) (*IssuedCalendarFeed, error)
	// SetCalendarFeedFilter changes the caller's feed filter, keeping its URL.
	SetCalendarFeedFilter(ctx context.Context, in CalendarFeedInput) (*CalendarFeed, error)
	// RevokeCalendarFeed stops the caller's feed, so its address answers
	// not_found from then on.
	RevokeCalendarFeed(ctx context.Context) error
	// ReadCalendarFeed answers the feed a secret addresses, reading as its
	// owner. It needs a tenant in the context and no actor, and every failure
	// to find a live feed is the same not found.
	ReadCalendarFeed(ctx context.Context, token string) (*CalendarEntries, error)
}
