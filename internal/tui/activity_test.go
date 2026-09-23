// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// activityEvent builds an event as the CLI's watch command would receive it.
func activityEvent(seq int64, actor string) core.Event {
	return core.Event{
		Seq: seq, Type: core.EventTaskClaimed, ActorID: actor,
		SubjectID:  "tsk_" + strconv.FormatInt(seq, 10),
		OccurredAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		Payload:    map[string]any{"ref": "infra-" + strconv.FormatInt(seq, 10)},
	}
}

// The v key opens the activity view from anywhere, and esc pops it back the
// same way every other view returns.
func TestActivityViewOpensAndPopsBack(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m, _ = m.reduce(pressKey("v"))
	if m.view != viewActivity {
		t.Fatalf("view = %v, want viewActivity", m.view)
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view != viewBoard {
		t.Fatalf("esc from activity landed on %v, want the board it was opened from", m.view)
	}
}

// An event arriving through onEvent is recorded whether or not the activity
// view is open, exactly as the CLI's watch command streams every event
// through core.Service.Subscribe with no business logic of its own.
func TestActivityRecordsEveryEventRegardlessOfTheOpenView(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(eventMsg{event: activityEvent(1, "alice")})
	if len(m.activity) != 1 {
		t.Fatalf("activity = %v, want one recorded event", m.activity)
	}
	if m.view != viewBoard {
		t.Fatalf("recording an event changed the open view to %v", m.view)
	}
}

// The activity ring is capped so a long session cannot grow it without limit,
// and it keeps the newest events rather than the oldest.
func TestActivityIsCappedToTheNewestEvents(t *testing.T) {
	m := New(Config{})
	for i := int64(1); i <= activityCap+10; i++ {
		m, _ = m.reduce(eventMsg{event: activityEvent(i, "alice")})
	}
	if len(m.activity) != activityCap {
		t.Fatalf("len(activity) = %d, want the cap %d", len(m.activity), activityCap)
	}
	if got := m.activity[len(m.activity)-1].Seq; got != activityCap+10 {
		t.Fatalf("newest kept seq = %d, want %d", got, activityCap+10)
	}
	if got := m.activity[0].Seq; got != 11 {
		t.Fatalf("oldest kept seq = %d, want the ring to have dropped everything before 11", got)
	}
}

// Before any event has arrived, the view says so rather than drawing blank
// space that looks like a failure.
func TestActivityEmptyStateDistinguishesQuietFromDisconnected(t *testing.T) {
	quiet := ActivityEmptyState(0, true)
	if quiet.Zero() || !strings.Contains(quiet.Title+quiet.Hint, "Waiting") {
		t.Fatalf("quiet state = %+v, want it to say it is waiting", quiet)
	}
	dropped := ActivityEmptyState(0, false)
	if dropped.Zero() || !strings.Contains(dropped.Title, "connect") {
		t.Fatalf("disconnected state = %+v, want it to say the stream is not connected", dropped)
	}
	if got := ActivityEmptyState(3, false); !got.Zero() {
		t.Fatalf("state with events present = %+v, want it to say nothing", got)
	}
}

// The rendered frame draws a readable line per event: the actor, the verb and
// the task ref all appear, and the same fields the CLI's watch command draws.
func TestActivityViewRendersAReadableLinePerEvent(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(eventMsg{event: activityEvent(5, "alice")})
	m, _ = m.reduce(pressKey("v"))
	frame := m.View()
	for _, want := range []string{"alice", "claimed", "infra-5"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame does not contain %q:\n%s", want, frame)
		}
	}
}

// Up and down move the selection through the tail and keep it on screen,
// using the same ScrollWindow every other scrolling view uses.
func TestActivityViewScrolls(t *testing.T) {
	m := boardModel(t)
	m.height = 10
	for i := int64(1); i <= 30; i++ {
		m, _ = m.reduce(eventMsg{event: activityEvent(i, "alice")})
	}
	m, _ = m.reduce(pressKey("v"))
	if m.activitySel != 29 {
		t.Fatalf("activitySel = %d, want the view to follow the newest event (29)", m.activitySel)
	}
	m, _ = m.reduce(pressKey("up"))
	if m.activitySel != 28 {
		t.Fatalf("activitySel after up = %d, want 28", m.activitySel)
	}
	m, _ = m.reduce(pressKey("g"))
	if m.activitySel != 0 {
		t.Fatalf("activitySel after top = %d, want 0", m.activitySel)
	}
}

// A subscription that drops while events are already on screen is shown
// honestly rather than looking idle: the title bar's connection indicator
// switches to disconnected, and it is visible in the activity view too.
func TestActivityViewShowsDisconnectHonestly(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(eventMsg{event: activityEvent(1, "alice")})
	m, _ = m.reduce(pressKey("v"))
	if !strings.Contains(m.View(), "live") {
		t.Fatalf("connected frame does not say live:\n%s", m.View())
	}
	m, _ = m.reduce(streamMsg{})
	frame := m.View()
	if !strings.Contains(frame, "disconnected") {
		t.Fatalf("frame does not report the dropped stream:\n%s", frame)
	}
}
