// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// Blank clears, but absent does not. The two forms on the task screen post to
// the same address: the top one carries no field.* input at all, the custom
// field editor carries one per definition. If an absent key were read as a
// blank one, saving a title would wipe every custom field on the task, so the
// rule has to be the key's presence rather than the value it holds.
//
// Two definitions, one submitted. A save naming no field at all is turned
// away earlier, by the short circuit for a form that mentions no field, so a
// guard built on that shape would pass with the presence rule gone; this one
// submits a field, which is the only way to reach the rule itself.
func TestSavingOneCustomFieldLeavesTheOthersAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for i, key := range []string{"owner", "team"} {
		created := b.post("/projects/infra/fields", url.Values{"key": {key},
			"label": {key}, "type": {"text"}, "position": {fmt.Sprint(i + 1)}})
		_ = created.Body.Close()
		wantStatus(t, created, http.StatusSeeOther)
	}

	ref := b.createTask("infra", "keeps its owner")
	saved := b.post("/tasks/"+ref, url.Values{"title": {"keeps its owner"},
		"body": {""}, "field.owner": {"alice"}, "field.team": {"platform"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	partial := b.post("/tasks/"+ref, url.Values{"title": {"keeps its owner"},
		"body": {""}, "field.team": {"infra"}})
	_ = partial.Body.Close()
	wantStatus(t, partial, http.StatusSeeOther)

	page := b.page("/tasks/" + ref)
	if got := inputValue(t, page, "field-team"); got != "infra" {
		t.Fatalf("the field that was submitted holds %q, want \"infra\"", got)
	}
	if got := inputValue(t, page, "field-owner"); got != "alice" {
		t.Errorf("a save naming only one field left the other as %q, want \"alice\"", got)
	}
}

// And the shape the top form actually posts: a save carrying no field.* key
// at all leaves every custom field where it was.
func TestSavingTheTaskWithoutTheFieldInputsLeavesThemAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects/infra/fields", url.Values{"key": {"owner"},
		"label": {"Owner"}, "type": {"text"}, "position": {"1"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	ref := b.createTask("infra", "keeps its owner")
	saved := b.post("/tasks/"+ref, url.Values{"title": {"keeps its owner"},
		"body": {""}, "field.owner": {"alice"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	renamed := b.post("/tasks/"+ref, url.Values{"title": {"renamed"}, "body": {"a note"}})
	_ = renamed.Body.Close()
	wantStatus(t, renamed, http.StatusSeeOther)

	if got := inputValue(t, b.page("/tasks/"+ref), "field-owner"); got != "alice" {
		t.Errorf("saving the title without the field inputs left the owner as %q, want \"alice\"", got)
	}
}

// A required field cannot be emptied, and the refusal has to be visible: a
// save that reports success and silently keeps the old value would be the
// same defect wearing a different coat, and a save that reports success and
// stores an empty required field would be worse.
//
// The control save matters as much as the refusal. Without it a screen that
// refused every save to this task would pass this guard.
func TestBlankingARequiredCustomFieldIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects/infra/fields", url.Values{"key": {"owner"},
		"label": {"Owner"}, "type": {"text"}, "position": {"1"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	ref := b.createTask("infra", "needs an owner")
	saved := b.post("/tasks/"+ref, url.Values{"title": {"needs an owner"},
		"body": {""}, "field.owner": {"alice"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	required := b.post("/projects/infra/fields", url.Values{"key": {"owner"},
		"label": {"Owner"}, "type": {"text"}, "position": {"1"}, "required": {"1"}})
	_ = required.Body.Close()
	wantStatus(t, required, http.StatusSeeOther)

	again := b.post("/tasks/"+ref, url.Values{"title": {"needs an owner"},
		"body": {""}, "field.owner": {"bob"}})
	_ = again.Body.Close()
	wantStatus(t, again, http.StatusSeeOther)
	if got := inputValue(t, b.page("/tasks/"+ref), "field-owner"); got != "bob" {
		t.Fatalf("a save that fills the required field was not applied: %q", got)
	}

	blanked := b.post("/tasks/"+ref, url.Values{"title": {"needs an owner"},
		"body": {""}, "field.owner": {""}})
	defer func() { _ = blanked.Body.Close() }()
	if blanked.StatusCode != http.StatusBadRequest {
		t.Errorf("blanking a required field answered %d, want 400", blanked.StatusCode)
	}
	if got := inputValue(t, b.page("/tasks/"+ref), "field-owner"); got != "bob" {
		t.Errorf("the refused save changed the stored value to %q", got)
	}
}

// The same presence rule on the account form. A caller naming no display_name
// is asking for no change; only one that names it, empty, removes the name.
func TestUpdatingAnAccountWithoutTheNameInputLeavesTheNameAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"kept@example.test"},
		"display_name": {"Kept Name"}, "password": {"correct-horse-battery"}, "role": {"member"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	id := userIDFor(t, b.page("/admin/users"), "kept@example.test")
	saved := b.post("/admin/users/update", url.Values{"id": {id}, "role": {"member"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after := b.page("/admin/users")
	if got := inputValue(t, personRow(t, after, id), "name-"+id); got != "Kept Name" {
		t.Errorf("a save that named no display name left %q, want \"Kept Name\"", got)
	}
}
