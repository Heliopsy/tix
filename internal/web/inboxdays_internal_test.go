// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestInboxDaysAreCountedInTheReadersZone(t *testing.T) {
	zone := time.FixedZone("plus3", 3*3600)
	now := time.Date(2026, 11, 12, 1, 0, 0, 0, zone)
	at := func(d time.Duration) inboxRow { return inboxRow{InboxItem: core.InboxItem{CreatedAt: now.Add(-d).UTC()}} }
	rows := []inboxRow{at(30 * time.Minute), at(90 * time.Minute), at(25 * time.Hour), at(26 * time.Hour), at(50 * time.Hour)}
	groups := groupInboxByDay(rows, now, zone)
	var got []string
	for _, g := range groups {
		got = append(got, g.Key+":"+string(rune('0'+len(g.Items))))
	}
	want := []string{"today:1", "yesterday:2", "earlier:2"}
	if len(got) != len(want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("groups = %v, want %v", got, want)
		}
	}
}
