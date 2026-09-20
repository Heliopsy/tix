package core

import "testing"

func TestComponentKindValid(t *testing.T) {
	for _, k := range ComponentKinds {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	for _, k := range []ComponentKind{"task", "comment", "", "saved_filter"} {
		if k.Valid() {
			t.Errorf("%q must not be a shareable component kind", k)
		}
	}
}

// A bundle carries configuration only. If a work-item kind ever became
// shareable, sharing a workflow could disclose what people are working on.
func TestComponentKindsExcludeWorkItems(t *testing.T) {
	for _, k := range ComponentKinds {
		switch k {
		case "task", "comment", "artifact", "audit_entry", "event", "dependency":
			t.Errorf("%q is a work item and must not be shareable", k)
		}
	}
}

func TestBundleImportInputRequiresACollisionPolicy(t *testing.T) {
	if err := (BundleImportInput{}).Validate(); err == nil {
		t.Error("an import with no collision policy must be refused")
	}
	for _, p := range []CollisionPolicy{CollisionSkip, CollisionRename, CollisionReplace} {
		if err := (BundleImportInput{OnCollision: p}).Validate(); err != nil {
			t.Errorf("policy %q should be accepted, got %v", p, err)
		}
	}
	if err := (BundleImportInput{OnCollision: "overwrite"}).Validate(); err == nil {
		t.Error("an unknown collision policy must be refused")
	}
}

func TestBundleExportInputValidate(t *testing.T) {
	if err := (BundleExportInput{Kinds: []ComponentKind{ComponentWorkflow}}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (BundleExportInput{Kinds: []ComponentKind{"task"}}).Validate(); err == nil {
		t.Error("an unshareable kind must be refused")
	}
	if err := (BundleExportInput{}).Validate(); err != nil {
		t.Errorf("an empty selection should be valid, got %v", err)
	}
}
