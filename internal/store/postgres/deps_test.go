package postgres

import (
	"context"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

func TestDependencyPathDetectsCycles(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	a := f.newTask(t, "a", core.PriorityNormal)
	b := f.newTask(t, "b", core.PriorityNormal)
	c := f.newTask(t, "c", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.AddDependency(ctx, &core.Dependency{TaskID: a.ID, DependsOn: b.ID}); err != nil {
			return err
		}
		return tx.AddDependency(ctx, &core.Dependency{TaskID: b.ID, DependsOn: c.ID})
	}); err != nil {
		t.Fatalf("adding dependencies: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		cases := []struct {
			name     string
			from, to string
			want     bool
		}{
			{"direct edge", a.ID, b.ID, true},
			{"transitive edge", a.ID, c.ID, true},
			{"reverse direction", c.ID, a.ID, false},
			{"unrelated", b.ID, a.ID, false},
		}
		for _, tc := range cases {
			got, err := tx.DependencyPathExists(ctx, tc.from, tc.to)
			if err != nil {
				return err
			}
			if got != tc.want {
				t.Fatalf("%s: DependencyPathExists = %v, want %v", tc.name, got, tc.want)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("walking dependencies: %v", err)
	}
}

func TestDependencyRejectsSelfEdgeAndDuplicates(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	a := f.newTask(t, "a", core.PriorityNormal)
	b := f.newTask(t, "b", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.AddDependency(ctx, &core.Dependency{TaskID: a.ID, DependsOn: a.ID}); !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("self dependency = %v, want invalid", err)
		}
		if err := tx.AddDependency(ctx, &core.Dependency{TaskID: a.ID, DependsOn: b.ID}); err != nil {
			return err
		}
		if err := tx.AddDependency(ctx, &core.Dependency{TaskID: a.ID, DependsOn: b.ID}); !core.IsKind(err, core.KindConflict) {
			t.Fatalf("duplicate dependency = %v, want conflict", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("dependency guards: %v", err)
	}
}

func TestBlockedFilterFollowsDependencies(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	blocker := f.newTask(t, "blocker", core.PriorityNormal)
	blocked := f.newTask(t, "blocked", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.AddDependency(ctx, &core.Dependency{TaskID: blocked.ID, DependsOn: blocker.ID})
	}); err != nil {
		t.Fatalf("adding dependency: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: blocked.ID})
		if err != nil {
			return err
		}
		if !got.Blocked || len(got.DependsOn) != 1 || got.DependsOn[0] != blocker.ID {
			t.Fatalf("blocked task = %+v", got)
		}
		blockedOnly, err := tx.ListTasks(ctx, core.TaskFilter{Blocked: core.Yes})
		if err != nil {
			return err
		}
		if len(blockedOnly) != 1 || blockedOnly[0].ID != blocked.ID {
			t.Fatalf("blocked filter = %+v", blockedOnly)
		}
		freeOnly, err := tx.ListTasks(ctx, core.TaskFilter{Blocked: core.No})
		if err != nil {
			return err
		}
		if len(freeOnly) != 1 || freeOnly[0].ID != blocker.ID {
			t.Fatalf("unblocked filter = %+v", freeOnly)
		}
		return nil
	}); err != nil {
		t.Fatalf("blocked filter: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		done, err := tx.GetTask(ctx, core.TaskRef{ID: blocker.ID})
		if err != nil {
			return err
		}
		now := clk.Now()
		done.Status = "done"
		done.CompletedAt = &now
		if err := tx.UpdateTask(ctx, done); err != nil {
			return err
		}
		got, err := tx.GetTask(ctx, core.TaskRef{ID: blocked.ID})
		if err != nil {
			return err
		}
		if got.Blocked {
			t.Fatal("a task whose dependency completed must not read as blocked")
		}
		if err := tx.RemoveDependency(ctx, blocked.ID, blocker.ID); err != nil {
			return err
		}
		deps, err := tx.ListDependencies(ctx, blocked.ID)
		if err != nil {
			return err
		}
		if len(deps) != 0 {
			t.Fatalf("dependencies after removal = %+v", deps)
		}
		return nil
	}); err != nil {
		t.Fatalf("completing the dependency: %v", err)
	}
}
