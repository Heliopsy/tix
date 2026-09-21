package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestProjectAppearanceRoundTrip covers the colour and icon through a create,
// an edit and a clear, and checks the clear leaves the rest of the row alone.
func TestProjectAppearanceRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		plain, err := tx.GetProject(ctx, f.project.ID)
		if err != nil {
			return err
		}
		if plain.Color != core.ColorNone || plain.Icon != "" {
			t.Fatalf("seeded project = %q/%q, want neither", plain.Color, plain.Icon)
		}

		marked := core.Project{Key: "beta", Name: "Beta", Description: "second",
			WorkflowID: f.workflow.ID, Color: core.ColorBlue, Icon: "\U0001F680"}
		if err := tx.CreateProject(ctx, &marked); err != nil {
			return err
		}
		got, err := tx.GetProject(ctx, "beta")
		if err != nil {
			return err
		}
		if got.Color != core.ColorBlue || got.Icon != "\U0001F680" {
			t.Fatalf("created = %q/%q, want blue/rocket", got.Color, got.Icon)
		}

		got.Color, got.Icon = core.ColorPink, "BE"
		if err := tx.UpdateProject(ctx, got); err != nil {
			return err
		}
		if got, err = tx.GetProject(ctx, "beta"); err != nil {
			return err
		}
		if got.Color != core.ColorPink || got.Icon != "BE" {
			t.Fatalf("edited = %q/%q, want pink/BE", got.Color, got.Icon)
		}

		got.Color, got.Icon = core.ColorNone, ""
		if err := tx.UpdateProject(ctx, got); err != nil {
			return err
		}
		if got, err = tx.GetProject(ctx, "beta"); err != nil {
			return err
		}
		if got.Color != core.ColorNone || got.Icon != "" {
			t.Fatalf("cleared = %q/%q, want neither", got.Color, got.Icon)
		}
		if got.Name != "Beta" || got.Description != "second" {
			t.Fatalf("clearing disturbed the project: %+v", got)
		}

		listed, err := tx.ListProjects(ctx, core.ProjectFilter{Keys: []string{"beta"}})
		if err != nil {
			return err
		}
		if len(listed) != 1 || listed[0].Color != core.ColorNone {
			t.Fatalf("listed = %+v", listed)
		}
		return nil
	}); err != nil {
		t.Fatalf("project appearance: %v", err)
	}
}

// TestProjectAppearanceStaysInsideTheTenant checks the new columns are read
// through the tenant scope like every other column.
func TestProjectAppearanceStaysInsideTheTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	one := seed(t, s, clk, "one")
	two := seed(t, s, clk, "two")

	if err := s.Update(ctx, one.scope, func(tx store.Tx) error {
		p, err := tx.GetProject(ctx, one.project.ID)
		if err != nil {
			return err
		}
		p.Color, p.Icon = core.ColorTeal, "\U0001F9EA"
		return tx.UpdateProject(ctx, p)
	}); err != nil {
		t.Fatalf("marking the first tenant's project: %v", err)
	}

	if err := s.View(ctx, two.scope, func(tx store.Tx) error {
		if _, err := tx.GetProject(ctx, one.project.ID); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("cross-tenant read = %v, want not found", err)
		}
		listed, err := tx.ListProjects(ctx, core.ProjectFilter{})
		if err != nil {
			return err
		}
		for _, p := range listed {
			if p.Color == core.ColorTeal || p.Icon == "\U0001F9EA" {
				t.Fatalf("project %q leaked another tenant's appearance", p.Key)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading as the second tenant: %v", err)
	}
}
