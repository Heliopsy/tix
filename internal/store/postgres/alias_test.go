// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// A key a project was renamed from reaches the project and its tasks, in its
// own tenant only, and goes away with the project.
func TestProjectKeyAliases(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	other := seed(t, s, clk, "globex")
	task := f.newTask(t, "kept", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.AddProjectAlias(ctx, "alpha", f.project.ID); err != nil {
			return err
		}
		p := f.project
		p.Key = "omega"
		return tx.UpdateProject(ctx, &p)
	}); err != nil {
		t.Fatalf("renaming: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		p, err := tx.GetProject(ctx, "alpha")
		if err != nil {
			return err
		}
		if p.ID != f.project.ID || p.Key != "omega" {
			t.Errorf("GetProject(alpha) = %q, want the project now keyed omega", p.Key)
		}
		got, err := tx.GetTask(ctx, core.TaskRef{ProjectKey: "alpha", Seq: task.Seq})
		if err != nil {
			return err
		}
		if got.ID != task.ID || got.Ref != "omega-1" {
			t.Errorf("GetTask(alpha-1) = %s, want omega-1", got.Ref)
		}
		if _, err := tx.GetTask(ctx, core.TaskRef{ProjectKey: "alpha", Seq: 99}); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("GetTask(alpha-99) = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading through the alias: %v", err)
	}

	if err := s.View(ctx, other.scope, func(tx store.Tx) error {
		p, err := tx.GetProject(ctx, "alpha")
		if err != nil {
			return err
		}
		if p.ID != other.project.ID {
			t.Error("another tenant's alias reached across the tenant boundary")
		}
		_, err = tx.ProjectAliasTarget(ctx, "alpha")
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("ProjectAliasTarget in another tenant = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the other tenant: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		dropped, err := tx.DeleteProjectAlias(ctx, "alpha")
		if err != nil || !dropped {
			t.Errorf("DeleteProjectAlias = %v, %v, want one dropped", dropped, err)
		}
		if err := tx.AddProjectAlias(ctx, "alpha", f.project.ID); err != nil {
			return err
		}
		return tx.DeleteProject(ctx, f.project.ID)
	}); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		if _, err := tx.ProjectAliasTarget(ctx, "alpha"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("alias after the project was deleted = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading after delete: %v", err)
	}
}
