// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"bytes"
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func readIcon(t *testing.T, f fixture, projectID string) (*core.ProjectIconImage, error) {
	t.Helper()
	var out *core.ProjectIconImage
	err := f.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		out, err = tx.GetProjectIcon(context.Background(), projectID)
		return err
	})
	return out, err
}

func putIcon(t *testing.T, f fixture, projectID string, data []byte) {
	t.Helper()
	img := &core.ProjectIconImage{ProjectID: projectID, ContentType: "image/png",
		Digest: core.ProjectIconDigest(data), Data: data}
	if err := f.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.PutProjectIcon(context.Background(), img)
	}); err != nil {
		t.Fatalf("storing an icon: %v", err)
	}
}

func TestProjectIconImagesRoundTripAndReplace(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	ctx := context.Background()

	if _, err := readIcon(t, f, f.project.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("reading an absent icon = %v, want not found", err)
	}
	first := []byte("\x89PNG\r\n\x1a\nfirst\x00\xff")
	putIcon(t, f, f.project.ID, first)
	second := []byte("\x89PNG\r\n\x1a\nsecond")
	putIcon(t, f, f.project.ID, second)
	got, err := readIcon(t, f, f.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, second) || got.ContentType != "image/png" ||
		got.Digest != core.ProjectIconDigest(second) || got.ProjectID != f.project.ID || got.UpdatedAt.IsZero() {
		t.Fatalf("read back %+v", got)
	}

	p := f.project
	p.Icon, p.IconName, p.IconImage = "", "rocket", ""
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.UpdateProject(ctx, &p); err != nil {
			return err
		}
		return tx.DeleteProjectIcon(ctx, p.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := readIcon(t, f, f.project.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("reading a removed icon = %v, want not found", err)
	}
	var back *core.Project
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		back, err = tx.GetProject(ctx, p.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if back.IconName != "rocket" || back.IconImage != "" {
		t.Fatalf("project read back with icon_name %q, icon_image %q", back.IconName, back.IconImage)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.DeleteProjectIcon(ctx, p.ID)
	}); err != nil {
		t.Fatalf("removing an icon that is not there: %v", err)
	}
}

func TestProjectIconImagesStayInTheirTenant(t *testing.T) {
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	putIcon(t, a, a.project.ID, []byte("\x89PNG\r\n\x1a\nacme"))

	if _, err := readIcon(t, b, a.project.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("another tenant read the icon: %v", err)
	}
	if err := b.store.Update(context.Background(), b.scope, func(tx store.Tx) error {
		return tx.DeleteProjectIcon(context.Background(), a.project.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := readIcon(t, a, a.project.ID); err != nil {
		t.Fatalf("another tenant's delete removed the icon: %v", err)
	}
}

func TestProjectIconImagesGoWithTheirProject(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	putIcon(t, f, f.project.ID, []byte("\x89PNG\r\n\x1a\ngone"))
	if err := s.Update(context.Background(), f.scope, func(tx store.Tx) error {
		return tx.DeleteProject(context.Background(), f.project.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := readIcon(t, f, f.project.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("the icon outlived its project: %v", err)
	}
}
