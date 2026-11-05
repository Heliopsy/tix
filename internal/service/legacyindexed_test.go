// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// withLegacyIndexed writes "indexed":true into every field definition the way
// a build before the flag was removed exported it.
func withLegacyIndexed(t *testing.T, raw []byte, anchor string) []byte {
	t.Helper()
	if !bytes.Contains(raw, []byte(anchor)) {
		t.Fatalf("the document has no %s to put the legacy key beside:\n%s", anchor, raw)
	}
	return bytes.ReplaceAll(raw, []byte(anchor), []byte(`"indexed":true,`+anchor))
}

func TestAnOldBundleWithIndexedStillImports(t *testing.T) {
	f := newBundleFixture(t)
	raw := f.export(t, core.BundleExportInput{
		Name:  "team",
		Kinds: []core.ComponentKind{core.ComponentWorkflow, core.ComponentTag, core.ComponentProject},
	})
	legacy := withLegacyIndexed(t, raw, `"key":"points"`)
	other := newBundleTenant(t, f.l)

	if _, err := f.l.ImportBundle(other, bytes.NewReader(legacy), core.BundleImportInput{
		OnCollision: core.CollisionSkip,
	}); err != nil {
		t.Fatalf("importing a bundle that carries indexed: %v", err)
	}
	defs, err := f.l.ListFieldDefs(other, "acme")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	if len(defs) != 1 || defs[0].Key != "points" {
		t.Errorf("field definitions = %+v, want points", defs)
	}
}

func TestAnOldSnapshotWithIndexedStillImports(t *testing.T) {
	l, _, _, actor := newLocal(t)
	source := core.WithActor(context.Background(), actor)
	seed(t, l, source, "infra")
	snapshot := exportBytes(t, l, source, core.ExportInput{})
	legacy := withLegacyIndexed(t, snapshot, `"required":`)

	target, _, _ := newTenant(t, l, "beta")
	result, err := l.ImportFrom(target, bytes.NewReader(legacy), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("importing a snapshot that carries indexed: %v", err)
	}
	if result.Created[string(core.RecordFieldDef)] != 1 {
		t.Errorf("created = %v, want the field definition", result.Created)
	}
	if strings.Contains(string(exportBytes(t, l, target, core.ExportInput{})), `"indexed"`) {
		t.Error("the re-exported snapshot still carries indexed")
	}
}
