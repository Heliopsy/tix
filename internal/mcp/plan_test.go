// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/heliopsy/tix/api"
)

// schemaBudget bounds one tool's input schema. Clients send every schema on
// every turn, so a type that grows past this belongs behind "body" or a
// hidden parameter rather than in every prompt.
const schemaBudget = 16 << 10

func shippedDocument(t *testing.T) *Document {
	t.Helper()
	raw, err := api.JSON()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := LoadDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func shippedPlan(t *testing.T, opts Options) []Tool {
	t.Helper()
	ov, err := BuiltinOverlay()
	if err != nil {
		t.Fatalf("reading the shipped overlay: %v", err)
	}
	tools, err := Plan(shippedDocument(t), ov, opts)
	if err != nil {
		t.Fatalf("planning the shipped tools: %v", err)
	}
	return tools
}

// TestEveryOperationIsAToolOrHiddenWithAReason is what keeps an endpoint from
// going missing silently: a new JSON operation becomes a tool by default, and
// one that does not says why on the listing.
func TestEveryOperationIsAToolOrHiddenWithAReason(t *testing.T) {
	doc := shippedDocument(t)
	tools := shippedPlan(t, Options{})
	if len(tools) != len(doc.Operations) {
		t.Fatalf("planned %d tools for %d operations", len(tools), len(doc.Operations))
	}
	for _, tool := range tools {
		switch tool.State {
		case StateExposed:
		case StateHidden, StateDestructive:
			if strings.TrimSpace(tool.Reason) == "" {
				t.Errorf("%s is %s with no reason given", tool.Op.ID, tool.State)
			}
		default:
			t.Errorf("%s ended in state %q with no scope filter applied", tool.Op.ID, tool.State)
		}
	}
	if len(Exposed(tools)) == 0 {
		t.Fatal("the shipped plan exposes nothing")
	}
}

func TestTheShippedOverlayHoldsToTheDocument(t *testing.T) {
	ov, err := BuiltinOverlay()
	if err != nil {
		t.Fatal(err)
	}
	if err := ov.Check(shippedDocument(t)); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayRefusesWhatItCannotHonour(t *testing.T) {
	doc := shippedDocument(t)
	for _, tc := range []struct {
		name, yaml, want string
	}{
		{"unknown operation", "operations:\n  task.nope: {hide: true, reason: gone}\n", `"task.nope", which is not an operation`},
		{"hidden without reason", "operations:\n  task.show: {hide: true}\n", `hides "task.show" without a reason`},
		{"unmarked delete without reason", "operations:\n  task.delete: {destructive: false}\n", `"task.delete" is destructive without a reason`},
		{"marked post without reason", "operations:\n  task.create: {destructive: true}\n", `"task.create" is destructive without a reason`},
		{"read-only without reason", "operations:\n  task.export: {read_only: true}\n", `marks "task.export" read-only without a reason`},
		{"read-only and destructive", "operations:\n  task.bulk: {read_only: true, destructive: true, reason: x}\n", `both read-only and destructive`},
		{"unknown parameter", "operations:\n  task.list: {hide_params: [nope]}\n", `parameter "nope" of "task.list"`},
		{"required parameter", "operations:\n  task.show: {hide_params: [ref]}\n", `parameter "ref" of "task.show"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ov, err := ParseOverlay([]byte(tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			err = ov.Check(doc)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Check = %v, want an error naming %s", err, tc.want)
			}
		})
	}
	if _, err := ParseOverlay([]byte("operations:\n  task.show: {hidden: true}\n")); err == nil {
		t.Error("an overlay with a misspelt key parsed; it would be silently ignored")
	}
}

func TestToolNamesAreUniqueAndValid(t *testing.T) {
	seen := map[string]string{}
	for _, tool := range shippedPlan(t, Options{}) {
		if !validName.MatchString(tool.Name) {
			t.Errorf("%s is named %q, which not every client accepts", tool.Op.ID, tool.Name)
		}
		if prev, dup := seen[tool.Name]; dup {
			t.Errorf("%s and %s are both named %q", prev, tool.Op.ID, tool.Name)
		}
		seen[tool.Name] = tool.Op.ID
	}

	ov := Overlay{Operations: map[string]Entry{"task.show": {Name: "task_list"}}}
	if _, err := Plan(shippedDocument(t), ov, Options{}); err == nil || !strings.Contains(err.Error(), `"task_list" is taken`) {
		t.Errorf("renaming task.show onto task_list planned without error: %v", err)
	}
	ov = Overlay{Operations: map[string]Entry{"task.show": {Name: "task.show"}}}
	if _, err := Plan(shippedDocument(t), ov, Options{}); err == nil || !strings.Contains(err.Error(), "not a valid MCP tool name") {
		t.Errorf("a dotted tool name planned without error: %v", err)
	}
}

func TestInputSchemasAreValidAndWithinBudget(t *testing.T) {
	for _, tool := range shippedPlan(t, Options{AllowDestructive: true}) {
		if tool.State == StateHidden && tool.Schema == nil {
			continue
		}
		raw, err := json.Marshal(tool.Schema)
		if err != nil {
			t.Fatalf("%s: %v", tool.Name, err)
		}
		if len(raw) > schemaBudget {
			t.Errorf("%s's input schema is %d bytes, over the %d budget", tool.Name, len(raw), schemaBudget)
		}
		if tool.Schema["type"] != "object" {
			t.Errorf("%s's input schema is not an object", tool.Name)
		}
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("%s: %v", tool.Name, err)
		}
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		if err := c.AddResource(tool.Name+".json", doc); err != nil {
			t.Fatalf("%s: %v", tool.Name, err)
		}
		if _, err := c.Compile(tool.Name + ".json"); err != nil {
			t.Errorf("%s's input schema is not valid JSON Schema: %v", tool.Name, err)
		}
		if strings.Contains(string(raw), `"$ref"`) {
			t.Errorf("%s's input schema still holds a $ref", tool.Name)
		}
	}
}

func TestDestructiveToolsAreOptIn(t *testing.T) {
	byOp := func(tools []Tool) map[string]Tool {
		out := map[string]Tool{}
		for _, tool := range tools {
			out[tool.Op.ID] = tool
		}
		return out
	}
	off := byOp(shippedPlan(t, Options{}))
	on := byOp(shippedPlan(t, Options{AllowDestructive: true}))
	for id, tool := range off {
		if tool.Op.Method == http.MethodDelete && tool.Op.Unsupported == "" && !tool.Destructive {
			if _, overridden := mustOverlay(t).Operations[id]; !overridden {
				t.Errorf("%s is a DELETE the overlay never marked, yet it is not destructive", id)
			}
		}
		if tool.Destructive && tool.State == StateExposed {
			t.Errorf("%s is destructive and exposed without --allow-destructive", id)
		}
		if tool.State == StateDestructive && on[id].State != StateExposed {
			t.Errorf("%s stays %s with --allow-destructive", id, on[id].State)
		}
	}
	for _, id := range []string{"task.delete", "project.delete", "tenant.delete", "token.revoke", "retention.prune", "task.bulk"} {
		if off[id].State != StateDestructive {
			t.Errorf("%s is %s by default, want destructive", id, off[id].State)
		}
	}
	if off["task.unstar"].State != StateExposed {
		t.Errorf("task.unstar is %s, want exposed: the overlay marks it reversible", off["task.unstar"].State)
	}
}

func mustOverlay(t *testing.T) Overlay {
	t.Helper()
	ov, err := BuiltinOverlay()
	if err != nil {
		t.Fatal(err)
	}
	return ov
}

func TestScopeFilterHidesWhatTheTokenCannotCall(t *testing.T) {
	tools := shippedPlan(t, Options{HasScope: func(s string) bool { return s == "task:read" }})
	for _, tool := range tools {
		if tool.State == StateHidden || tool.State == StateDestructive {
			continue
		}
		want := StateExposed
		if tool.Op.Scope != "" && tool.Op.Scope != "task:read" {
			want = StateOutOfScope
		}
		if tool.State != want {
			t.Errorf("%s (scope %q) is %s, want %s", tool.Op.ID, tool.Op.Scope, tool.State, want)
		}
	}
}

func TestOverlayFileLaysOverTheShippedOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overlay.yaml")
	if err := os.WriteFile(path, []byte("operations:\n  task.delete:\n    enable: true\n  task.show:\n    name: show_task\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ov, err := LoadOverlay(path)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Operations["session.login"].Reason == "" {
		t.Error("the user file replaced the shipped overlay instead of laying over it")
	}
	tools, err := Plan(shippedDocument(t), ov, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		switch tool.Op.ID {
		case "task.delete":
			if tool.State != StateExposed {
				t.Errorf("task.delete enabled by the overlay is %s", tool.State)
			}
		case "task.show":
			if tool.Name != "show_task" {
				t.Errorf("task.show is named %q, want show_task", tool.Name)
			}
		}
	}
	if _, err := LoadOverlay(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("a missing overlay file loaded")
	}
}

// TestEveryParameterArgumentIsDescribed holds each tool argument that fills a
// path or query parameter to the description the document gives it, so a
// model choosing a value reads what the parameter takes, not a bare name.
func TestEveryParameterArgumentIsDescribed(t *testing.T) {
	checked := 0
	for _, tool := range shippedPlan(t, Options{AllowDestructive: true}) {
		props, _ := tool.Schema["properties"].(map[string]any)
		for name, p := range tool.args {
			if p == nil {
				continue
			}
			checked++
			prop, _ := props[name].(map[string]any)
			got, _ := prop["description"].(string)
			if got == "" || got != p.Description {
				t.Errorf("%s: argument %q has description %q, want the parameter's %q", tool.Name, name, got, p.Description)
			}
		}
	}
	if checked < 100 {
		t.Fatalf("checked %d parameter arguments, which is not all of them", checked)
	}
}
