package service

import (
	"context"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// seedWorkflow installs a workflow directly, so project tests do not depend on
// the workflow service.
func seedWorkflow(t *testing.T, l *Local, scope core.TenantScope, key string, def core.WorkflowDefinition, builtin bool) *core.Workflow {
	t.Helper()
	ctx := context.Background()
	wf := &core.Workflow{Key: key, Name: key, Definition: def, Builtin: builtin}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.PutWorkflow(ctx, wf)
	}); err != nil {
		t.Fatalf("seeding workflow %q: %v", key, err)
	}
	return wf
}

// seedTask inserts a task directly in a chosen status.
func seedTask(t *testing.T, l *Local, scope core.TenantScope, projectID, status string) *core.Task {
	t.Helper()
	ctx := context.Background()
	task := &core.Task{ProjectID: projectID, Title: "seeded", Status: status}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		actors, err := tx.ListActors(ctx, core.Page{Limit: 1})
		if err != nil {
			return err
		}
		if len(actors) == 0 {
			t.Fatal("tenant has no actor to attribute the seeded task to")
		}
		task.CreatorActorID = actors[0].ID
		return tx.CreateTask(ctx, task)
	}); err != nil {
		t.Fatalf("seeding task: %v", err)
	}
	return task
}

// withDefaults returns a service and an admin context over a tenant that already
// has the builtin workflow.
func withDefaults(t *testing.T) (*Local, core.TenantScope, context.Context) {
	t.Helper()
	l, _, scope, actor := newLocal(t)
	seedWorkflow(t, l, scope, BuiltinWorkflowKey, BuiltinWorkflow(), true)
	return l, scope, core.WithActor(context.Background(), actor)
}

func TestCreateProjectUsesTheBuiltinWorkflowByDefault(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	events, audits := countRows(t, l, scope)

	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "Infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Key != "infra" {
		t.Errorf("key = %q, want the lowercased form", p.Key)
	}
	if p.WorkflowID == "" {
		t.Error("project was created without a workflow")
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("CreateProject did not write both an audit entry and an event")
	}
}

func TestCreateProjectRejections(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	tests := []struct {
		name string
		in   core.CreateProjectInput
		kind core.Kind
	}{
		{"duplicate key", core.CreateProjectInput{Key: "INFRA", Name: "Copy"}, core.KindConflict},
		{"bad key", core.CreateProjectInput{Key: "1infra", Name: "Bad"}, core.KindInvalid},
		{"no name", core.CreateProjectInput{Key: "ops"}, core.KindInvalid},
		{"unknown workflow", core.CreateProjectInput{Key: "ops", Name: "Ops", WorkflowKey: "nope"}, core.KindNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateProject(ctx, tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("CreateProject = %v, want %s", err, tc.kind)
			}
		})
	}
}

func TestGetProjectByKeyIdentifierAndCase(t *testing.T) {
	l, _, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	for _, ref := range []string{"infra", "INFRA", p.ID} {
		got, err := l.GetProject(ctx, ref)
		if err != nil {
			t.Fatalf("GetProject(%q): %v", ref, err)
		}
		if got.ID != p.ID {
			t.Errorf("GetProject(%q) = %q, want %q", ref, got.ID, p.ID)
		}
	}
	if _, err := l.GetProject(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetProject for an unknown key = %v, want not found", err)
	}
	if _, err := l.GetProject(ctx, ""); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("GetProject with no reference = %v, want invalid", err)
	}
}

func TestListProjectsHidesArchivedAndPages(t *testing.T) {
	l, _, ctx := withDefaults(t)
	for _, key := range []string{"alpha", "beta", "gamma"} {
		if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: key, Name: key}); err != nil {
			t.Fatalf("CreateProject(%q): %v", key, err)
		}
	}
	if err := l.ArchiveProject(ctx, "beta"); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}

	got, _, err := l.ListProjects(ctx, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ListProjects = %d projects, want 2 with the archived one hidden", len(got))
	}

	all, _, err := l.ListProjects(ctx, core.ProjectFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListProjects including archived: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListProjects including archived = %d, want 3", len(all))
	}

	first, cursor, err := l.ListProjects(ctx, core.ProjectFilter{
		IncludeArchived: true,
		Page:            core.Page{Limit: 2},
	})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 || cursor == "" {
		t.Fatalf("first page = %d projects, cursor %q", len(first), cursor)
	}
	second, next, err := l.ListProjects(ctx, core.ProjectFilter{
		IncludeArchived: true,
		Page:            core.Page{Limit: 2, Cursor: cursor},
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 1 || next != "" {
		t.Errorf("second page = %d projects, cursor %q", len(second), next)
	}
	if second[0].ID == first[0].ID {
		t.Error("the second page repeated the first")
	}

	if _, _, err := l.ListProjects(ctx, core.ProjectFilter{Page: core.Page{Sort: "nonsense"}}); !core.IsKind(err, core.KindInvalid) {
		t.Error("ListProjects accepted an unknown sort field")
	}
}

func TestUpdateProject(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	other := seedWorkflow(t, l, scope, "lean", core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
	}, false)

	name := "Platform"
	description := "Everything below the product"
	key := "lean"
	got, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{
		Name:        &name,
		Description: &description,
		WorkflowKey: &key,
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if got.Name != name || got.Description != description || got.WorkflowID != other.ID {
		t.Errorf("updated project = %+v", got)
	}

	blank := " "
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Name: &blank}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("UpdateProject with a blank name = %v, want invalid", err)
	}
	missing := "nope"
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{WorkflowKey: &missing}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("UpdateProject with an unknown workflow = %v, want not found", err)
	}
	if _, err := l.UpdateProject(ctx, "missing", core.UpdateProjectInput{Name: &name}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("UpdateProject on an unknown project = %v, want not found", err)
	}
}

// A project may only move to a workflow that still defines every status its
// tasks are in.
func TestUpdateProjectRefusesAWorkflowMissingAStatusInUse(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	seedTask(t, l, scope, p.ID, "blocked")
	seedWorkflow(t, l, scope, "lean", core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
	}, false)

	key := "lean"
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{WorkflowKey: &key}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("reassignment = %v, want conflict", err)
	}
	got, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.WorkflowID != p.WorkflowID {
		t.Error("a refused reassignment changed the project's workflow")
	}
}

func TestArchiveProjectIsRefusedTwice(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	events, audits := countRows(t, l, scope)
	if err := l.ArchiveProject(ctx, "infra"); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("ArchiveProject did not write both an audit entry and an event")
	}

	got, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if !got.Archived() {
		t.Error("the project is not marked archived")
	}
	if err := l.ArchiveProject(ctx, "infra"); !core.IsKind(err, core.KindConflict) {
		t.Errorf("archiving twice = %v, want conflict", err)
	}
}

func TestDeleteProject(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	events, audits := countRows(t, l, scope)
	if err := l.DeleteProject(ctx, "infra"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("DeleteProject did not write both an audit entry and an event")
	}
	if _, err := l.GetProject(ctx, "infra"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetProject after deletion = %v, want not found", err)
	}
	if err := l.DeleteProject(ctx, "infra"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting twice = %v, want not found", err)
	}
}

func TestProjectsDoNotLeakAcrossTenants(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Ours"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	other, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	otherScope := core.TenantScope{TenantID: other.ID}
	seedWorkflow(t, l, otherScope, BuiltinWorkflowKey, BuiltinWorkflow(), true)
	otherCtx := core.WithActor(context.Background(), core.SystemActor(other.ID))
	theirs, err := l.CreateProject(otherCtx, core.CreateProjectInput{Key: "infra", Name: "Theirs"})
	if err != nil {
		t.Fatalf("CreateProject in the other tenant: %v", err)
	}

	got, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.ID == theirs.ID || got.Name != "Ours" {
		t.Errorf("GetProject returned %+v from the other tenant", got)
	}
	if _, err := l.GetProject(ctx, theirs.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetProject by another tenant's identifier = %v, want not found", err)
	}
	if err := l.DeleteProject(ctx, theirs.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("DeleteProject across tenants = %v, want not found", err)
	}

	list, _, err := l.ListProjects(ctx, core.ProjectFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, p := range list {
		if p.ID == theirs.ID {
			t.Error("ListProjects leaked another tenant's project")
		}
	}
}

// A token restricted to one project must not reach another.
func TestProjectScopedTokenIsConfined(t *testing.T) {
	l, _, ctx := withDefaults(t)
	mine, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "mine", Name: "Mine"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "other", Name: "Other"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	actor, _ := core.ActorFrom(ctx)
	scoped := &core.Actor{
		ID: actor.ID, TenantID: actor.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeAll}, ProjectID: mine.ID,
	}
	scopedCtx := core.WithActor(context.Background(), scoped)

	if _, err := l.GetProject(scopedCtx, "mine"); err != nil {
		t.Fatalf("GetProject on its own project: %v", err)
	}
	if _, err := l.GetProject(scopedCtx, "other"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("GetProject outside the token's project = %v, want forbidden", err)
	}
	list, _, err := l.ListProjects(scopedCtx, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(list) != 1 || list[0].ID != mine.ID {
		t.Errorf("ListProjects = %+v, want only the token's project", list)
	}
}

func TestProjectWritesRequireProjectWriteScope(t *testing.T) {
	l, _, ctx := withDefaults(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	actor, _ := core.ActorFrom(ctx)
	viewer := &core.Actor{ID: "v1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	viewerCtx := core.WithActor(context.Background(), viewer)

	if _, err := l.CreateProject(viewerCtx, core.CreateProjectInput{Key: "ops", Name: "Ops"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer creating a project = %v, want forbidden", err)
	}
	if _, err := l.UpdateProject(viewerCtx, "infra", core.UpdateProjectInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer updating a project = %v, want forbidden", err)
	}
	if err := l.ArchiveProject(viewerCtx, "infra"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer archiving a project = %v, want forbidden", err)
	}
	if err := l.DeleteProject(viewerCtx, "infra"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer deleting a project = %v, want forbidden", err)
	}
	if _, err := l.GetProject(viewerCtx, "infra"); err != nil {
		t.Errorf("viewer reading a project = %v, want allowed", err)
	}
}

// readTask reads a task straight from the store, so assertions do not depend on
// the task service.
func readTask(t *testing.T, l *Local, scope core.TenantScope, taskID string) *core.Task {
	t.Helper()
	ctx := context.Background()
	var out *core.Task
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: taskID})
		out = got
		return err
	}); err != nil {
		t.Fatalf("reading task %q: %v", taskID, err)
	}
	return out
}
