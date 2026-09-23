// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

type fixture struct {
	store    *Store
	clock    *clock.Fake
	tenant   core.Tenant
	scope    core.TenantScope
	actor    core.Actor
	workflow core.Workflow
	project  core.Project
}

func newStore(t *testing.T) (*Store, *clock.Fake) {
	t.Helper()
	clk := clock.NewFakeAt()
	path := filepath.Join(t.TempDir(), "tix.db")
	s, err := Open(path, clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return s, clk
}

func seed(t *testing.T, s *Store, clk *clock.Fake, key string) fixture {
	t.Helper()
	ctx := context.Background()

	f := fixture{store: s, clock: clk}
	f.tenant = core.Tenant{Key: key, Name: key}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &f.tenant)
	}); err != nil {
		t.Fatalf("creating tenant %q: %v", key, err)
	}
	f.scope = core.TenantScope{TenantID: f.tenant.ID}

	f.workflow = core.Workflow{
		Key:  "default",
		Name: "Default",
		Definition: core.WorkflowDefinition{
			Initial: "todo",
			States: []core.State{
				{Key: "todo", Label: "To do"},
				{Key: "doing", Label: "Doing"},
				{Key: "done", Label: "Done", Terminal: true},
			},
			Transitions: []core.Transition{{From: "todo", To: "doing"}, {From: "doing", To: "done"}},
		},
	}
	f.actor = core.Actor{Kind: core.ActorAgent, Handle: "worker", DisplayName: "Worker"}
	f.project = core.Project{Key: "alpha", Name: "Alpha"}

	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &f.workflow); err != nil {
			return err
		}
		if err := tx.CreateActor(ctx, &f.actor); err != nil {
			return err
		}
		f.project.WorkflowID = f.workflow.ID
		return tx.CreateProject(ctx, &f.project)
	})
	if err != nil {
		t.Fatalf("seeding tenant %q: %v", key, err)
	}
	return f
}

func (f fixture) newTask(t *testing.T, title string, priority core.Priority) core.Task {
	t.Helper()
	ctx := context.Background()
	task := core.Task{
		ProjectID:      f.project.ID,
		Title:          title,
		Status:         "todo",
		Priority:       priority,
		CreatorActorID: f.actor.ID,
	}
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("creating task %q: %v", title, err)
	}
	return task
}

func TestMigrateOnFreshDatabase(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	v, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}
	if v < 1 {
		t.Fatalf("schema version = %d, want at least 1", v)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	again, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("schema version after rerun: %v", err)
	}
	if again != v {
		t.Fatalf("schema version moved from %d to %d on a second run", v, again)
	}
	if s.Dialect() != store.SQLite {
		t.Fatalf("dialect = %q, want %q", s.Dialect(), store.SQLite)
	}
	if err := s.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open("  ", clock.NewFakeAt()); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Open(\"\") error = %v, want invalid", err)
	}
}

// TestOpenAllowsNetworkFSOptionOnLocalDisk cannot exercise the refusal path
// itself, which needs a real network mount; that decision is covered
// exhaustively without touching the OS in internal/storeloc. This only
// proves the option plumbs through without breaking an ordinary open, on
// disk or overridden.
func TestOpenAllowsNetworkFSOptionOnLocalDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tix.db")
	s, err := Open(path, clock.NewFakeAt(), WithAllowNetworkFS(true))
	if err != nil {
		t.Fatalf("Open with WithAllowNetworkFS(true) on local disk: %v", err)
	}
	_ = s.Close()
}

func TestScopeRequired(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	if _, err := s.Begin(ctx, core.TenantScope{}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Begin without scope = %v, want invalid", err)
	}
	if err := s.View(ctx, core.TenantScope{}, func(store.Tx) error { return nil }); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("View without scope = %v, want invalid", err)
	}
	if err := s.Update(ctx, core.TenantScope{}, func(store.Tx) error { return nil }); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("Update without scope = %v, want invalid", err)
	}
}

func TestBeginCommitAndRollback(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	tx, err := s.Begin(ctx, f.scope)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if tx.Scope() != f.scope {
		t.Fatalf("scope = %+v, want %+v", tx.Scope(), f.scope)
	}
	task := core.Task{ProjectID: f.project.ID, Title: "rolled back", Status: "todo",
		Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
	if err := tx.CreateTask(ctx, &task); err != nil {
		t.Fatalf("create in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("second rollback: %v", err)
	}

	err = s.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("task after rollback = %v, want not found", err)
	}

	tx2, err := s.Begin(ctx, f.scope)
	if err != nil {
		t.Fatalf("begin again: %v", err)
	}
	kept := core.Task{ProjectID: f.project.ID, Title: "committed", Status: "todo",
		Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
	if err := tx2.CreateTask(ctx, &kept); err != nil {
		t.Fatalf("create in tx2: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := tx2.Commit(); err == nil {
		t.Fatal("committing twice should fail")
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: kept.ID})
		if err != nil {
			return err
		}
		if got.Title != "committed" {
			t.Fatalf("title = %q, want %q", got.Title, "committed")
		}
		return nil
	}); err != nil {
		t.Fatalf("view after commit: %v", err)
	}
}

func TestUpdateRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	sentinel := errors.New("stop")
	var task core.Task
	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		task = core.Task{ProjectID: f.project.ID, Title: "doomed", Status: "todo",
			Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
		if err := tx.CreateTask(ctx, &task); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Update error = %v, want the sentinel", err)
	}
	err = s.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("task after failed update = %v, want not found", err)
	}
}
