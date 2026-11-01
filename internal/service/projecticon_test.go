// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

var (
	iconPNG   = []byte("\x89PNG\r\n\x1a\nservice-one")
	iconOther = []byte("\x89PNG\r\n\x1a\nservice-two")
)

func strp(s string) *string { return &s }

func iconRows(t *testing.T, l *Local, scope core.TenantScope, projectID string) bool {
	t.Helper()
	found := false
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		_, err := tx.GetProjectIcon(context.Background(), projectID)
		found = err == nil
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return found
}

func TestCreateProjectWithEachIconKind(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	cases := []struct {
		key  string
		in   core.CreateProjectInput
		kind core.ProjectIconKind
	}{
		{"text", core.CreateProjectInput{Icon: "T"}, core.IconKindText},
		{"named", core.CreateProjectInput{IconName: " Rocket "}, core.IconKindBuiltin},
		{"image", core.CreateProjectInput{IconFile: &core.ProjectIconFile{ContentType: "text/plain", Data: iconPNG}}, core.IconKindImage},
		{"plain", core.CreateProjectInput{IconFile: &core.ProjectIconFile{}}, core.IconKindNone},
	}
	for _, tc := range cases {
		tc.in.Key, tc.in.Name = tc.key, tc.key
		p, err := l.CreateProject(ctx, tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.key, err)
		}
		if p.IconKind() != tc.kind {
			t.Errorf("%s: kind %q, want %q", tc.key, p.IconKind(), tc.kind)
		}
		if got := iconRows(t, l, scope, p.ID); got != (tc.kind == core.IconKindImage) {
			t.Errorf("%s: image row present = %v", tc.key, got)
		}
	}
	named, _ := l.GetProject(ctx, "named")
	if named.IconName != "rocket" {
		t.Errorf("icon name stored as %q, want it normalised", named.IconName)
	}
	img, err := l.GetProjectIcon(ctx, "image")
	if err != nil {
		t.Fatal(err)
	}
	if img.ContentType != "image/png" || !bytes.Equal(img.Data, iconPNG) {
		t.Errorf("image read back as %q, %q", img.ContentType, img.Data)
	}

	bad := []core.CreateProjectInput{
		{Key: "two", Name: "Two", Icon: "T", IconName: "rocket"},
		{Key: "unknown", Name: "Unknown", IconName: "unicorn"},
		{Key: "fake", Name: "Fake", IconFile: &core.ProjectIconFile{ContentType: "image/png", Data: []byte("text")}},
		{Key: "script", Name: "Script", IconFile: &core.ProjectIconFile{Data: []byte(
			`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}},
	}
	for _, in := range bad {
		if _, err := l.CreateProject(ctx, in); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("%s: err = %v, want invalid", in.Key, err)
		}
		if _, err := l.GetProject(ctx, in.Key); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("%s: a refused project was written", in.Key)
		}
	}
}

func TestUpdateProjectIconReplacesTheKindItHad(t *testing.T) {
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "web", Name: "Web", Icon: "W"})
	if err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		name  string
		in    core.UpdateProjectInput
		want  core.Project
		image []byte
	}{
		{"built-in replaces text", core.UpdateProjectInput{IconName: strp("globe")},
			core.Project{IconName: "globe"}, nil},
		{"image replaces built-in", core.UpdateProjectInput{IconFile: &core.ProjectIconFile{Data: iconPNG}},
			core.Project{IconImage: core.ProjectIconDigest(iconPNG)}, iconPNG},
		{"another name leaves the image", core.UpdateProjectInput{Name: strp("Website")},
			core.Project{IconImage: core.ProjectIconDigest(iconPNG)}, iconPNG},
		{"a new image replaces the old", core.UpdateProjectInput{IconFile: &core.ProjectIconFile{Data: iconOther}},
			core.Project{IconImage: core.ProjectIconDigest(iconOther)}, iconOther},
		{"text replaces the image", core.UpdateProjectInput{Icon: strp("W")},
			core.Project{Icon: "W"}, nil},
		{"image again", core.UpdateProjectInput{IconFile: &core.ProjectIconFile{Data: iconPNG}},
			core.Project{IconImage: core.ProjectIconDigest(iconPNG)}, iconPNG},
		{"an empty built-in clears everything", core.UpdateProjectInput{IconName: strp("")},
			core.Project{}, nil},
	}
	for _, step := range steps {
		events, audits := countRows(t, l, scope)
		got, err := l.UpdateProject(ctx, "web", step.in)
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got.Icon != step.want.Icon || got.IconName != step.want.IconName || got.IconImage != step.want.IconImage {
			t.Fatalf("%s: icon = %q / %q / %q, want %q / %q / %q", step.name,
				got.Icon, got.IconName, got.IconImage, step.want.Icon, step.want.IconName, step.want.IconImage)
		}
		img, err := l.GetProjectIcon(ctx, "web")
		switch {
		case step.image == nil && !core.IsKind(err, core.KindNotFound):
			t.Fatalf("%s: reading the icon = %v, want not found", step.name, err)
		case step.image != nil && (err != nil || !bytes.Equal(img.Data, step.image)):
			t.Fatalf("%s: reading the icon = %v", step.name, err)
		}
		if got := iconRows(t, l, scope, p.ID); got != (step.image != nil) {
			t.Fatalf("%s: image row present = %v", step.name, got)
		}
		if after, afterAudits := countRows(t, l, scope); after != events+1 || afterAudits != audits+1 {
			t.Fatalf("%s: events %d -> %d, audits %d -> %d", step.name, events, after, audits, afterAudits)
		}
	}
	if _, err := l.UpdateProject(ctx, "web", core.UpdateProjectInput{Icon: strp("W"), IconName: strp("globe")}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an update naming two kinds = %v, want invalid", err)
	}
}

func TestProjectIconIsReadLikeTheProject(t *testing.T) {
	l, _, ctx := withDefaults(t)
	mine, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "mine", Name: "Mine",
		IconFile: &core.ProjectIconFile{Data: iconPNG}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "other", Name: "Other",
		IconFile: &core.ProjectIconFile{Data: iconOther}}); err != nil {
		t.Fatal(err)
	}
	actor, _ := core.ActorFrom(ctx)
	scoped := core.WithActor(context.Background(), &core.Actor{
		ID: actor.ID, TenantID: actor.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeAll}, ProjectID: mine.ID,
	})
	if _, err := l.GetProjectIcon(scoped, "mine"); err != nil {
		t.Fatalf("its own project's icon: %v", err)
	}
	if _, err := l.GetProjectIcon(scoped, "other"); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("another project's icon = %v, want forbidden", err)
	}
	if _, err := l.GetProjectIcon(ctx, "absent"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("an absent project's icon = %v, want not found", err)
	}

	target, _, _ := newTenant(t, l, "beta")
	if _, err := l.GetProjectIcon(target, "mine"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("another tenant read the icon: %v", err)
	}
}

func TestImportsCarryNamedIconsAndKeepImages(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	if _, err := l.CreateProject(source, core.CreateProjectInput{Key: "named", Name: "Named", IconName: "rocket", WorkflowKey: "flow"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CreateProject(source, core.CreateProjectInput{Key: "pictured", Name: "Pictured", WorkflowKey: "flow",
		IconFile: &core.ProjectIconFile{Data: iconPNG}}); err != nil {
		t.Fatal(err)
	}
	snapshot := exportBytes(t, l, source, core.ExportInput{})

	target, _, _ := newTenant(t, l, "beta")
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import: %v", err)
	}
	named, _ := l.GetProject(target, "named")
	pictured, _ := l.GetProject(target, "pictured")
	if named.IconName != "rocket" {
		t.Errorf("the built-in icon did not travel: %+v", named)
	}
	if pictured.IconKind() != core.IconKindNone {
		t.Errorf("an image digest travelled without its bytes: %+v", pictured)
	}

	if _, err := l.ImportFrom(source, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if img, err := l.GetProjectIcon(source, "pictured"); err != nil || !bytes.Equal(img.Data, iconPNG) {
		t.Fatalf("re-importing lost the image the project had: %v", err)
	}

	if _, err := l.UpdateProject(target, "named", core.UpdateProjectInput{IconFile: &core.ProjectIconFile{Data: iconOther}}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err != nil {
		t.Fatalf("import over an image: %v", err)
	}
	named, _ = l.GetProject(target, "named")
	if named.IconName != "rocket" || named.IconImage != "" {
		t.Fatalf("a named icon in the snapshot did not replace the image: %+v", named)
	}
	if _, err := l.GetProjectIcon(target, "named"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("the replaced image is still readable: %v", err)
	}
}
