// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TestSubscribeResumesWithoutAGap is the property a monitoring agent depends
// on: it consumes events, stops at a known sequence number, more work happens
// while it is not watching, and resuming from that number delivers every event
// committed in between, in order and once each.
//
// It runs on both transports because an agent author will assume they behave
// the same. The direct path replays the durable outbox through the tailer; the
// served path replays the same rows over the WebSocket before switching to
// live delivery. A divergence here is the bug this test exists to catch.
func TestSubscribeResumesWithoutAGap(t *testing.T) {
	t.Parallel()
	for _, remote := range []bool{false, true} {
		name := "direct"
		if remote {
			name = "remote"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newMatrixHarness(t)
			tg := h.localTarget()
			if remote {
				tg = h.remoteTarget()
			}
			project := h.newProject(t, tg)

			// A first watcher, started before anything happens, records where
			// it got to and then stops, exactly as a restarting agent does.
			first, cancelFirst := subscribeFrom(t, tg, 0)
			createTasks(t, tg, project.Key, "before-1", "before-2")
			before := collect(t, first, 2)
			cancelFirst()

			cursor := before[len(before)-1].Seq

			// Work committed while nobody is watching. A stream that only
			// delivered live events would lose all of it.
			createTasks(t, tg, project.Key, "gap-1", "gap-2", "gap-3")

			second, cancelSecond := subscribeFrom(t, tg, cursor)
			defer cancelSecond()
			resumed := collect(t, second, 3)

			assertContiguous(t, tg, cursor, resumed)
			assertTitles(t, tg, resumed, "gap-1", "gap-2", "gap-3")
		})
	}
}

// TestSubscribeRefusesACursorItCannotHonour proves the failure is loud on both
// transports. A cursor above the newest event cannot be resumed from without
// inventing history, and a watcher told nothing would wait forever believing
// it was caught up.
func TestSubscribeRefusesANegativeCursor(t *testing.T) {
	t.Parallel()
	for _, remote := range []bool{false, true} {
		name := "direct"
		if remote {
			name = "remote"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newMatrixHarness(t)
			tg := h.localTarget()
			if remote {
				tg = h.remoteTarget()
			}
			_, err := tg.svc.Subscribe(tg.ctx, core.EventFilter{SinceSeq: -1})
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("[%s] subscribing from a negative cursor = %v, want invalid", tg.name, err)
			}
		})
	}
}

// subscribeFrom opens a subscription resuming after cursor.
func subscribeFrom(t *testing.T, tg target, cursor int64) (<-chan core.Event, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(tg.ctx)
	events, err := tg.svc.Subscribe(ctx, core.EventFilter{
		Types: []core.EventType{core.EventTaskCreated}, SinceSeq: cursor,
	})
	if err != nil {
		cancel()
		t.Fatalf("[%s] subscribing from %d: %v", tg.name, cursor, err)
	}
	return events, cancel
}

// createTasks commits one task per title, in order.
func createTasks(t *testing.T, tg target, projectKey string, titles ...string) {
	t.Helper()
	for _, title := range titles {
		_, err := tg.svc.CreateTask(tg.ctx, core.CreateTaskInput{ProjectRef: projectKey, Title: title})
		mustf(t, tg, err, "creating %q", title)
	}
}

// collect reads exactly n events, failing rather than hanging when the stream
// goes quiet. The deadline is real time on purpose: it bounds a test that has
// genuinely lost an event, and nothing under test reads it as a clock.
func collect(t *testing.T, events <-chan core.Event, n int) []core.Event {
	t.Helper()
	out := make([]core.Event, 0, n)
	deadline := time.After(20 * time.Second)
	for len(out) < n {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatalf("stream closed after %d of %d events", len(out), n)
			}
			out = append(out, e)
		case <-deadline:
			t.Fatalf("timed out after %d of %d events", len(out), n)
		}
	}
	return out
}

// assertContiguous checks the sequence numbers strictly increase and start
// above the cursor the subscription resumed from, which together are what
// "no gap, no duplicate, in order" means for a cursor-resumed stream.
func assertContiguous(t *testing.T, tg target, cursor int64, events []core.Event) {
	t.Helper()
	seen := map[int64]bool{}
	previous := cursor
	for i, e := range events {
		if e.Seq <= cursor {
			t.Errorf("[%s] event %d has seq %d, at or before the resume cursor %d: resuming replayed history it was told to skip",
				tg.name, i, e.Seq, cursor)
		}
		if e.Seq <= previous && i > 0 {
			t.Errorf("[%s] event %d has seq %d after seq %d: the stream went backwards",
				tg.name, i, e.Seq, previous)
		}
		if seen[e.Seq] {
			t.Errorf("[%s] seq %d was delivered twice within one subscription", tg.name, e.Seq)
		}
		seen[e.Seq] = true
		previous = e.Seq
	}
}

// assertTitles checks the events name the tasks created while nobody watched,
// in the order they were committed. This is the gap itself: a stream that
// dropped the events committed between the two subscriptions fails here.
func assertTitles(t *testing.T, tg target, events []core.Event, want ...string) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("[%s] got %d events, want %d", tg.name, len(events), len(want))
	}
	for i, e := range events {
		got, _ := e.Payload["title"].(string)
		if got != want[i] {
			t.Errorf("[%s] event %d is %q, want %q: the resumed stream is missing or reordering events",
				tg.name, i, got, want[i])
		}
	}
}
