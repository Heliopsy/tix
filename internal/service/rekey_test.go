// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// rekeyFixture is a tenant with project infra holding one task, infra-1.
func rekeyFixture(t *testing.T) (*Local, core.TenantScope, context.Context, *core.Project, *core.Task) {
	t.Helper()
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	task, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "patch the kernel"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	return l, scope, ctx, p, task
}

func TestRenamingAProjectKeyRenamesItsReferences(t *testing.T) {
	l, scope, ctx, p, task := rekeyFixture(t)
	events, audits := countRows(t, l, scope)

	got, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp(" OPS ")})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if got.Key != "ops" || got.ID != p.ID {
		t.Fatalf("renamed project = %q (%s), want ops on the same project", got.Key, got.ID)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Errorf("rename wrote %d events and %d audit entries, want one of each",
			afterEvents-events, afterAudits-audits)
	}
	ev := lastEvent(t, l, scope)
	if ev.Type != core.EventProjectUpdated || ev.Payload["previous_key"] != "infra" || ev.Payload["key"] != "ops" {
		t.Errorf("rename event = %s %v, want project.updated naming infra and ops", ev.Type, ev.Payload)
	}

	byID, err := l.GetTask(ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if byID.Ref != "ops-1" {
		t.Errorf("task ref after rename = %q, want ops-1", byID.Ref)
	}
}

func TestOldKeysResolveToTheRenamedProject(t *testing.T) {
	l, _, ctx, p, task := rekeyFixture(t)
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp("ops"), IconName: strp("rocket")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if got, err := l.GetProject(ctx, "infra"); err != nil || got.IconName != "rocket" {
		t.Errorf("a project read through its old key = %+v, %v, want its icon too", got, err)
	}

	for _, ref := range []string{"infra", "INFRA"} {
		got, err := l.GetProject(ctx, ref)
		if err != nil {
			t.Fatalf("GetProject(%q): %v", ref, err)
		}
		if got.ID != p.ID || got.Key != "ops" {
			t.Errorf("GetProject(%q) = %q, want the project now keyed ops", ref, got.Key)
		}
	}
	got, err := l.GetTask(ctx, core.MustParseTaskRef("infra-1"))
	if err != nil {
		t.Fatalf("GetTask(infra-1): %v", err)
	}
	if got.ID != task.ID || got.Ref != "ops-1" {
		t.Errorf("GetTask(infra-1) = %s, want the same task as ops-1", got.Ref)
	}
	if _, err := l.GetTask(ctx, core.MustParseTaskRef("infra-2")); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetTask(infra-2) = %v, want not found", err)
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{ProjectKeys: []string{"infra"}})
	if err != nil {
		t.Fatalf("ListTasks(project:infra): %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].Ref != "ops-1" {
		t.Errorf("project:infra listed %d tasks, want ops-1 alone", len(page.Tasks))
	}

	added, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "second"})
	if err != nil {
		t.Fatalf("CreateTask in an old key: %v", err)
	}
	if added.Ref != "ops-2" {
		t.Errorf("task added under the old key = %q, want ops-2", added.Ref)
	}
}

func TestARenameRefusesAKeyHeldByAnotherProject(t *testing.T) {
	l, scope, ctx, _, _ := rekeyFixture(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := l.UpdateProject(ctx, "web", core.UpdateProjectInput{Key: strp("infra")}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("renaming web to infra = %v, want a conflict", err)
	}

	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp("ops")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	events, audits := countRows(t, l, scope)
	if _, err := l.UpdateProject(ctx, "web", core.UpdateProjectInput{Key: strp("infra"), Name: strp("Changed")}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("renaming web to the old key infra = %v, want a conflict", err)
	}
	_, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Again"})
	if !core.IsKind(err, core.KindConflict) || !strings.Contains(err.Error(), `old key of project "ops"`) {
		t.Errorf("creating a project keyed with an old key = %v, want a conflict naming ops", err)
	}
	if web, err := l.GetProject(ctx, "web"); err != nil || web.Name != "Web" {
		t.Errorf("a refused rename changed the project: %+v, %v", web, err)
	}
	if e, a := countRows(t, l, scope); e != events || a != audits {
		t.Error("a refused rename wrote an event or an audit entry")
	}
	if got, err := l.GetProject(ctx, "infra"); err != nil || got.Key != "ops" {
		t.Errorf("old key infra now resolves to %+v, %v, want ops", got, err)
	}
}

func TestARenameRefusesAnInvalidKey(t *testing.T) {
	l, _, ctx, _, _ := rekeyFixture(t)
	for _, key := range []string{"", "9lives", "a b", "trailing-", "a/b"} {
		if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp(key)}); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("renaming to %q = %v, want a validation error", key, err)
		}
	}
	if got, err := l.GetProject(ctx, "infra"); err != nil || got.Key != "infra" {
		t.Errorf("project after refused renames = %+v, %v", got, err)
	}
}

func TestAProjectTakesBackItsOwnOldKey(t *testing.T) {
	l, _, ctx, p, _ := rekeyFixture(t)
	for _, key := range []string{"ops", "infra", "ops", "infra"} {
		if _, err := l.UpdateProject(ctx, p.Key, core.UpdateProjectInput{Key: strp(key)}); err != nil {
			t.Fatalf("renaming to %q: %v", key, err)
		}
		p.Key = key
	}
	got, err := l.GetProject(ctx, "ops")
	if err != nil || got.ID != p.ID || got.Key != "infra" {
		t.Fatalf("GetProject(ops) after renaming back = %+v, %v, want infra", got, err)
	}
	same, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp("infra")})
	if err != nil || same.Key != "infra" {
		t.Errorf("renaming to the current key = %+v, %v, want no change", same, err)
	}
}

func TestDeletingAProjectFreesItsOldKeys(t *testing.T) {
	l, _, ctx, _, _ := rekeyFixture(t)
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp("ops")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if _, err := l.DeleteProject(ctx, "ops", core.DeleteProjectInput{DeleteTasks: true}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := l.GetProject(ctx, "infra"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("old key after deletion = %v, want not found", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Fresh"}); err != nil {
		t.Errorf("creating a project with a freed key: %v", err)
	}
}

func TestOldKeysDoNotCrossTenants(t *testing.T) {
	l, _, ctx, _, _ := rekeyFixture(t)
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp("ops")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	other, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	otherScope := core.TenantScope{TenantID: other.ID}
	seedWorkflow(t, l, otherScope, BuiltinWorkflowKey, BuiltinWorkflow(), true)
	otherCtx := core.WithActor(context.Background(), core.SystemActor(other.ID))
	if _, err := l.GetProject(otherCtx, "infra"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("another tenant resolved our old key: %v", err)
	}
	if _, err := l.CreateProject(otherCtx, core.CreateProjectInput{Key: "infra", Name: "Theirs"}); err != nil {
		t.Errorf("another tenant could not use a key that is only our alias: %v", err)
	}
}

func TestASnapshotNamingAnOldKeyUpdatesTheRenamedProject(t *testing.T) {
	l, _, ctx, p, _ := rekeyFixture(t)
	var snap bytes.Buffer
	if err := l.ExportTo(ctx, core.ExportInput{}, &snap); err != nil {
		t.Fatalf("ExportTo: %v", err)
	}
	if _, err := l.UpdateProject(ctx, "infra", core.UpdateProjectInput{Key: strp("ops")}); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if _, err := l.ImportFrom(ctx, &snap, core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("ImportFrom: %v", err)
	}
	got, err := l.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Key != "ops" {
		t.Errorf("import reverted the key to %q, want ops", got.Key)
	}
	if again, err := l.GetProject(ctx, "infra"); err != nil || again.ID != p.ID {
		t.Errorf("old key after import = %+v, %v, want the same project", again, err)
	}
}

// lastEvent reads the most recent outbox event.
func lastEvent(t *testing.T, l *Local, scope core.TenantScope) core.Event {
	t.Helper()
	ctx := context.Background()
	var out core.Event
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		evs, err := tx.ReadEvents(ctx, 0, 1000)
		if err != nil {
			return err
		}
		if len(evs) == 0 {
			t.Fatal("no events")
		}
		out = evs[len(evs)-1]
		return nil
	}); err != nil {
		t.Fatalf("reading events: %v", err)
	}
	return out
}
