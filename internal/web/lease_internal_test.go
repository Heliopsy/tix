// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// at is a pointer to an instant offset from now, for building the lease
// columns a task carries. Every instant here is relative to the clock rather
// than a written-down date, so the host's zone cannot decide the result.
func at(d time.Duration) *time.Time {
	t := time.Now().Add(d)
	return &t
}

// The badge used to be read off the lease columns still being populated past
// their time, which the sweeper clears within a minute, so "claim expired"
// was all but unreachable. It reads the durable evidence now.
func TestClaimStateReadsTheEvidenceRatherThanTheSweptColumns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		task core.Task
		want string
	}{
		{"nobody has touched it", core.Task{}, ""},
		{"an agent holds a live lease",
			core.Task{ClaimedByActorID: "a1", LeaseExpiresAt: at(time.Hour)}, "held"},
		{"a claim with no lease at all never lapses",
			core.Task{ClaimedByActorID: "a1"}, "held"},
		{"the lease has run out but nothing has swept it yet",
			core.Task{ClaimedByActorID: "a1", LeaseExpiresAt: at(-time.Minute),
				LeaseExpiredAt: at(-time.Minute)}, "expired"},
		{"the sweeper has cleared the columns and left its evidence",
			core.Task{LeaseExpiredAt: at(-3 * time.Hour), LeaseExpiredByActorID: "a1"}, "expired"},
		{"the evidence has aged out of the window",
			core.Task{LeaseExpiredAt: at(-core.LeaseExpiryEvidenceWindow - time.Hour),
				LeaseExpiredByActorID: "a1"}, ""},
		{"somebody claimed it again after the expiry",
			core.Task{ClaimedByActorID: "a2", LeaseExpiresAt: at(time.Hour),
				LeaseExpiredAt: at(-time.Hour), LeaseExpiredByActorID: "a1"}, "held"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := claimState(c.task); got != c.want {
				t.Errorf("claimState = %q, want %q", got, c.want)
			}
		})
	}
}

// The badge says when, not only that. "claim expired" alone leaves a reader
// unable to tell a lease that lapsed a minute ago from one that lapsed
// yesterday, which is the whole of the question it exists to answer.
func TestExpiredAgoRendersHowLongAgoTheClaimLapsed(t *testing.T) {
	t.Parallel()
	if got := expiredAgo(core.Task{}); got != "" {
		t.Errorf("expiredAgo of a task that never lapsed = %q, want empty", got)
	}
	got := expiredAgo(core.Task{LeaseExpiredAt: at(-3 * time.Hour)})
	if got != "3h" {
		t.Errorf("expiredAgo = %q, want %q", got, "3h")
	}
}

// The value is the contract and must not move; only the words a reader sees
// are decided here.
func TestCategoryLabelsNameTheCategoryWithoutRenamingIt(t *testing.T) {
	t.Parallel()
	cases := map[core.StateCategory]string{
		core.CategoryTodo:                   "To do",
		core.CategoryInProgress:             "In progress",
		core.CategoryDone:                   "Done",
		core.StateCategory("something_new"): "something_new",
	}
	for category, want := range cases {
		if got := categoryLabel(category); got != want {
			t.Errorf("categoryLabel(%q) = %q, want %q", category, got, want)
		}
	}
	if core.CategoryInProgress != "in_progress" {
		t.Fatal("the category value moved; it is the wire and store contract, not a label")
	}
}

// A lease running out is the one history entry nobody performed. It shared
// the "other" mark with every action this build has no opinion on.
func TestLeaseExpiryIsItsOwnKindOfHistoryEntry(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"task.lease_expire": "expire",
		"task.create":       "create",
		"task.update":       "update",
		"task.delete":       "delete",
		"task.claim":        "other",
	}
	for action, want := range cases {
		if got := actionVerb(action); got != want {
			t.Errorf("actionVerb(%q) = %q, want %q", action, got, want)
		}
	}
}
