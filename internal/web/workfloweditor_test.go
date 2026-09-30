// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// textareaValue reads back exactly what the editor put in one textarea, so a
// test can resubmit the form the way a reader who changed nothing would.
func textareaValue(t *testing.T, page, id string) string {
	t.Helper()
	open := `<textarea id="` + id + `"`
	i := strings.Index(page, open)
	if i < 0 {
		t.Fatalf("the editor has no %q textarea:\n%s", id, page)
	}
	rest := page[i:]
	start := strings.Index(rest, ">")
	end := strings.Index(rest, "</textarea>")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("the %q textarea is not closed:\n%s", id, page)
	}
	return html.UnescapeString(rest[start+1 : end])
}

// Saving the workflow editor without changing anything has to leave the
// workflow as it was.
//
// The editor renders a state as "key|label|terminal" and a transition as
// "from>to", which is not everything either one carries: a state also holds
// its category and its lease-expiry revert, and a transition holds the scope
// it requires. PutWorkflow replaces the stored definition with whatever the
// form parsed, so anything the editor does not render is not edited away by
// the reader -- it is erased by the round trip.
func TestSavingTheWorkflowEditorUnchangedKeepsTheWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	before, err := f.svc.GetWorkflow(f.ctx(), "default")
	if err != nil {
		t.Fatalf("reading the shipped workflow: %v", err)
	}
	// The shipped workflow has to actually carry the fields under test, or
	// this guard would pass on a workflow that never had them.
	var hadCategory, hadRevert bool
	for _, s := range before.Definition.States {
		if s.Category != "" {
			hadCategory = true
		}
		if s.RevertOnLeaseExpiry {
			hadRevert = true
		}
	}
	if !hadCategory || !hadRevert {
		t.Fatalf("the shipped workflow carries no category (%v) or no lease revert (%v) to lose",
			hadCategory, hadRevert)
	}

	page := b.page("/workflows/default")
	saved := b.post("/workflows", url.Values{
		"key":         {before.Key},
		"name":        {before.Name},
		"initial":     {before.Definition.Initial},
		"states":      {textareaValue(t, page, "states")},
		"transitions": {textareaValue(t, page, "transitions")},
		"migrate":     {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "default")
	if err != nil {
		t.Fatalf("re-reading the workflow: %v", err)
	}

	byKey := map[string]core.State{}
	for _, s := range after.Definition.States {
		byKey[s.Key] = s
	}
	for _, want := range before.Definition.States {
		got, ok := byKey[want.Key]
		if !ok {
			t.Errorf("state %q is gone after a save that changed nothing", want.Key)
			continue
		}
		if got.Category != want.Category {
			t.Errorf("state %q lost its category: %q, want %q", want.Key, got.Category, want.Category)
		}
		if got.RevertOnLeaseExpiry != want.RevertOnLeaseExpiry || got.RevertTo != want.RevertTo {
			t.Errorf("state %q lost its lease-expiry revert: %v/%q, want %v/%q",
				want.Key, got.RevertOnLeaseExpiry, got.RevertTo, want.RevertOnLeaseExpiry, want.RevertTo)
		}
	}
	if after.Definition.DefaultLease != before.Definition.DefaultLease {
		t.Errorf("the workflow lost its default lease: %v, want %v",
			after.Definition.DefaultLease, before.Definition.DefaultLease)
	}
}

// The same round trip, for the one field on a transition that is an
// authorization gate rather than a display detail. A scope the editor never
// rendered comes back cleared, so a move an operator restricted becomes a
// move anybody holding the plain transition scope may make.
func TestSavingTheWorkflowEditorUnchangedKeepsATransitionScope(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	gated := core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "shipped", Label: "Shipped", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "shipped", RequiresScope: core.ScopeAll, RequiresComment: true},
		},
	}
	if _, err := f.svc.PutWorkflow(f.ctx(), core.WorkflowInput{
		Key: "gated", Name: "Gated", Definition: gated,
	}); err != nil {
		t.Fatalf("defining the gated workflow: %v", err)
	}

	page := b.page("/workflows/gated")
	saved := b.post("/workflows", url.Values{
		"key":         {"gated"},
		"name":        {"Gated"},
		"initial":     {"todo"},
		"states":      {textareaValue(t, page, "states")},
		"transitions": {textareaValue(t, page, "transitions")},
		"migrate":     {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "gated")
	if err != nil {
		t.Fatalf("re-reading the gated workflow: %v", err)
	}
	if len(after.Definition.Transitions) != 1 {
		t.Fatalf("the save left %d transitions, want 1", len(after.Definition.Transitions))
	}
	got := after.Definition.Transitions[0]
	if got.RequiresScope != core.ScopeAll {
		t.Errorf("the transition lost the scope it required: %q, want %q", got.RequiresScope, core.ScopeAll)
	}
	if !got.RequiresComment {
		t.Error("the transition no longer requires a comment")
	}
}

// memberCtx speaks for an actor holding the plain transition scope and nothing
// more, which is exactly the authority a restricted edge is supposed to stop
// short of.
func (f *fixture) memberCtx() context.Context {
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID})
	return core.WithActor(ctx, copyActor(f.actorA, core.RoleMember.Scopes(), core.RoleMember))
}

// The stored field is not the point: the gate is. A scope that survives the
// round trip as a string but no longer refuses anybody is the same defect
// wearing a passing assertion, so this guard attempts the move as an actor
// without the scope and requires the service to refuse it.
func TestSavingTheWorkflowEditorUnchangedLeavesAGatedTransitionEnforced(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	gated := core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "shipped", Label: "Shipped", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "shipped", RequiresScope: core.ScopeTenantAdmin},
		},
	}
	if _, err := f.svc.PutWorkflow(f.ctx(), core.WorkflowInput{
		Key: "release", Name: "Release", Definition: gated,
	}); err != nil {
		t.Fatalf("defining the gated workflow: %v", err)
	}
	project := b.post("/projects", url.Values{
		"key": {"rel"}, "name": {"Releases"}, "workflow_key": {"release"}})
	_ = project.Body.Close()
	wantStatus(t, project, http.StatusSeeOther)

	move := func(ref string) error {
		_, err := f.svc.TransitionTask(f.memberCtx(),
			core.TaskRef{ProjectKey: "rel", Seq: seqOf(t, ref)},
			core.TransitionInput{To: "shipped"})
		return err
	}

	// Before the save, so that a refusal afterwards cannot be a refusal that
	// was always going to happen for some other reason.
	if err := move(b.createTask("rel", "before")); core.KindOf(err) != core.KindForbidden {
		t.Fatalf("the gate did not refuse the move before any save: %v", err)
	}

	page := b.page("/workflows/release")
	saved := b.post("/workflows", url.Values{
		"key":         {"release"},
		"name":        {"Release"},
		"initial":     {"todo"},
		"states":      {textareaValue(t, page, "states")},
		"transitions": {textareaValue(t, page, "transitions")},
		"migrate":     {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	err := move(b.createTask("rel", "after"))
	if core.KindOf(err) != core.KindForbidden {
		t.Fatalf("after a save that changed nothing the gated move was allowed: err = %v, want forbidden", err)
	}
	if !strings.Contains(err.Error(), string(core.ScopeTenantAdmin)) {
		t.Errorf("the refusal does not name the scope it enforced: %v", err)
	}
	// The refusal has to be the scope and not a workflow the save broke, so
	// the same move must still succeed for somebody who holds the scope.
	if _, err := f.svc.TransitionTask(f.ctx(),
		core.TaskRef{ProjectKey: "rel", Seq: seqOf(t, b.createTask("rel", "allowed"))},
		core.TransitionInput{To: "shipped"}); err != nil {
		t.Fatalf("the move is refused even for an actor holding the scope, so the refusal above proves nothing: %v", err)
	}
}

// seqOf reads the sequence number out of a task reference such as "rel-3".
func seqOf(t *testing.T, ref string) int64 {
	t.Helper()
	_, tail, ok := strings.Cut(ref, "-")
	if !ok {
		t.Fatalf("task reference %q has no sequence", ref)
	}
	n, err := strconv.ParseInt(tail, 10, 64)
	if err != nil {
		t.Fatalf("task reference %q: %v", ref, err)
	}
	return n
}

// A field added to core.State or core.Transition that the editor neither
// renders nor carries through would reintroduce this defect silently. Rather
// than listing the fields to preserve, this guard stores a state and a
// transition with every field set, refuses to run if reflection finds one left
// at its zero value, and requires the whole struct back after a save that
// edited nothing.
func TestTheWorkflowEditorLosesNoFieldOfAStateOrATransition(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	full := core.State{
		Key: "parked", Label: "Parked", Terminal: true,
		Category: core.CategoryDone, RevertOnLeaseExpiry: true, RevertTo: "open",
	}
	edge := core.Transition{
		From: "open", To: "parked",
		RequiresScope: core.ScopeTenantAdmin, RequiresComment: true,
	}
	requireEveryFieldSet(t, "the state under test", full)
	requireEveryFieldSet(t, "the transition under test", edge)

	if _, err := f.svc.PutWorkflow(f.ctx(), core.WorkflowInput{
		Key: "everything", Name: "Everything",
		Definition: core.WorkflowDefinition{
			Initial:      "open",
			States:       []core.State{{Key: "open", Label: "Open", Category: core.CategoryTodo}, full},
			Transitions:  []core.Transition{edge},
			DefaultLease: core.Duration(45 * time.Minute),
		},
	}); err != nil {
		t.Fatalf("defining the workflow: %v", err)
	}

	page := b.page("/workflows/everything")
	saved := b.post("/workflows", url.Values{
		"key":         {"everything"},
		"name":        {"Everything"},
		"initial":     {"open"},
		"states":      {textareaValue(t, page, "states")},
		"transitions": {textareaValue(t, page, "transitions")},
		"migrate":     {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "everything")
	if err != nil {
		t.Fatalf("re-reading the workflow: %v", err)
	}
	got, ok := after.Definition.State("parked")
	if !ok {
		t.Fatalf("the state is gone after a save that changed nothing")
	}
	reportFieldLosses(t, "state", full, got)
	gotEdge, ok := after.Definition.CanTransition("open", "parked")
	if !ok {
		t.Fatalf("the transition is gone after a save that changed nothing")
	}
	reportFieldLosses(t, "transition", edge, gotEdge)
	if after.Definition.DefaultLease != core.Duration(45*time.Minute) {
		t.Errorf("the definition lost its default lease: %v", after.Definition.DefaultLease)
	}
}

// requireEveryFieldSet fails when the fixture leaves a field at its zero
// value, which is what happens the day somebody adds one.
func requireEveryFieldSet(t *testing.T, label string, v any) {
	t.Helper()
	rv := reflect.ValueOf(v)
	for i := range rv.NumField() {
		if rv.Field(i).IsZero() {
			t.Fatalf("%s leaves %s at its zero value, so this guard cannot see the editor losing it; give the field a distinctive value here",
				label, rv.Type().Field(i).Name)
		}
	}
}

// reportFieldLosses names each field the round trip did not return intact.
func reportFieldLosses(t *testing.T, label string, want, got any) {
	t.Helper()
	w, g := reflect.ValueOf(want), reflect.ValueOf(got)
	for i := range w.NumField() {
		if !reflect.DeepEqual(w.Field(i).Interface(), g.Field(i).Interface()) {
			t.Errorf("%s lost %s: %v, want %v", label, w.Type().Field(i).Name,
				g.Field(i).Interface(), w.Field(i).Interface())
		}
	}
}

// Renaming a state key gives the old key's unrendered fields nowhere to go
// unless the form says the new key is the old one renamed, which is what the
// migration lines already say.
func TestRenamingAStateThroughAMigrationKeepsWhatTheEditorDoesNotShow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/workflows/default")
	states := strings.ReplaceAll(textareaValue(t, page, "states"), "doing|", "wip|")
	transitions := strings.ReplaceAll(textareaValue(t, page, "transitions"), "doing", "wip")
	saved := b.post("/workflows", url.Values{
		"key":         {"default"},
		"name":        {"Default"},
		"initial":     {"todo"},
		"states":      {states},
		"transitions": {transitions},
		"migrate":     {"doing=wip"},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "default")
	if err != nil {
		t.Fatalf("re-reading the workflow: %v", err)
	}
	if _, stillThere := after.Definition.State("doing"); stillThere {
		t.Fatal("the renamed state is still there under its old key")
	}
	got, ok := after.Definition.State("wip")
	if !ok {
		t.Fatalf("the renamed state is missing")
	}
	if got.Category != core.CategoryInProgress {
		t.Errorf("the renamed state lost its category: %q", got.Category)
	}
	if !got.RevertOnLeaseExpiry || got.RevertTo != "todo" {
		t.Errorf("the renamed state lost its lease-expiry revert: %v/%q",
			got.RevertOnLeaseExpiry, got.RevertTo)
	}
}

// A state the form dropped is dropped, so the merge cannot be mistaken for a
// rule that nothing is ever removed.
func TestRemovingAStateInTheEditorRemovesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	if _, err := f.svc.PutWorkflow(f.ctx(), core.WorkflowInput{
		Key: "trim", Name: "Trim",
		Definition: core.WorkflowDefinition{
			Initial: "open",
			States: []core.State{
				{Key: "open", Label: "Open", Category: core.CategoryTodo},
				{Key: "parked", Label: "Parked", Category: core.CategoryTodo},
				{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
			},
			Transitions: []core.Transition{
				{From: "open", To: "parked", RequiresScope: core.ScopeTenantAdmin},
				{From: "open", To: "done"},
			},
		},
	}); err != nil {
		t.Fatalf("defining the workflow: %v", err)
	}

	saved := b.post("/workflows", url.Values{
		"key": {"trim"}, "name": {"Trim"}, "initial": {"open"},
		"states": {"open|Open|open\ndone|Done|terminal"}, "transitions": {"open>done"}, "migrate": {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "trim")
	if err != nil {
		t.Fatalf("re-reading the workflow: %v", err)
	}
	if _, ok := after.Definition.State("parked"); ok {
		t.Error("a state removed in the editor came back")
	}
	if _, ok := after.Definition.CanTransition("open", "parked"); ok {
		t.Error("a transition removed in the editor came back")
	}
}

// The help text under the states box says to write true or false. The parser
// used to accept only the word "terminal", so following the instructions
// produced an open state and no complaint at all.
func TestTheStateLineAcceptsTheTerminalWordsItDocuments(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, tc := range []struct {
		name  string
		lines string
		want  bool
	}{
		{"true", "done|Done|true", true},
		{"terminal", "done|Done|terminal", true},
		{"false", "done|Done|false", false},
		{"yes", "done|Done|yes", true},
		{"no", "done|Done|no", false},
		{"blank", "done|Done|", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "words" + tc.name
			// "closed" is here only so the definition always has a terminal
			// state, whatever the line under test is read as.
			resp := b.post("/workflows", url.Values{
				"key": {key}, "name": {key}, "initial": {"open"},
				"states":      {"open|Open|open\n" + tc.lines + "\nclosed|Closed|terminal"},
				"transitions": {"open>done\nopen>closed"}, "migrate": {""},
			})
			defer func() { _ = resp.Body.Close() }()
			wantStatus(t, resp, http.StatusSeeOther)
			wf, err := f.svc.GetWorkflow(f.ctx(), key)
			if err != nil {
				t.Fatalf("reading %q: %v", key, err)
			}
			done, ok := wf.Definition.State("done")
			if !ok {
				t.Fatalf("no done state in %q", key)
			}
			if done.Terminal != tc.want {
				t.Errorf("%q read as terminal=%v, want %v", tc.lines, done.Terminal, tc.want)
			}
		})
	}
}

// A third field naming neither answer is refused rather than quietly read as
// an open state.
func TestAnUnreadableTerminalFieldIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/workflows", url.Values{
		"key": {"maybe"}, "name": {"Maybe"}, "initial": {"open"},
		"states": {"open|Open|open\ndone|Done|maybe"}, "transitions": {"open>done"}, "migrate": {""},
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unreadable terminal field", resp.StatusCode)
	}
	if _, err := f.svc.GetWorkflow(f.ctx(), "maybe"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the refused save stored a workflow anyway: %v", err)
	}
}
