// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestImportExportReadsAsOneScreen pins what was actually wrong with these two
// pages. Not that they were separate, but that nothing on either said how they
// differed, so the only way to learn that one moves work and the other moves
// configuration was to export one and read the file.
func TestImportExportReadsAsOneScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, tc := range []struct {
		path    string
		says    string
		points  string
		current string
	}{
		{"/transfer", "work", "Components", `href="/transfer" aria-current="page"`},
		{"/bundles", "configuration", "Snapshot", `href="/bundles" aria-current="page"`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			page := b.page(tc.path)
			if !strings.Contains(page, `href="/transfer"`) || !strings.Contains(page, `href="/bundles"`) {
				t.Fatal("the page does not offer both tabs, so neither names the other")
			}
			if !strings.Contains(page, tc.current) {
				t.Errorf("the tab strip does not mark %s as the current one", tc.path)
			}
			if !strings.Contains(page, tc.says) {
				t.Errorf("the page never says it moves %q, which is the whole distinction", tc.says)
			}
			if !strings.Contains(page, tc.points) {
				t.Errorf("the page does not point at %s, so the pair is not discoverable", tc.points)
			}
		})
	}
}

// TestImportExportExplainsWhatItOverwrites is the other half of the complaint.
// A destructive mode that does not say it deletes anything is a trap, and
// "replace" means two different things on these two screens.
func TestImportExportExplainsWhatItOverwrites(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	snapshot := b.page("/transfer")
	for _, want := range []string{"Merge", "Replace", "deletes", "dry run"} {
		if !strings.Contains(snapshot, want) {
			t.Errorf("the snapshot screen never mentions %q", want)
		}
	}
	if !strings.Contains(snapshot, "never exported") {
		t.Error("the snapshot screen does not say credentials stay behind")
	}

	components := b.page("/bundles")
	for _, want := range []string{"skip", "rename", "replace", "preview"} {
		if !strings.Contains(components, want) {
			t.Errorf("the components screen never mentions the %q choice", want)
		}
	}
	if !strings.Contains(components, "Nothing here is deleted") {
		t.Error("the components screen does not say it deletes nothing, " +
			"which is what makes it different from a snapshot replace")
	}
}

// TestTenantShowsItsShape pins the diagram, including the part that makes it
// worth having: live counts rather than a drawing of the model.
//
// A picture of an empty tenant answers a question nobody has. "Domains 0" is
// how somebody works out why their hostname does not resolve, so the count
// has to be real and a zero has to render.
func TestTenantShowsItsShape(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/admin/tenant")

	if !strings.Contains(page, `class="shape"`) {
		t.Fatal("the tenant page has no shape diagram")
	}
	for _, kind := range []string{"Members", "Domains", "API tokens", "Workflows", "Projects", "Tasks"} {
		if !strings.Contains(page, kind) {
			t.Errorf("the diagram never mentions %q", kind)
		}
	}
	// Nesting is in the markup, not only in the indent, so a reader with no
	// styles still learns that tasks sit under projects.
	if !strings.Contains(page, `class="depth-2"`) {
		t.Error("nothing is drawn as nested, so the diagram states no hierarchy")
	}
	// The kinds that have somewhere to go, go there.
	for _, href := range []string{`href="/admin/users"`, `href="/admin/domains"`, `href="/projects"`} {
		if !strings.Contains(page, href) {
			t.Errorf("the diagram does not link %s", href)
		}
	}
	// A zero must render rather than being mistaken for absent. A fresh
	// fixture has no domains, so this is the live zero.
	if !strings.Contains(page, `<span class="count">0</span>`) {
		t.Error("a count of zero is not rendered, so an empty kind looks unmeasured")
	}
}
