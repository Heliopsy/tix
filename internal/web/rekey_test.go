// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// settingsForm is the project settings form as the board renders it.
func settingsForm(name, key string) url.Values {
	return url.Values{"name": {name}, "key": {key}, "description": {""},
		"color": {""}, "icon": {""}, "workflow_key": {""}}
}

// settingsAt is the multipart project form posting to action.
func settingsAt(t *testing.T, page, action string) string {
	t.Helper()
	return formAt(t, page, action+`" enctype="multipart/form-data`)
}

// The settings form shows the key with its explanation beside it.
func TestProjectSettingsOfferTheKey(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	form := settingsAt(t, b.page("/projects/infra"), "/projects/infra/update")
	if got := inputValue(t, form, "key"); got != "infra" {
		t.Errorf("key field holds %q, want infra", got)
	}
	if !strings.Contains(form, "old links, references and filters keep working") {
		t.Errorf("the key field carries no explanation of what a change does:\n%s", form)
	}
}

// A changed key asks first, shows the references it changes, and saves only
// once confirmed, landing on the new board.
func TestChangingAProjectKeyIsConfirmedFirst(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "kept task")
	seq := strings.TrimPrefix(ref, "infra-")

	resp := b.post("/projects/infra/update", settingsForm("Renamed", "ops"))
	wantStatus(t, resp, http.StatusOK)
	page := readAll(t, resp)
	_ = resp.Body.Close()
	if !strings.Contains(page, "<code>"+ref+"</code> &rarr; <code>ops-"+seq+"</code>") {
		t.Errorf("the confirmation does not show %s becoming ops-%s:\n%s", ref, seq, page)
	}
	confirm := settingsAt(t, page, "/projects/infra/update")
	for _, want := range []string{`name="confirm_key" value="1"`, `name="key" value="ops"`, `name="name" value="Renamed"`} {
		if !strings.Contains(confirm, want) {
			t.Errorf("the confirmation form lacks %s:\n%s", want, confirm)
		}
	}
	if board := b.page("/projects/infra"); !strings.Contains(board, `value="infra"`) {
		t.Error("asking for a confirmation already changed the key")
	}

	form := settingsForm("Renamed", "ops")
	form.Set("confirm_key", "1")
	resp = b.post("/projects/infra/update", form)
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/projects/ops?") {
		t.Errorf("confirmed rename went to %q, want /projects/ops", loc)
	}
	if got := inputValue(t, settingsAt(t, b.page("/projects/ops"), "/projects/ops/update"), "name"); got != "Renamed" {
		t.Errorf("confirming did not save the other fields: name %q", got)
	}
}

// Submitting the key unchanged saves without a question.
func TestAnUnchangedKeySavesAtOnce(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/projects/infra/update", settingsForm("Same key", "INFRA"))
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
}

// Old project and task addresses redirect permanently to the new ones, the
// rest of the path and the query kept.
func TestOldProjectAndTaskURLsRedirect(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "linked task")
	seq := strings.TrimPrefix(ref, "infra-")
	form := settingsForm("Infra", "ops")
	form.Set("confirm_key", "1")
	_ = b.post("/projects/infra/update", form).Body.Close()

	for path, want := range map[string]string{
		"/projects/infra":         "/projects/ops",
		"/projects/infra/fields":  "/projects/ops/fields",
		"/projects/infra/live":    "/projects/ops/live",
		"/tasks/" + ref:           "/tasks/ops-" + seq,
		"/tasks/" + ref + "?x=1":  "/tasks/ops-" + seq + "?x=1",
		"/tasks/" + ref + "/live": "/tasks/ops-" + seq + "/live",
	} {
		resp := b.get(path)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != want {
			t.Errorf("GET %s = %d to %q, want 301 to %s", path, resp.StatusCode, resp.Header.Get("Location"), want)
		}
	}
	if resp := b.get("/projects/ops"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /projects/ops = %d, want 200", resp.StatusCode)
	}
	resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"posted to the old address"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
}

// A key change confirmed from a form that also chose an icon saves the icon
// too, and an uploaded image the confirmation cannot carry is asked for again.
func TestAKeyChangeKeepsTheIconChoice(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	fields := settingsFields("builtin")
	fields["icon_name"], fields["key"] = "rocket", "ops"
	resp := b.postIcon("/projects/infra/update", fields, nil)
	wantStatus(t, resp, http.StatusOK)
	confirm := settingsAt(t, readAll(t, resp), "/projects/infra/update")
	_ = resp.Body.Close()
	for _, want := range []string{`name="icon_kind" value="builtin"`, `name="icon_name" value="rocket"`} {
		if !strings.Contains(confirm, want) {
			t.Errorf("the confirmation does not carry %s:\n%s", want, confirm)
		}
	}
	if strings.Contains(confirm, `name="icon_file"`) {
		t.Error("the confirmation asks for an image nobody uploaded")
	}

	fields = settingsFields("builtin")
	fields["icon_name"], fields["key"], fields["confirm_key"] = "rocket", "ops", "1"
	resp = b.postIcon("/projects/infra/update", fields, nil)
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if h := boardHeading(t, b.page("/projects/ops")); !strings.Contains(h, "#pi-rocket") {
		t.Errorf("confirming the key change lost the icon: %s", h)
	}

	fields = settingsFields("image")
	fields["key"] = "platform"
	resp = b.postIcon("/projects/ops/update", fields, webPNG)
	wantStatus(t, resp, http.StatusOK)
	page := readAll(t, resp)
	_ = resp.Body.Close()
	if !strings.Contains(settingsAt(t, page, "/projects/ops/update"), `name="icon_file" type="file"`) {
		t.Errorf("an uploaded image is not asked for again on the confirmation:\n%s", page)
	}
}

// A custom colour chosen in the same save reaches the confirmation as the
// colour itself, so confirming keeps it.
func TestAKeyChangeKeepsACustomColour(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	form := settingsForm("Infra", "ops")
	form.Set("color", "#custom")
	form.Set("color_custom", "1a7f5a")
	resp := b.post("/projects/infra/update", form)
	wantStatus(t, resp, http.StatusOK)
	confirm := settingsAt(t, readAll(t, resp), "/projects/infra/update")
	_ = resp.Body.Close()
	if !strings.Contains(confirm, `name="color" value="#1a7f5a"`) {
		t.Errorf("the confirmation does not carry the custom colour:\n%s", confirm)
	}
}
