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

func TestRetentionRejectsAMalformedWindow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("alice").post("/admin/tenant/retention", url.Values{"events": {"forever"}})
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(page, "duration") {
		t.Fatalf("the refusal does not explain itself:\n%s", page)
	}
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
	if page := b.page("/tasks/" + ref); !strings.Contains(page, `<span class="tag">ops</span>`) {
		t.Fatal("the tag was not added to the task")
	}

	removed := b.post("/tasks/"+ref+"/tags/remove", url.Values{"tag": {"ops"}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)
	if page := b.page("/tasks/" + ref); strings.Contains(page, `<span class="tag">ops</span>`) {
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
