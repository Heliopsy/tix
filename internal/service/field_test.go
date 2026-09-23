// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func withProject(t *testing.T) (*Local, core.TenantScope, context.Context, *core.Project) {
	t.Helper()
	l, scope, ctx := withDefaults(t)
	p, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "infra", Name: "Infrastructure"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return l, scope, ctx, p
}

func TestPutFieldDefCreatesAndReplaces(t *testing.T) {
	l, scope, ctx, p := withProject(t)
	events, audits := countRows(t, l, scope)

	def, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{
		Key:         "severity",
		Type:        core.FieldEnum,
		EnumOptions: []string{"low", "high"},
		Required:    true,
		Position:    2,
	})
	if err != nil {
		t.Fatalf("PutFieldDef: %v", err)
	}
	if def.ProjectID != p.ID || def.Label != "severity" {
		t.Errorf("definition = %+v, want the project set and the key used as the tag", def)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("PutFieldDef did not write both an audit entry and an event")
	}

	again, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{
		Key:         "severity",
		Label:       "Severity",
		Type:        core.FieldEnum,
		EnumOptions: []string{"low", "medium", "high"},
	})
	if err != nil {
		t.Fatalf("replacing a field definition: %v", err)
	}
	if again.ID != def.ID {
		t.Error("replacing a definition changed its identifier")
	}
	if len(again.EnumOptions) != 3 || again.Required {
		t.Errorf("replaced definition = %+v", again)
	}
}

func TestPutFieldDefValidation(t *testing.T) {
	l, _, ctx, _ := withProject(t)

	tests := []struct {
		name string
		in   core.FieldDefInput
		kind core.Kind
	}{
		{"no key", core.FieldDefInput{Type: core.FieldString}, core.KindInvalid},
		{"unknown type", core.FieldDefInput{Key: "x", Type: core.FieldType("colour")}, core.KindInvalid},
		{"enum without options", core.FieldDefInput{Key: "x", Type: core.FieldEnum}, core.KindInvalid},
		{"options on a non-enum", core.FieldDefInput{Key: "x", Type: core.FieldString, EnumOptions: []string{"a"}}, core.KindInvalid},
		{"default of the wrong type", core.FieldDefInput{Key: "x", Type: core.FieldInt, Default: "many"}, core.KindInvalid},
		{"default outside the options", core.FieldDefInput{
			Key: "x", Type: core.FieldEnum, EnumOptions: []string{"low"}, Default: "high",
		}, core.KindInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.PutFieldDef(ctx, "infra", tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("PutFieldDef = %v, want %s", err, tc.kind)
			}
		})
	}

	if _, err := l.PutFieldDef(ctx, "missing", core.FieldDefInput{Key: "x", Type: core.FieldString}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("PutFieldDef on an unknown project = %v, want not found", err)
	}
}

func TestPutFieldDefAcceptsAValidDefault(t *testing.T) {
	l, _, ctx, _ := withProject(t)

	def, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{
		Key: "severity", Type: core.FieldEnum, EnumOptions: []string{"low", "high"}, Default: "low",
	})
	if err != nil {
		t.Fatalf("PutFieldDef: %v", err)
	}
	if def.Default != "low" {
		t.Errorf("default = %v, want low", def.Default)
	}
}

func TestListAndDeleteFieldDefs(t *testing.T) {
	l, scope, ctx, _ := withProject(t)
	for _, key := range []string{"severity", "component"} {
		if _, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{Key: key, Type: core.FieldString}); err != nil {
			t.Fatalf("PutFieldDef(%q): %v", key, err)
		}
	}

	defs, err := l.ListFieldDefs(ctx, "INFRA")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("ListFieldDefs = %d, want 2", len(defs))
	}

	events, audits := countRows(t, l, scope)
	if err := l.DeleteFieldDef(ctx, "infra", "severity"); err != nil {
		t.Fatalf("DeleteFieldDef: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("DeleteFieldDef did not write both an audit entry and an event")
	}
	if err := l.DeleteFieldDef(ctx, "infra", "severity"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting twice = %v, want not found", err)
	}
	if _, err := l.ListFieldDefs(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("ListFieldDefs on an unknown project = %v, want not found", err)
	}
	if err := l.DeleteFieldDef(ctx, "missing", "severity"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("DeleteFieldDef on an unknown project = %v, want not found", err)
	}
}

// The same key may exist in two projects without the definitions interfering.
func TestFieldDefsAreScopedToTheirProject(t *testing.T) {
	l, _, ctx, _ := withProject(t)
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "ops", Name: "Ops"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if _, err := l.PutFieldDef(ctx, "infra", core.FieldDefInput{
		Key: "severity", Type: core.FieldEnum, EnumOptions: []string{"low"},
	}); err != nil {
		t.Fatalf("PutFieldDef: %v", err)
	}
	if _, err := l.PutFieldDef(ctx, "ops", core.FieldDefInput{
		Key: "severity", Type: core.FieldString,
	}); err != nil {
		t.Fatalf("PutFieldDef in the second project: %v", err)
	}

	infra, err := l.ListFieldDefs(ctx, "infra")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	if len(infra) != 1 || infra[0].Type != core.FieldEnum {
		t.Errorf("infra definitions = %+v", infra)
	}
	ops, err := l.ListFieldDefs(ctx, "ops")
	if err != nil {
		t.Fatalf("ListFieldDefs: %v", err)
	}
	if len(ops) != 1 || ops[0].Type != core.FieldString {
		t.Errorf("ops definitions = %+v", ops)
	}
}

func TestFieldDefWritesRequireProjectWriteScope(t *testing.T) {
	l, _, ctx, _ := withProject(t)
	actor, _ := core.ActorFrom(ctx)
	member := &core.Actor{ID: "m1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	memberCtx := core.WithActor(context.Background(), member)

	if _, err := l.PutFieldDef(memberCtx, "infra", core.FieldDefInput{Key: "x", Type: core.FieldString}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member defining a field = %v, want forbidden", err)
	}
	if err := l.DeleteFieldDef(memberCtx, "infra", "x"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("member deleting a field = %v, want forbidden", err)
	}
	if _, err := l.ListFieldDefs(memberCtx, "infra"); err != nil {
		t.Errorf("member listing fields = %v, want allowed", err)
	}
}
