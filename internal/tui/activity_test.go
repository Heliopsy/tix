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
	quiet := ActivityEmptyState(0, 0, true)
	if quiet.Zero() || !strings.Contains(quiet.Title+quiet.Hint, "Waiting") {
		t.Fatalf("quiet state = %+v, want it to say it is waiting", quiet)
	}
	dropped := ActivityEmptyState(0, 0, false)
	if dropped.Zero() || !strings.Contains(dropped.Title, "connect") {
		t.Fatalf("disconnected state = %+v, want it to say the stream is not connected", dropped)
	}
	if got := ActivityEmptyState(3, 3, false); !got.Zero() {
		t.Fatalf("state with events present = %+v, want it to say nothing", got)
	}
	filtered := ActivityEmptyState(0, 9, true)
	if filtered.Zero() || !strings.Contains(filtered.Title, "filter") {
		t.Fatalf("filtered-out state = %+v, want it to blame the filter", filtered)
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

// openFilteredActivity opens the activity view over a scripted tail and types
// an expression into its filter bar.
func openFilteredActivity(t *testing.T, expr string, events ...core.Event) Model {
	t.Helper()
	m := boardModel(t)
	for _, e := range events {
		m, _ = m.reduce(eventMsg{event: e})
	}
	m, _ = m.reduce(pressKey("v"))
	m, _ = m.reduce(pressKey("/"))
	if m.prompt != promptActivityFilter {
		t.Fatalf("prompt = %v, want the activity filter bar", m.prompt)
	}
	m = typeText(m, expr)
	m, _ = m.reduce(pressKey("enter"))
	return m
}

// projectEvent builds an event about something other than a task, so a kind
// term has two kinds to choose between.
func projectEvent(seq int64, actor string) core.Event {
	return core.Event{
		Seq: seq, Type: core.EventProjectCreated, ActorID: actor, SubjectType: "project",
		SubjectID:  "prj_" + strconv.FormatInt(seq, 10),
		OccurredAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		Payload:    map[string]any{"name": "Infrastructure"},
	}
}

// taskEvent is activityEvent with the subject type the service records, which
// is what a kind term selects on.
func taskEvent(seq int64, actor string) core.Event {
	e := activityEvent(seq, actor)
	e.SubjectType = "task"
	return e
}

func TestEventRowCarriesWhatTheActivityFilterAsksAbout(t *testing.T) {
	t.Parallel()
	row := EventRow(taskEvent(4, "u1"))
	if row.Kind != "task" || row.Action != string(core.EventTaskClaimed) {
		t.Fatalf("EventRow lost a field: %+v", row)
	}
	joined := strings.Join(row.Text, " ")
	for _, want := range []string{"infra-4", "claimed"} {
		if !strings.Contains(joined, want) {
			t.Errorf("EventRow text %q does not carry %q", joined, want)
		}
	}
}

func TestActivityFilterNarrowsTheTail(t *testing.T) {
	events := []core.Event{taskEvent(1, "alice"), projectEvent(2, "bob"), taskEvent(3, "bob")}
	for _, tc := range []struct {
		name, expr string
		want       []int64
	}{
		{"by kind", "kind:task", []int64{1, 3}},
		{"by actor", "actor:bob", []int64{2, 3}},
		{"by event type", "type:project.created", []int64{2}},
		{"by text", "infra-3", []int64{3}},
		{"negated", "-kind:project", []int64{1, 3}},
		{"conjunction", "kind:task actor:bob", []int64{3}},
		{"nothing matches", "kind:webhook", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := openFilteredActivity(t, tc.expr, events...)
			if m.activityFilterErr != "" {
				t.Fatalf("filter %q was refused: %s", tc.expr, m.activityFilterErr)
			}
			var got []int64
			for _, e := range m.shownActivity() {
				got = append(got, e.Seq)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("filter %q kept %v, want %v", tc.expr, got, tc.want)
			}
			for i, seq := range tc.want {
				if got[i] != seq {
					t.Fatalf("filter %q kept %v, want %v", tc.expr, got, tc.want)
				}
			}
			if len(m.activity) != len(events) {
				t.Fatalf("the filter dropped events from the tail itself: %d kept", len(m.activity))
			}
		})
	}
}

// An event carries no source, so a source term has to be refused by name
// rather than quietly selecting nothing.
func TestActivityFilterRefusesTermsAnEventCannotAnswer(t *testing.T) {
	m := openFilteredActivity(t, "source:web", taskEvent(1, "alice"))
	if m.activityFilterErr == "" {
		t.Fatal("a source term was accepted on the live tail")
	}
	for _, want := range []string{"source", "tix audit ls"} {
		if !strings.Contains(m.activityFilterErr, want) {
			t.Errorf("refusal %q does not mention %q", m.activityFilterErr, want)
		}
	}
	if len(m.shownActivity()) != 1 {
		t.Fatal("a refused filter still narrowed the tail")
	}
	if !strings.Contains(m.View(), "activity filter error") {
		t.Fatal("the refusal is not shown on the status bar")
	}
}

func TestABadActivityFilterKeepsTheOneInForce(t *testing.T) {
	m := openFilteredActivity(t, "kind:project", taskEvent(1, "alice"), projectEvent(2, "bob"))
	m, _ = m.reduce(pressKey("/"))
	m = typeText(m, " nonsense:1")
	m, _ = m.reduce(pressKey("enter"))
	if m.activityFilterErr == "" {
		t.Fatal("an unknown key was accepted")
	}
	if m.activityFilterText != "kind:project" {
		t.Fatalf("filter text = %q, want the expression still in force", m.activityFilterText)
	}
	if got := len(m.shownActivity()); got != 1 {
		t.Fatalf("shown = %d, want the previous filter still applied", got)
	}
}

func TestClearingTheActivityFilterRestoresTheTail(t *testing.T) {
	m := openFilteredActivity(t, "kind:project", taskEvent(1, "alice"), projectEvent(2, "bob"))
	if len(m.shownActivity()) != 1 {
		t.Fatalf("the filter did not narrow the tail")
	}
	m, _ = m.reduce(pressKey("C"))
	if m.activityFilterText != "" || len(m.shownActivity()) != 2 {
		t.Fatalf("C left filter %q showing %d events", m.activityFilterText, len(m.shownActivity()))
	}
}

func TestTheActivityBarSaysHowMuchItIsHiding(t *testing.T) {
	m := openFilteredActivity(t, "kind:task", taskEvent(1, "alice"), projectEvent(2, "bob"))
	if got := m.activityCount(); !strings.Contains(got, "1 of 2 events match") {
		t.Fatalf("status = %q, want it to count what the filter keeps", got)
	}
	m, _ = m.reduce(pressKey("C"))
	if got := m.activityCount(); !strings.Contains(got, "2 events kept") {
		t.Fatalf("unfiltered status = %q, want the plain count", got)
	}
	if !strings.Contains(m.View(), "2 events kept") {
		t.Fatal("the status bar does not carry the count")
	}
}

// The selection indexes the filtered tail, so an event the filter hides can
// never pull it off the line the reader is on.
func TestActivitySelectionStaysInsideTheFilteredTail(t *testing.T) {
	m := openFilteredActivity(t, "kind:task", taskEvent(1, "alice"))
	if m.activitySel != 0 {
		t.Fatalf("selection = %d, want the only matching line", m.activitySel)
	}
	m, _ = m.reduce(eventMsg{event: projectEvent(2, "bob")})
	if m.activitySel != 0 || len(m.shownActivity()) != 1 {
		t.Fatalf("a hidden event moved the selection to %d of %d", m.activitySel, len(m.shownActivity()))
	}
	m, _ = m.reduce(eventMsg{event: taskEvent(3, "bob")})
	if m.activitySel != 1 {
		t.Fatalf("selection = %d, want it to follow the new matching line", m.activitySel)
	}
	m, _ = m.reduce(pressKey("v"))
	m, _ = m.reduce(pressKey("/"))
	m = typeText(m, "kind:webhook")
	m, _ = m.reduce(pressKey("enter"))
	if m.activitySel != 0 {
		t.Fatalf("selection = %d after everything was filtered out, want 0", m.activitySel)
	}
	if !strings.Contains(m.View(), "No event matches the filter") {
		t.Fatal("a tail with nothing left does not blame the filter")
	}
}

func TestActivityHelpNamesTheFilterKeys(t *testing.T) {
	m := openFilteredActivity(t, "kind:task", taskEvent(1, "alice"))
	m, _ = m.reduce(pressKey("?"))
	frame := m.View()
	for _, want := range []string{"activity filter accepts", "kind", "actor", "source"} {
		if !strings.Contains(frame, want) {
			t.Errorf("help does not name %q", want)
		}
	}
}
