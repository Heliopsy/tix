// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestMembershipAdministration(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	added := b.post("/admin/tenant/members", url.Values{
		"actor_id": {f.actorA.ID}, "role": {"member"}})
	_ = added.Body.Close()
	wantStatus(t, added, http.StatusSeeOther)

	page := b.page("/admin/tenant")
	if !strings.Contains(page, f.actorA.ID) {
		t.Fatalf("the member is not listed")
	}

	removed := b.post("/admin/tenant/members/remove", url.Values{"actor_id": {f.actorA.ID}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)
	if strings.Contains(b.page("/admin/tenant"), f.actorA.ID) {
		t.Fatalf("the removed member is still listed")
	}
}

func TestUserAdministrationUpdatesAndDeletes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/users", url.Values{"email": {"temp@example.test"},
		"password": {"correct-horse-battery"}, "role": {"viewer"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/users")
	id := between(t, page, `<input type="hidden" name="id" value="`, `"`)
	if id == "" {
		t.Fatalf("no user identifier was rendered")
	}

	updated := b.post("/admin/users/update", url.Values{"id": {id},
		"display_name": {"Temporary"}, "role": {"member"}, "disabled": {"1"}})
	_ = updated.Body.Close()
	wantStatus(t, updated, http.StatusSeeOther)

	afterUpdate := b.page("/admin/users")
	for _, want := range []string{"Temporary", "disabled"} {
		if !strings.Contains(afterUpdate, want) {
			t.Fatalf("the user list does not show %q", want)
		}
	}

	deleted := b.post("/admin/users/delete", url.Values{"id": {id}})
	_ = deleted.Body.Close()
	wantStatus(t, deleted, http.StatusSeeOther)
	if strings.Contains(b.page("/admin/users"), "temp@example.test") {
		t.Fatalf("the deleted user is still listed")
	}
}

func TestWebhookDeleteAndRedelivery(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/admin/webhooks", url.Values{"url": {"https://hooks.example/one"},
		"event_types": {"task.created"}, "active": {"1"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/admin/webhooks")
	id := between(t, page, `<input type="hidden" name="id" value="`, `"`)

	missing := b.post("/admin/webhooks/deliveries/redeliver", url.Values{"id": {"absent"}})
	defer func() { _ = missing.Body.Close() }()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unknown delivery", missing.StatusCode)
	}

	deleted := b.post("/admin/webhooks/delete", url.Values{"id": {id}})
	_ = deleted.Body.Close()
	wantStatus(t, deleted, http.StatusSeeOther)
	if strings.Contains(b.page("/admin/webhooks"), "hooks.example/one") {
		t.Fatalf("the deleted endpoint is still listed")
	}
}

// The field takes the wider grammar now, so the cases here have to be values
// that grammar still refuses: "720h" and "30d" are both accepted, and a test
// written against either would pass whatever the handler did.
func TestRetentionRejectsAMalformedWindow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, raw := range []string{"forever", "4w", "-30d", "30 days"} {
		t.Run(raw, func(t *testing.T) {
			resp := f.as("alice").post("/admin/tenant/retention", url.Values{"events": {raw}})
			page := body(t, resp)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("saving %q = %d, want 400", raw, resp.StatusCode)
			}
			if !strings.Contains(page, "duration") {
				t.Fatalf("the refusal of %q does not explain itself:\n%s", raw, page)
			}
		})
	}
}

// A window shown one way and read another is a form that refuses its own
// value. The screen printed "720h0m0s" and the handler parsed with Go's own
// syntax, so the day the rest of the product speaks in was unsayable here.
func TestRetentionSavesTheVocabularyItDisplays(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	want := map[string]string{"events": "30d", "audit_entries": "365d", "webhook_deliveries": "12h"}

	first := url.Values{}
	for name, value := range want {
		first.Set(name, value)
	}
	saved := b.post("/admin/tenant/retention", first)
	if text := body(t, saved); saved.StatusCode != http.StatusSeeOther {
		t.Fatalf("saving the vocabulary the product prints = %d, want 303: %s", saved.StatusCode, text)
	}

	page := b.page("/admin/tenant")
	again := url.Values{}
	for name, value := range want {
		shown := inputValue(t, page, name)
		if shown != value {
			t.Errorf("the %s field shows %q, want %q", name, shown, value)
		}
		again.Set(name, shown)
	}

	resaved := b.post("/admin/tenant/retention", again)
	if text := body(t, resaved); resaved.StatusCode != http.StatusSeeOther {
		t.Fatalf("saving what the screen rendered = %d, want 303: %s", resaved.StatusCode, text)
	}
}

// The tree carries five live figures, so a row with none reads as a count that
// broke rather than one nobody makes. Each row is read on its own: the page
// carries other rows' numbers, and an assertion over the whole page would be
// satisfied by any of them.
func TestEveryShapeRowCarriesAFigureOrSaysWhyItHasNone(t *testing.T) {
	t.Parallel()
	page := newFixture(t).as("alice").page("/admin/tenant")

	for _, name := range []string{"Members", "Domains", "API tokens", "Workflows", "Projects"} {
		row := shapeRow(t, page, name)
		if !strings.Contains(row, `<span class="count">`) {
			t.Errorf("the %s row carries no figure:\n%s", name, row)
		}
	}
	for _, name := range []string{"Tasks", "Field definitions"} {
		row := shapeRow(t, page, name)
		if !strings.Contains(row, `<span class="count uncounted">`) {
			t.Errorf("the %s row leaves the figure blank instead of saying why it has none:\n%s", name, row)
		}
	}
}

// The "i" is a disclosure whose text is behind it, so an "i" with nothing
// beside it reads as an icon whose label failed to render. Every other one on
// this screen sits in a field-label row with the control it explains, and the
// assertion reads the prune form alone: the retention form above it has three
// correctly placed ones that would satisfy a page-wide check.
func TestThePruneNoticeSitsBesideTheControlItExplains(t *testing.T) {
	t.Parallel()
	page := newFixture(t).as("alice").page("/admin/tenant")
	form := formAt(t, page, "/admin/tenant/prune")

	if !strings.Contains(form, `class="field-info"`) {
		t.Fatalf("the prune form carries no notice at all:\n%s", form)
	}
	label := between(t, form, `<div class="field-label">`, "</div>")
	if !strings.Contains(label, "Dry run") || !strings.Contains(label, `class="field-info"`) {
		t.Errorf("the prune notice is not on the line of the control it explains:\n%s", form)
	}
}

// inputValue reads one named input's value attribute. A retention window is
// three characters long and occurs in the notice text beside the field, so a
// test about what the field holds has to read the field.
func inputValue(t *testing.T, page, id string) string {
	t.Helper()
	tag := between(t, page, `<input id="`+id+`"`, ">")
	if tag == "" {
		t.Fatalf("the page has no input named %q", id)
	}
	return between(t, tag+">", `value="`, `"`)
}

// shapeRow returns the one list item of the tenant tree that names a kind.
func shapeRow(t *testing.T, page, name string) string {
	t.Helper()
	tree := between(t, page, `<ul class="shape">`, "</ul>")
	if tree == "" {
		t.Fatalf("the page renders no tenant tree")
	}
	for _, row := range strings.Split(tree, "<li ") {
		if strings.Contains(row, ">"+name+"<") {
			return row
		}
	}
	t.Fatalf("the tenant tree has no %s row:\n%s", name, tree)
	return ""
}

// formAt returns the body of the form posting to one action, so an assertion
// about one form on a screen of forms reads that form.
func formAt(t *testing.T, page, action string) string {
	t.Helper()
	form := between(t, page, `action="`+action+`">`, "</form>")
	if form == "" {
		t.Fatalf("the page has no form posting to %s", action)
	}
	return form
}

func TestCommentEditingAndRemoval(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "commented on")

	added := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"first draft"}})
	_ = added.Body.Close()
	wantStatus(t, added, http.StatusSeeOther)

	page := b.page("/tasks/" + ref)
	id := between(t, page, `<input type="hidden" name="id" value="`, `"`)
	if id == "" {
		t.Fatalf("no comment identifier was rendered")
	}

	edited := b.post("/tasks/"+ref+"/comments/edit", url.Values{"id": {id},
		"body": {"second draft"}})
	_ = edited.Body.Close()
	wantStatus(t, edited, http.StatusSeeOther)
	if !strings.Contains(b.page("/tasks/"+ref), "second draft") {
		t.Fatalf("the edited comment is not shown")
	}

	removed := b.post("/tasks/"+ref+"/comments/delete", url.Values{"id": {id}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)
	if strings.Contains(b.page("/tasks/"+ref), "second draft") {
		t.Fatalf("the deleted comment is still shown")
	}
}

func TestTagRemoval(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "tagged")

	added := b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}})
	_ = added.Body.Close()
	wantStatus(t, added, http.StatusSeeOther)
	if page := b.page("/tasks/" + ref); !strings.Contains(page, `>#</span>ops</a>`) {
		t.Fatal("the tag was not added to the task")
	}

	removed := b.post("/tasks/"+ref+"/tags/remove", url.Values{"tag": {"ops"}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)
	if page := b.page("/tasks/" + ref); strings.Contains(page, `>#</span>ops</a>`) {
		t.Error("the tag is still on the task after its removal was accepted")
	}
}

func TestMalformedReferencesAndNumbersAreRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "well formed")

	cases := []struct {
		name string
		path string
		form url.Values
	}{
		{"task ref", "/tasks/not a ref/transition", url.Values{"to": {"doing"}}},
		{"board ref", "/projects/infra/move", url.Values{"ref": {"!!"}, "to": {"doing"}}},
		{"dependency ref", "/tasks/" + ref + "/deps", url.Values{"depends_on": {"!!"}}},
		{"version", "/tasks/" + ref, url.Values{"title": {"x"}, "version": {"many"}}},
		{"priority", "/tasks/" + ref, url.Values{"title": {"x"}, "priority": {"urgent"}}},
		{"field position", "/projects/infra/fields", url.Values{"key": {"k"},
			"type": {"string"}, "position": {"first"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := b.post(tc.path, tc.form)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestCustomFieldValuesRoundTrip(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects/infra/fields", url.Values{"key": {"points"},
		"label": {"Points"}, "type": {"int"}, "position": {"1"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	ref := b.createTask("infra", "estimated")
	saved := b.post("/tasks/"+ref, url.Values{"title": {"estimated"},
		"body": {""}, "field.points": {"8"}})
	wantStatus(t, saved, http.StatusSeeOther)
	_ = saved.Body.Close()

	page := b.page("/tasks/" + ref)
	if !strings.Contains(page, "8") {
		t.Fatalf("the custom field value is not shown:\n%s", page)
	}
}

// TestTheTenantKeyIsALabelledReadOnlyField pins the repair of a string that
// read as debug output.
//
// The tenant screen rendered "Key default" as a bare paragraph between the
// theme's help text and the Save button: no label, no field frame, and
// nothing saying what a tenant key is or what anybody would do with one. Two
// independent capture passes reported it as something left behind.
//
// It is the key the tenant is addressed by from outside the browser, it is
// fixed at creation, and this form's handler takes only a display name and a
// theme. So it is a field a reader can read and copy, with the same label and
// notice idiom every other field on the screen carries, and it submits
// nothing: a readonly input still posts its value, so it carries no name.
func TestTheTenantKeyIsALabelledReadOnlyField(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/admin/tenant")

	if strings.Contains(page, `<p class="meta">Key `) {
		t.Error("the tenant key is still a bare paragraph of body text")
	}
	form := formAt(t, page, "/admin/tenant")
	label := between(t, form, `<label for="key">`, "</label>")
	if label != "Tenant key" {
		t.Errorf("the tenant key carries no label of its own: %q", label)
	}
	notice := between(t, form, `<label for="key">Tenant key</label>`, "</details>")
	if !strings.Contains(notice, `class="field-info"`) {
		t.Errorf("the tenant key has no notice saying what a tenant key is:\n%s", notice)
	}
	for _, want := range []string{"--tenant", "TIX_TENANT", "cannot be changed"} {
		if !strings.Contains(notice, want) {
			t.Errorf("the tenant key's notice does not mention %q:\n%s", want, notice)
		}
	}
	field := between(t, form, `<input id="key"`, ">")
	if !strings.Contains(field, "readonly") {
		t.Errorf("the tenant key is offered as editable, and no handler saves it: %s", field)
	}
	if strings.Contains(field, "name=") {
		t.Errorf("the tenant key would be submitted with the form: %s", field)
	}
	if got := inputValue(t, form, "key"); got != "acme" {
		t.Errorf("the tenant key field holds %q, not the tenant's key", got)
	}

	// And it has to look unlike the fields around it, or it reads as one more
	// box to type into whose Save silently drops what was typed.
	sheet := body(t, f.as("alice").get("/assets/app.css"))
	painted := declarations(t, sheet, "input[readonly]")
	for _, want := range []string{"background:", "border-style: dashed"} {
		if !strings.Contains(painted, want) {
			t.Errorf("a read-only field is painted like an editable one, missing %q: %s", want, painted)
		}
	}
}
