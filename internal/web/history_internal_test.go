package web

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// snap builds an audit snapshot the way the service writes one: a JSON
// object with at least a "status" key.
func snap(t *testing.T, fields map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshalling snapshot: %v", err)
	}
	return raw
}

func transitionRow(t *testing.T, seq int64, actor, from, to string, changed []string) historyRow {
	return historyRow{
		Entry: core.AuditEntry{
			Seq: seq, ActorID: actor, Action: "task.transition", SubjectType: "task",
			Before: snap(t, map[string]any{"status": from}),
			After:  snap(t, map[string]any{"status": to}),
		},
		Changed: changed,
	}
}

// A single click on "complete" that has to cross two workflow hops writes two
// audit entries whose statuses chain (todo->doing, doing->done); the history
// view has to fold that into one group, or a one-click action looks like two
// unrelated changes.
func TestGroupHistoryChainsAMultiHopCompleteIntoOneGroup(t *testing.T) {
	rows := []historyRow{
		transitionRow(t, 1, "alice", "todo", "doing", []string{"status", "updated_at", "version"}),
		transitionRow(t, 2, "alice", "doing", "done", []string{"completed_at", "status", "updated_at", "version"}),
	}
	groups := groupHistory(rows)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1: %+v", len(groups), groups)
	}
	if len(groups[0].Hops) != 2 {
		t.Fatalf("hops = %d, want 2", len(groups[0].Hops))
	}
	from, _ := stringField(groups[0].Hops[0].Entry.Before, "status")
	to, _ := stringField(groups[0].Latest().Entry.After, "status")
	if from != "todo" || to != "done" {
		t.Errorf("chain spans %q to %q, want todo to done", from, to)
	}
}

// Two genuinely separate transitions by the same actor, on states that do
// not chain (todo->doing, then later a different, unrelated hop that does
// not start where the first one ended), must stay on their own rows: only a
// matching before/after status justifies a merge.
func TestGroupHistoryDoesNotChainUnrelatedTransitions(t *testing.T) {
	rows := []historyRow{
		transitionRow(t, 1, "alice", "todo", "doing", nil),
		transitionRow(t, 2, "alice", "blocked", "doing", nil), // does not start at "doing"
	}
	groups := groupHistory(rows)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2 (no chain across a status gap): %+v", len(groups), groups)
	}
}

// Completing a task and, much later, reopening it retraces the same path in
// reverse-compatible status pairs; without a time bound that would chain two
// separate clicks into one row. A gap far wider than one request keeps them
// apart even though the statuses connect end to end.
func TestGroupHistoryDoesNotChainHopsFarApartInTime(t *testing.T) {
	early := transitionRow(t, 1, "alice", "todo", "done", nil)
	early.Entry.OccurredAt = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	later := transitionRow(t, 2, "alice", "done", "todo", nil)
	later.Entry.OccurredAt = early.Entry.OccurredAt.Add(time.Hour)

	groups := groupHistory([]historyRow{early, later})
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2 (an hour apart is not one click)", len(groups))
	}
}

// A chain never crosses actors: one person's hop is not folded into
// another's, even if the statuses happen to line up.
func TestGroupHistoryDoesNotChainAcrossActors(t *testing.T) {
	rows := []historyRow{
		transitionRow(t, 1, "alice", "todo", "doing", nil),
		transitionRow(t, 2, "bob", "doing", "done", nil),
	}
	groups := groupHistory(rows)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2 (different actors never chain)", len(groups))
	}
}

// A non-transition action never joins a chain, and never starts one either.
func TestGroupHistoryLeavesNonTransitionActionsUngrouped(t *testing.T) {
	rows := []historyRow{
		{Entry: core.AuditEntry{Seq: 1, ActorID: "alice", Action: "task.create"}},
		transitionRow(t, 2, "alice", "todo", "doing", nil),
		{Entry: core.AuditEntry{Seq: 3, ActorID: "alice", Action: "task.update"}, Changed: []string{"title"}},
	}
	groups := groupHistory(rows)
	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3: %+v", len(groups), groups)
	}
}

func TestSentenceForTransition(t *testing.T) {
	g := historyGroup{Hops: []historyRow{transitionRow(t, 1, "alice", "doing", "done", nil)}}
	got := sentenceFor("alice", g)
	want := "alice moved this from doing to done"
	if got != want {
		t.Errorf("sentence = %q, want %q", got, want)
	}
}

func TestSentenceForChainedTransitionNamesTheStepCount(t *testing.T) {
	g := historyGroup{Hops: []historyRow{
		transitionRow(t, 1, "alice", "todo", "doing", nil),
		transitionRow(t, 2, "alice", "doing", "done", nil),
	}}
	got := sentenceFor("alice", g)
	want := "alice moved this from todo to done (2 steps)"
	if got != want {
		t.Errorf("sentence = %q, want %q", got, want)
	}
}

func TestSentenceForUpdateDropsNoisyFieldsAndNamesTheRest(t *testing.T) {
	g := historyGroup{Hops: []historyRow{{
		Entry:   core.AuditEntry{Action: "task.update"},
		Changed: []string{"priority", "title", "updated_at", "version"},
	}}}
	got := sentenceFor("alice", g)
	want := "alice updated the priority, the title"
	if got != want {
		t.Errorf("sentence = %q, want %q", got, want)
	}
}

// An update whose only changed fields are the noisy bookkeeping ones (a
// version bump with nothing else, however that arises) still reads as a
// sentence rather than an empty list.
func TestSentenceForUpdateWithOnlyNoisyFieldsFallsBackToAGenericSentence(t *testing.T) {
	g := historyGroup{Hops: []historyRow{{
		Entry:   core.AuditEntry{Action: "task.update"},
		Changed: []string{"updated_at", "version"},
	}}}
	got := sentenceFor("alice", g)
	if got != "alice updated this" {
		t.Errorf("sentence = %q, want a generic fallback", got)
	}
}

func TestSentenceForEmptyActorNamesTheSystem(t *testing.T) {
	g := historyGroup{Hops: []historyRow{{Entry: core.AuditEntry{Action: "task.lease_expire"}}}}
	got := sentenceFor("", g)
	if got != "The claim on this expired" {
		t.Errorf("sentence = %q, want the system-attributed wording", got)
	}
}

func TestSentenceForCreateDeleteRestore(t *testing.T) {
	cases := map[string]string{
		"task.create":  "alice created this",
		"task.delete":  "alice deleted this",
		"task.restore": "alice restored this",
		"task.claim":   "alice claimed this",
		"task.release": "alice released the claim on this",
	}
	for action, want := range cases {
		g := historyGroup{Hops: []historyRow{{Entry: core.AuditEntry{Action: action}}}}
		if got := sentenceFor("alice", g); got != want {
			t.Errorf("%s: sentence = %q, want %q", action, got, want)
		}
	}
}

func TestMeaningfulFieldsDropsNoise(t *testing.T) {
	got := meaningfulFields([]string{"status", "updated_at", "version", "completed_at", "title"})
	want := []string{"status", "title"}
	if len(got) != len(want) {
		t.Fatalf("meaningfulFields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("meaningfulFields = %v, want %v", got, want)
		}
	}
}

// Relative-time rendering itself is now output.TimeStyle.Relative, tested in
// internal/output. What belongs to this package is only that the template
// wiring in render.go binds "relativeAt" to it; see TestFuncsBindsTimeStyle.
