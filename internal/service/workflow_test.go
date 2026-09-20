package service

import (
	"context"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

func twoStateDefinition() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
		Transitions: []core.Transition{
			{From: "todo", To: "done"},
		},
	}
}

func TestPutWorkflowCreatesAndRecords(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	events, audits := countRows(t, l, scope)

	wf, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "Lean", Definition: twoStateDefinition()})
	if err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}
	if wf.Key != "lean" {
		t.Errorf("key = %q, want the lowercased form", wf.Key)
	}
	if wf.Name != "lean" {
		t.Errorf("name = %q, want the key when no name is supplied", wf.Name)
	}
	if wf.Builtin {
		t.Error("a user-defined workflow must not be marked builtin")
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("PutWorkflow did not write both an audit entry and an event")
	}
}

func TestPutWorkflowValidation(t *testing.T) {
	l, _, ctx := withDefaults(t)

	tests := []struct {
		name string
		in   core.WorkflowInput
	}{
		{"no key", core.WorkflowInput{Definition: twoStateDefinition()}},
		{"no states", core.WorkflowInput{Key: "empty", Definition: core.WorkflowDefinition{Initial: "todo"}}},
		{"unknown initial", core.WorkflowInput{Key: "bad", Definition: core.WorkflowDefinition{
			Initial: "start",
			States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
		}}},
		{"transition to an unknown state", core.WorkflowInput{Key: "bad", Definition: core.WorkflowDefinition{
			Initial:     "todo",
			States:      []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
			Transitions: []core.Transition{{From: "todo", To: "elsewhere"}},
		}}},
		{"no terminal state", core.WorkflowInput{Key: "bad", Definition: core.WorkflowDefinition{
			Initial: "todo",
			States:  []core.State{{Key: "todo"}},
		}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.PutWorkflow(ctx, tc.in); !core.IsKind(err, core.KindInvalid) {
				t.Errorf("PutWorkflow = %v, want invalid", err)
			}
		})
	}
}

func TestPutWorkflowReplacesAndKeepsTheBuiltinFlag(t *testing.T) {
	l, _, ctx := withDefaults(t)

	def := BuiltinWorkflow()
	def.States = append(def.States, core.State{Key: "review"})
	def.Transitions = append(def.Transitions, core.Transition{From: "doing", To: "review"})

	wf, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: BuiltinWorkflowKey, Name: "Default", Definition: def})
	if err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}
	if !wf.Builtin {
		t.Error("editing the builtin workflow cleared its builtin flag")
	}
	got, err := l.GetWorkflow(ctx, BuiltinWorkflowKey)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if !got.Definition.HasState("review") {
		t.Error("the edit was not stored")
	}
	if got.ID != wf.ID {
		t.Error("replacing a workflow changed its identifier")
	}
}

func TestGetAndListWorkflows(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "lean", Definition: twoStateDefinition()}); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}

	got, err := l.GetWorkflow(ctx, "LEAN")
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.Key != "lean" {
		t.Errorf("GetWorkflow = %+v", got)
	}
	if _, err := l.GetWorkflow(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetWorkflow for an unknown key = %v, want not found", err)
	}

	all, err := l.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListWorkflows = %d, want the builtin and the new one", len(all))
	}
}

// Removing a state that tasks are still in destroys data unless the caller says
// where those tasks should go.
func TestPutWorkflowRefusesToOrphanTasks(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	task := seedTask(t, l, scope, p.ID, "blocked")

	shrunk := BuiltinWorkflow()
	var states []core.State
	for _, s := range shrunk.States {
		if s.Key != "blocked" {
			states = append(states, s)
		}
	}
	shrunk.States = states
	var transitions []core.Transition
	for _, tr := range shrunk.Transitions {
		if tr.From != "blocked" && tr.To != "blocked" {
			transitions = append(transitions, tr)
		}
	}
	shrunk.Transitions = transitions

	_, err = l.PutWorkflow(ctx, core.WorkflowInput{Key: BuiltinWorkflowKey, Name: "Default", Definition: shrunk})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("PutWorkflow dropping a state in use = %v, want conflict", err)
	}

	after, err := l.GetWorkflow(ctx, BuiltinWorkflowKey)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if !after.Definition.HasState("blocked") {
		t.Error("a refused edit was applied anyway")
	}
	if got := readTask(t, l, scope, task.ID); got.Status != "blocked" {
		t.Errorf("task status = %q, want it untouched", got.Status)
	}
}

func TestPutWorkflowMigratesTasksOutOfARemovedState(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	blocked := seedTask(t, l, scope, p.ID, "blocked")
	untouched := seedTask(t, l, scope, p.ID, "todo")

	shrunk := BuiltinWorkflow()
	var states []core.State
	for _, s := range shrunk.States {
		if s.Key != "blocked" {
			states = append(states, s)
		}
	}
	shrunk.States = states
	var transitions []core.Transition
	for _, tr := range shrunk.Transitions {
		if tr.From != "blocked" && tr.To != "blocked" {
			transitions = append(transitions, tr)
		}
	}
	shrunk.Transitions = transitions

	events, audits := countRows(t, l, scope)
	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{
		Key:        BuiltinWorkflowKey,
		Name:       "Default",
		Definition: shrunk,
		Migrate:    map[string]string{"blocked": "todo"},
	}); err != nil {
		t.Fatalf("PutWorkflow with a migration: %v", err)
	}

	if got := readTask(t, l, scope, blocked.ID); got.Status != "todo" {
		t.Errorf("migrated task status = %q, want todo", got.Status)
	}
	if got := readTask(t, l, scope, untouched.ID); got.Status != "todo" {
		t.Errorf("untouched task status = %q", got.Status)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-events != 2 || afterAudits-audits != 2 {
		t.Errorf("the migration wrote %d events and %d audit entries, want one each for the workflow and the task",
			afterEvents-events, afterAudits-audits)
	}
}

func TestPutWorkflowRejectsAMigrationToAnUndefinedState(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	seedTask(t, l, scope, p.ID, "blocked")

	shrunk := twoStateDefinition()
	_, err = l.PutWorkflow(ctx, core.WorkflowInput{
		Key:        BuiltinWorkflowKey,
		Definition: shrunk,
		Migrate:    map[string]string{"blocked": "nowhere", "doing": "todo", "cancelled": "done"},
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Errorf("PutWorkflow with a migration to an undefined state = %v, want invalid", err)
	}
}

// Removing a state nothing is in needs no migration.
func TestPutWorkflowAllowsRemovingAnUnusedState(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{
		Key:        BuiltinWorkflowKey,
		Definition: twoStateDefinition(),
	}); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}
	got, err := l.GetWorkflow(ctx, BuiltinWorkflowKey)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.Definition.HasState("blocked") {
		t.Error("the removed state is still present")
	}
}

func TestDeleteWorkflow(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "lean", Definition: twoStateDefinition()}); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}

	events, audits := countRows(t, l, scope)
	if err := l.DeleteWorkflow(ctx, "LEAN"); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("DeleteWorkflow did not write both an audit entry and an event")
	}
	if _, err := l.GetWorkflow(ctx, "lean"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetWorkflow after deletion = %v, want not found", err)
	}
	if err := l.DeleteWorkflow(ctx, "lean"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting twice = %v, want not found", err)
	}
}

func TestDeleteWorkflowRefusesTheBuiltinAndOnesInUse(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if err := l.DeleteWorkflow(ctx, BuiltinWorkflowKey); !core.IsKind(err, core.KindConflict) {
		t.Errorf("deleting the builtin workflow = %v, want conflict", err)
	}

	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "lean", Definition: twoStateDefinition()}); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infra", WorkflowKey: "lean"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	err := l.DeleteWorkflow(ctx, "lean")
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("deleting a workflow in use = %v, want conflict", err)
	}

	if err := l.ArchiveProject(ctx, "infra"); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	if err := l.DeleteWorkflow(ctx, "lean"); !core.IsKind(err, core.KindConflict) {
		t.Errorf("an archived project still uses its workflow; got %v", err)
	}
}

func TestWorkflowsDoNotLeakAcrossTenants(t *testing.T) {
	l, _, ctx := withDefaults(t)
	other, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	otherCtx := core.WithActor(context.Background(), core.SystemActor(other.ID))
	if _, err := l.PutWorkflow(otherCtx, core.WorkflowInput{Key: "theirs", Definition: twoStateDefinition()}); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}

	if _, err := l.GetWorkflow(ctx, "theirs"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetWorkflow across tenants = %v, want not found", err)
	}
	all, err := l.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	for _, wf := range all {
		if wf.Key == "theirs" {
			t.Error("ListWorkflows leaked another tenant's workflow")
		}
	}
}

func TestWorkflowWritesRequireWorkflowWriteScope(t *testing.T) {
	l, _, ctx := withDefaults(t)
	actor, _ := core.ActorFrom(ctx)
	member := &core.Actor{ID: "m1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	memberCtx := core.WithActor(context.Background(), member)

	if _, err := l.PutWorkflow(memberCtx, core.WorkflowInput{Key: "lean", Definition: twoStateDefinition()}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member writing a workflow = %v, want forbidden", err)
	}
	if err := l.DeleteWorkflow(memberCtx, "lean"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member deleting a workflow = %v, want forbidden", err)
	}
	if _, err := l.ListWorkflows(memberCtx); err != nil {
		t.Errorf("member listing workflows = %v, want allowed", err)
	}
}
