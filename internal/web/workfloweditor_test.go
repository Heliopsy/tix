// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// textareaValue reads back exactly what the editor put in one textarea, so a
// test can resubmit the form the way a reader who changed nothing would.
func textareaValue(t *testing.T, page, id string) string {
	t.Helper()
	open := `<textarea id="` + id + `"`
	i := strings.Index(page, open)
	if i < 0 {
		t.Fatalf("the editor has no %q textarea:\n%s", id, page)
	}
	rest := page[i:]
	start := strings.Index(rest, ">")
	end := strings.Index(rest, "</textarea>")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("the %q textarea is not closed:\n%s", id, page)
	}
	return html.UnescapeString(rest[start+1 : end])
}

// Saving the workflow editor without changing anything has to leave the
// workflow as it was.
//
// The editor renders a state as "key|label|terminal" and a transition as
// "from>to", which is not everything either one carries: a state also holds
// its category and its lease-expiry revert, and a transition holds the scope
// it requires. PutWorkflow replaces the stored definition with whatever the
// form parsed, so anything the editor does not render is not edited away by
// the reader -- it is erased by the round trip.
func TestSavingTheWorkflowEditorUnchangedKeepsTheWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	before, err := f.svc.GetWorkflow(f.ctx(), "default")
	if err != nil {
		t.Fatalf("reading the shipped workflow: %v", err)
	}
	// The shipped workflow has to actually carry the fields under test, or
	// this guard would pass on a workflow that never had them.
	var hadCategory, hadRevert bool
	for _, s := range before.Definition.States {
		if s.Category != "" {
			hadCategory = true
		}
		if s.RevertOnLeaseExpiry {
			hadRevert = true
		}
	}
	if !hadCategory || !hadRevert {
		t.Fatalf("the shipped workflow carries no category (%v) or no lease revert (%v) to lose",
			hadCategory, hadRevert)
	}

	page := b.page("/workflows/default")
	saved := b.post("/workflows", url.Values{
		"key":         {before.Key},
		"name":        {before.Name},
		"initial":     {before.Definition.Initial},
		"states":      {textareaValue(t, page, "states")},
		"transitions": {textareaValue(t, page, "transitions")},
		"migrate":     {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "default")
	if err != nil {
		t.Fatalf("re-reading the workflow: %v", err)
	}

	byKey := map[string]core.State{}
	for _, s := range after.Definition.States {
		byKey[s.Key] = s
	}
	for _, want := range before.Definition.States {
		got, ok := byKey[want.Key]
		if !ok {
			t.Errorf("state %q is gone after a save that changed nothing", want.Key)
			continue
		}
		if got.Category != want.Category {
			t.Errorf("state %q lost its category: %q, want %q", want.Key, got.Category, want.Category)
		}
		if got.RevertOnLeaseExpiry != want.RevertOnLeaseExpiry || got.RevertTo != want.RevertTo {
			t.Errorf("state %q lost its lease-expiry revert: %v/%q, want %v/%q",
				want.Key, got.RevertOnLeaseExpiry, got.RevertTo, want.RevertOnLeaseExpiry, want.RevertTo)
		}
	}
	if after.Definition.DefaultLease != before.Definition.DefaultLease {
		t.Errorf("the workflow lost its default lease: %v, want %v",
			after.Definition.DefaultLease, before.Definition.DefaultLease)
	}
}

// The same round trip, for the one field on a transition that is an
// authorization gate rather than a display detail. A scope the editor never
// rendered comes back cleared, so a move an operator restricted becomes a
// move anybody holding the plain transition scope may make.
func TestSavingTheWorkflowEditorUnchangedKeepsATransitionScope(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	gated := core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "shipped", Label: "Shipped", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "shipped", RequiresScope: core.ScopeAll, RequiresComment: true},
		},
	}
	if _, err := f.svc.PutWorkflow(f.ctx(), core.WorkflowInput{
		Key: "gated", Name: "Gated", Definition: gated,
	}); err != nil {
		t.Fatalf("defining the gated workflow: %v", err)
	}

	page := b.page("/workflows/gated")
	saved := b.post("/workflows", url.Values{
		"key":         {"gated"},
		"name":        {"Gated"},
		"initial":     {"todo"},
		"states":      {textareaValue(t, page, "states")},
		"transitions": {textareaValue(t, page, "transitions")},
		"migrate":     {""},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after, err := f.svc.GetWorkflow(f.ctx(), "gated")
	if err != nil {
		t.Fatalf("re-reading the gated workflow: %v", err)
	}
	if len(after.Definition.Transitions) != 1 {
		t.Fatalf("the save left %d transitions, want 1", len(after.Definition.Transitions))
	}
	got := after.Definition.Transitions[0]
	if got.RequiresScope != core.ScopeAll {
		t.Errorf("the transition lost the scope it required: %q, want %q", got.RequiresScope, core.ScopeAll)
	}
	if !got.RequiresComment {
		t.Error("the transition no longer requires a comment")
	}
}
