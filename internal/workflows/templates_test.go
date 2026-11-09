// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/workflows"
)

// wantKeys is the shipped set, in offering order.
var wantKeys = []string{
	"tix-kanban", "tix-gtd", "tix-dev-review", "tix-bugs", "tix-releases", "tix-agent-review",
	"tix-incident", "tix-support", "tix-approval", "tix-sales", "tix-hiring", "tix-content",
}

func TestEveryTemplateValidates(t *testing.T) {
	t.Parallel()
	all, err := workflows.Templates()
	if err != nil {
		t.Fatalf("loading templates: %v", err)
	}
	var keys []string
	for _, tpl := range all {
		keys = append(keys, tpl.Key)
		if strings.TrimSpace(tpl.Key) == "" || strings.TrimSpace(tpl.Name) == "" || strings.TrimSpace(tpl.Purpose) == "" {
			t.Errorf("template %+v has an empty key, name or purpose", tpl)
		}
		if !strings.HasPrefix(tpl.Key, core.WorkflowTemplatePrefix) {
			t.Errorf("%s does not carry the reserved prefix", tpl.Key)
		}
		if strings.HasPrefix(tpl.Name, core.WorkflowTemplatePrefix) {
			t.Errorf("%s: the display name %q carries the key prefix", tpl.Key, tpl.Name)
		}
		in := core.WorkflowInput{Key: tpl.Key, Name: tpl.Name, Definition: tpl.Definition}
		if err := in.Validate(); err != nil {
			t.Errorf("%s: %v", tpl.Key, err)
		}
		for _, s := range tpl.Definition.States {
			if s.Category == "" || s.Label == "" {
				t.Errorf("%s: state %q names no category or label", tpl.Key, s.Key)
			}
			if s.Terminal && !slices.Contains([]core.StateCategory{core.CategoryDone, core.CategoryCancelled}, s.Category) {
				t.Errorf("%s: terminal state %q is %s, not done or cancelled", tpl.Key, s.Key, s.Category)
			}
		}
		for _, f := range tpl.Fields {
			if err := f.Validate(); err != nil {
				t.Errorf("%s: field %q: %v", tpl.Key, f.Key, err)
			}
		}
		if tpl.States == "" || !strings.Contains(tpl.States, " → ") {
			t.Errorf("%s: states summary %q", tpl.Key, tpl.States)
		}
		if !reachesATerminal(tpl.Definition) {
			t.Errorf("%s: some state cannot reach a terminal state", tpl.Key)
		}
	}
	if !slices.Equal(keys, wantKeys) {
		t.Fatalf("templates = %v, want %v", keys, wantKeys)
	}
}

// reachesATerminal reports whether every state has a route to a terminal one,
// so no template can strand a task.
func reachesATerminal(d core.WorkflowDefinition) bool {
	for _, s := range d.States {
		if s.Terminal {
			continue
		}
		found := false
		for _, r := range core.Routes(d, s.Key) {
			if r.To.Terminal {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestTheRulesTheTemplatesPromise(t *testing.T) {
	t.Parallel()
	tests := []struct {
		template, from, to string
		comment            bool
		scope              core.Scope
	}{
		{"tix-dev-review", "in_review", "in_progress", true, ""},
		{"tix-bugs", "new", "wontfix", true, ""},
		{"tix-bugs", "triaged", "duplicate", true, ""},
		{"tix-releases", "deployed", "rolled_back", true, ""},
		{"tix-agent-review", "rejected", "queued", true, ""},
		{"tix-agent-review", "needs_review", "approved", false, core.ScopeTaskApprove},
		{"tix-approval", "under_review", "approved", false, core.ScopeTaskApprove},
		{"tix-approval", "under_review", "rejected", true, core.ScopeTaskApprove},
		{"tix-sales", "negotiation", "lost", true, ""},
		{"tix-hiring", "interview", "rejected", true, ""},
		{"tix-incident", "mitigated", "investigating", false, ""},
		{"tix-support", "resolved", "open", false, ""},
		{"tix-content", "editing", "drafting", false, ""},
	}
	for _, tc := range tests {
		tpl, ok, err := workflows.Lookup(tc.template)
		if err != nil || !ok {
			t.Fatalf("lookup %s: %v %v", tc.template, ok, err)
		}
		edge, ok := tpl.Definition.CanTransition(tc.from, tc.to)
		if !ok {
			t.Errorf("%s: no edge %s > %s", tc.template, tc.from, tc.to)
			continue
		}
		if edge.RequiresComment != tc.comment || edge.RequiresScope != tc.scope {
			t.Errorf("%s: %s > %s = %+v, want comment %v scope %q", tc.template, tc.from, tc.to, edge, tc.comment, tc.scope)
		}
	}
	for _, key := range []string{"tix-dev-review", "tix-agent-review"} {
		tpl, _, _ := workflows.Lookup(key)
		reverts := false
		for _, s := range tpl.Definition.States {
			reverts = reverts || (s.RevertOnLeaseExpiry && s.RevertTo != "")
		}
		if !reverts {
			t.Errorf("%s: no state reverts when its lease expires", key)
		}
	}
	if tpl, _, _ := workflows.Lookup("tix-kanban"); hasRules(tpl.Definition) {
		t.Error("the simple kanban template carries a rule")
	}
}

// hasRules reports whether any edge asks for a comment or a scope.
func hasRules(d core.WorkflowDefinition) bool {
	for _, e := range d.Transitions {
		if e.RequiresComment || e.RequiresScope != "" {
			return true
		}
	}
	return false
}

func TestParseRefusesAnIncompleteTemplate(t *testing.T) {
	t.Parallel()
	const def = "definition:\n  initial: a\n  states:\n    - {key: a, label: A}\n    - {key: b, label: B, terminal: true}\n" +
		"  transitions:\n    - {from: a, to: b}\n"
	tests := []struct {
		name, doc, want string
	}{
		{"ok", "key: tix-x\nname: X\npurpose: Do x.\n" + def, ""},
		{"unprefixed", "key: x\nname: X\npurpose: Do x.\n" + def, "does not start with"},
		{"no name", "key: tix-x\npurpose: Do x.\n" + def, "no name"},
		{"no purpose", "key: tix-x\nname: X\n" + def, "no purpose"},
		{"unknown key", "key: tix-x\nname: X\npurpose: Do x.\ncolour: red\n" + def, "colour"},
		{"bad field", "key: tix-x\nname: X\npurpose: Do x.\n" + def + "fields:\n  - {key: f, type: enum}\n", "option"},
		{"bad workflow", "key: tix-x\nname: X\npurpose: Do x.\ndefinition:\n  initial: z\n  states:\n    - {key: a, terminal: true}\n", "initial"},
	}
	for _, tc := range tests {
		_, err := workflows.Parse([]byte(tc.doc))
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}
