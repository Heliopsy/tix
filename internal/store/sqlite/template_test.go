// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestTemplatesRoundTripPageAndStayInTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")

	write := func(f fixture, fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.Update(ctx, f.scope, fn); err != nil {
			t.Fatal(err)
		}
	}
	list := func(f fixture, q store.TemplateQuery) []core.TaskTemplate {
		t.Helper()
		var out []core.TaskTemplate
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			out, err = tx.ListTemplates(ctx, q)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}

	release := &core.TaskTemplate{
		Name: "Release", ProjectID: a.project.ID, DueIn: core.Duration(72 * time.Hour),
		Task: core.TemplateTask{Title: "Ship {N}", Tags: []string{"ops"}, CustomFields: map[string]any{"points": float64(3)},
			Subtasks: []core.TemplateTask{{Title: "Notes"}}},
	}
	write(a, func(tx store.Tx) error {
		if err := tx.CreateTemplate(ctx, release); err != nil {
			return err
		}
		return tx.CreateTemplate(ctx, &core.TaskTemplate{Name: "audit", Task: core.TemplateTask{Title: "Audit"}})
	})

	var got *core.TaskTemplate
	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		var err error
		got, err = tx.GetTemplate(ctx, "RELEASE")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.ID != release.ID || got.ProjectKey != a.project.Key || got.DueIn != release.DueIn ||
		got.Task.Title != "Ship {N}" || len(got.Task.Subtasks) != 1 || got.Task.CustomFields["points"] != float64(3) {
		t.Fatalf("read back %+v", got)
	}

	if all := list(a, store.TemplateQuery{}); len(all) != 2 || all[0].Name != "audit" || all[1].Name != "Release" {
		t.Fatalf("listing = %+v, want audit then Release", all)
	}
	if first := list(a, store.TemplateQuery{Page: core.Page{Limit: 1}}); len(first) != 1 || first[0].Name != "audit" {
		t.Fatalf("first page = %+v", first)
	}
	cursor := core.Cursor{SortValue: "audit", ID: list(a, store.TemplateQuery{})[0].ID, Sort: "name", Direction: core.Ascending}.Encode()
	if next := list(a, store.TemplateQuery{Page: core.Page{Limit: 1, Sort: "name", Cursor: cursor}}); len(next) != 1 || next[0].Name != "Release" {
		t.Fatalf("second page = %+v", next)
	}
	if other := list(b, store.TemplateQuery{}); len(other) != 0 {
		t.Fatalf("another tenant lists %+v", other)
	}

	dup := &core.TaskTemplate{Name: "release", Task: core.TemplateTask{Title: "x"}}
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.CreateTemplate(ctx, dup) }); !core.IsKind(err, core.KindConflict) {
		t.Errorf("a name differing only by case = %v, want conflict", err)
	}

	write(a, func(tx store.Tx) error {
		got.Name, got.ProjectID, got.Uses = "ship", "", 4
		return tx.UpdateTemplate(ctx, got)
	})
	if scoped := list(a, store.TemplateQuery{ProjectID: "none"}); len(scoped) != 2 {
		t.Fatalf("both templates are tenant-wide now, a project filter lists %+v", scoped)
	}
	write(b, func(tx store.Tx) error {
		if err := tx.DeleteTemplate(ctx, release.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("another tenant deleting = %v, want not found", err)
		}
		return nil
	})
	write(a, func(tx store.Tx) error { return tx.DeleteTemplate(ctx, release.ID) })
	if left := list(a, store.TemplateQuery{}); len(left) != 1 || left[0].Name != "audit" {
		t.Fatalf("after delete %+v", left)
	}
}
