// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// forkedTaskWorkflow reaches done from todo two ways in the same number of
// steps, which is the shape a route may not silently choose between.
func forkedTaskWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "Todo"},
			{Key: "review", Label: "Review"},
			{Key: "qa", Label: "QA"},
			{Key: "done", Label: "Done", Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "review"},
			{From: "todo", To: "qa"},
			{From: "review", To: "done"},
			{From: "qa", To: "done"},
		},
	}
}

// transitionTrail lists the task's recorded state changes, oldest first, as
// "from>to". It reads the audit entries themselves rather than the task's
// current status, because the question is what the trail says happened.
func transitionTrail(t *testing.T, l *Local, scope core.TenantScope, taskID string) []string {
	t.Helper()
	ctx := context.Background()
	var out []string
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{
			SubjectType: "task", SubjectID: taskID,
			Actions: []string{"task.transition"}, Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
		for _, e := range entries {
			out = append(out, auditStatus(t, e.Before)+">"+auditStatus(t, e.After))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the trail: %v", err)
	}
	return out
}

// eventTrail lists the transition events the task emitted, oldest first, as
// "from>to". A subscriber and a webhook receiver see exactly this sequence.
func eventTrail(t *testing.T, l *Local, scope core.TenantScope, taskID string) []string {
	t.Helper()
	ctx := context.Background()
	var out []string
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(ctx, 0, 1000)
		if err != nil {
			return err
		}
		sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })
		for _, e := range events {
			if e.Type != core.EventTaskTransitioned || e.SubjectID != taskID {
				continue
			}
			out = append(out, payloadString(e.Payload, "from")+">"+payloadString(e.Payload, "to"))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the stream: %v", err)
	}
	return out
}

func auditStatus(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var image struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &image); err != nil {
		t.Fatalf("reading an audit image: %v", err)
	}
	return image.Status
}

func payloadString(payload map[string]any, key string) string {
	s, _ := payload[key].(string)
	return s
}

// TestARouteWritesOneAuditEntryPerHopAndNothingElse is the guarantee the whole
// change exists for: the trail says the task passed through the states it
// passed through, once each, in order, and says nothing about a state it did
// not enter.
//
// It sits on the service because the service writes the entries. A surface
// that wires a route up wrongly then fails against a rule the service states,
// rather than each surface being spot-checked for the same thing separately.
func TestARouteWritesOneAuditEntryPerHopAndNothingElse(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)

	tests := []struct {
		name string
		in   core.RouteInput
		want []string
	}{
		{"one hop", core.RouteInput{Route: []string{"doing"}}, []string{"todo>doing"}},
		{"through one state", core.RouteInput{Route: []string{"doing", "done"}},
			[]string{"todo>doing", "doing>done"}},
		{"through two states", core.RouteInput{Route: []string{"doing", "review", "done"}, Comment: "ok"},
			[]string{"todo>doing", "doing>review", "review>done"}},
		{"a named destination finds its own route", core.RouteInput{To: "done"},
			[]string{"todo>doing", "doing>done"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "route " + tt.name})
			result, err := l.TransitionRoute(ctx, core.TaskRef{ID: task.ID}, tt.in)
			if err != nil {
				t.Fatalf("TransitionRoute: %v", err)
			}
			if result.Partial() {
				t.Fatalf("route stopped at %q: %s", result.Stopped, result.Reason)
			}
			if got := transitionTrail(t, l, scope, task.ID); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("audit = %v, want exactly one entry per hop in order: %v", got, tt.want)
			}
			if got := eventTrail(t, l, scope, task.ID); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("events = %v, want exactly one event per hop in order: %v", got, tt.want)
			}
			if result.Hops() != len(tt.want) {
				t.Errorf("result reports %d hops, want %d", result.Hops(), len(tt.want))
			}
			if got := strings.Join(result.Applied, ","); got != strings.Join(hopTargets(tt.want), ",") {
				t.Errorf("result applied %q, want %q", got, strings.Join(hopTargets(tt.want), ","))
			}
		})
	}
}

// hopTargets is the "to" half of each expected hop.
func hopTargets(hops []string) []string {
	out := make([]string, 0, len(hops))
	for _, h := range hops {
		_, to, _ := strings.Cut(h, ">")
		out = append(out, to)
	}
	return out
}

// TestARouteThatStopsPartWayWritesOnlyTheHopsThatHappened is the error path,
// which is the one most likely to be got wrong: an entry or an event for a hop
// that was refused would say a task entered a state it never entered.
func TestARouteThatStopsPartWayWritesOnlyTheHopsThatHappened(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "broken route"})

	// review to done requires a comment, so the third hop is refused after the
	// first two have already been written.
	result, err := l.TransitionRoute(ctx, core.TaskRef{ID: task.ID},
		core.RouteInput{Route: []string{"doing", "review", "done"}})
	if err != nil {
		t.Fatalf("a route that stops part way is a result, not an error: %v", err)
	}
	if !result.Partial() || result.Stopped != "done" {
		t.Fatalf("result = %+v, want a partial route stopped at done", result)
	}
	if result.Status() != "review" {
		t.Errorf("result says the task is in %q, want review", result.Status())
	}
	want := "todo>doing,doing>review"
	if got := transitionTrail(t, l, scope, task.ID); strings.Join(got, ",") != want {
		t.Errorf("audit = %v, want only the two hops that happened: %s", got, want)
	}
	if got := eventTrail(t, l, scope, task.ID); strings.Join(got, ",") != want {
		t.Errorf("events = %v, want only the two hops that happened: %s", got, want)
	}
	live, err := l.GetTask(ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if live.Status != "review" {
		t.Errorf("task is in %q, want the state the route reached", live.Status)
	}
}

// TestARouteRefusedOnItsFirstHopWritesNothing keeps the one case that is an
// error an error: nothing happened, so there is nothing to report but the
// refusal.
func TestARouteRefusedOnItsFirstHopWritesNothing(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "refused route"})

	if _, err := l.TransitionRoute(ctx, core.TaskRef{ID: task.ID},
		core.RouteInput{Route: []string{"done", "todo"}}); !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("a route whose first hop is illegal = %v, want precondition failed", err)
	}
	if got := transitionTrail(t, l, scope, task.ID); len(got) != 0 {
		t.Errorf("audit = %v, want nothing written", got)
	}
	if got := eventTrail(t, l, scope, task.ID); len(got) != 0 {
		t.Errorf("events = %v, want nothing emitted", got)
	}
}

// TestARouteToAnAmbiguousDestinationIsRefused holds the decision not to pick.
// Choosing the first of two equal routes would walk the task through a state
// nobody named, and that write is visible to every subscriber.
func TestARouteToAnAmbiguousDestinationIsRefused(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	project := seedTaskProjectWith(t, l, scope, "fork", forkedTaskWorkflow())
	ctx := taskContext(actor)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: project.Key, Title: "forked"})

	_, err := l.TransitionRoute(ctx, core.TaskRef{ID: task.ID}, core.RouteInput{To: "done"})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an ambiguous destination = %v, want invalid", err)
	}
	for _, want := range []string{"review>done", "qa>done", "name the route you want"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err.Error(), want)
		}
	}
	if got := transitionTrail(t, l, scope, task.ID); len(got) != 0 {
		t.Errorf("audit = %v, want nothing written for a refused route", got)
	}
	// Naming the route is what the refusal asks for, and it works.
	result, err := l.TransitionRoute(ctx, core.TaskRef{ID: task.ID},
		core.RouteInput{Route: []string{"qa", "done"}})
	if err != nil {
		t.Fatalf("the named route: %v", err)
	}
	if got := transitionTrail(t, l, scope, task.ID); strings.Join(got, ",") != "todo>qa,qa>done" {
		t.Errorf("audit = %v, want the route that was named", got)
	}
	_ = result
}

func TestRouteInputIsChecked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   core.RouteInput
	}{
		{"nothing named", core.RouteInput{}},
		{"both named", core.RouteInput{To: "done", Route: []string{"doing"}}},
		{"a state twice", core.RouteInput{Route: []string{"doing", "doing"}}},
		{"an empty state", core.RouteInput{Route: []string{"doing", " "}}},
		{"longer than the bound", core.RouteInput{Route: tooManySteps()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.in.Validate(); !core.IsKind(err, core.KindInvalid) {
				t.Errorf("Validate() = %v, want invalid", err)
			}
		})
	}
}

func tooManySteps() []string {
	out := make([]string, 0, core.MaxRouteSteps+1)
	for i := range core.MaxRouteSteps + 1 {
		out = append(out, string(rune('a'+i)))
	}
	return out
}

// TestARouteResultSaysWhereTheTaskWent holds the sentence every surface
// reports, so a partial route reads the same in a browser, a terminal and a
// shell.
func TestARouteResultSaysWhereTheTaskWent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		res  core.RouteResult
		want string
	}{
		{"one hop", core.RouteResult{From: "todo", Applied: []string{"doing"}}, "task moved to doing"},
		{"several", core.RouteResult{From: "todo", Applied: []string{"doing", "done"}},
			"task moved todo → doing → done, one step at a time"},
		{"stopped", core.RouteResult{From: "todo", Applied: []string{"doing"}, Stopped: "done", Reason: "needs a comment"},
			"task moved to doing, then stopped: could not move to done (needs a comment)"},
		{"stopped after several", core.RouteResult{From: "todo", Applied: []string{"doing", "review"}, Stopped: "done"},
			"task moved todo → doing → review, then stopped: could not move to done"},
		{"stopped at once", core.RouteResult{From: "todo", Stopped: "doing"},
			"task moved nowhere, then stopped: could not move to doing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.res.Sentence(); got != tt.want {
				t.Errorf("Sentence() = %q, want %q", got, tt.want)
			}
		})
	}
}
