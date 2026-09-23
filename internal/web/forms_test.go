// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestFormWorksWithoutJavaScript(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/tasks")
	if !hasPlainForm(page, "/tasks") {
		t.Fatalf("the task list offers no plain form")
	}

	resp := b.postRaw("/tasks", url.Values{
		"csrf_token":  {b.csrf()},
		"project_ref": {"infra"},
		"title":       {"created without scripts"},
		"body":        {"submitted by a plain form post"},
		"priority":    {"2"},
		"tags":        {"ops, urgent"},
	})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	listed := b.page("/tasks")
	if !strings.Contains(listed, "created without scripts") {
		t.Fatalf("the task a plain form created is not listed")
	}
	detail := b.page(resp.Header.Get("Location"))
	for _, want := range []string{"submitted by a plain form post", "ops", "urgent"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("the created task lost %q", want)
		}
	}
}

func TestBoardMoveTransitionsATask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "moves across the board")

	before := b.page("/projects/infra")
	if !strings.Contains(before, ref) {
		t.Fatalf("the board does not show the new task")
	}

	resp := b.post("/projects/infra/move", url.Values{"ref": {ref}, "to": {"doing"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	detail := b.page("/tasks/" + ref)
	if !strings.Contains(detail, `<span class="pill status doing">doing</span>`) {
		t.Fatalf("the task did not transition:\n%s", detail)
	}
}

func TestIllegalBoardMoveIsRefusedWithAnExplanation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "stays put")

	resp := b.post("/projects/infra/move", url.Values{"ref": {ref}, "to": {"done"}})
	page := body(t, resp)
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatalf("an illegal move was accepted")
	}
	if !strings.Contains(strings.ToLower(page), "todo") {
		t.Fatalf("the refusal does not explain itself:\n%s", page)
	}

	detail := b.page("/tasks/" + ref)
	if !strings.Contains(detail, `<span class="pill status todo">todo</span>`) {
		t.Fatalf("the refused move changed the task")
	}
}

func TestBoardColumnsFollowTheWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/projects/infra")
	for _, state := range []string{"todo", "doing", "blocked", "done", "cancelled"} {
		if !strings.Contains(page, `data-state="`+state+`"`) {
			t.Fatalf("the board has no column for %q", state)
		}
	}
}

func TestTaskDetailEditsApply(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "edit me")

	steps := []struct {
		name string
		path string
		form url.Values
		want string
	}{
		{"update", "/tasks/" + ref, url.Values{"title": {"edited title"},
			"body": {"edited body"}, "priority": {"1"}}, "edited title"},
		{"comment", "/tasks/" + ref + "/comments", url.Values{"body": {"a remark"}}, "a remark"},
		{"tag", "/tasks/" + ref + "/tags", url.Values{"tag": {"ops"}}, "ops"},
		{"artifact", "/tasks/" + ref + "/artifacts", url.Values{"kind": {"result"},
			"name": {"summary"}, "payload": {"lines=12"}}, "summary"},
		{"transition", "/tasks/" + ref + "/transition", url.Values{"to": {"doing"}}, `<span class="pill status doing">doing</span>`},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			resp := b.post(step.path, step.form)
			defer func() { _ = resp.Body.Close() }()
			wantStatus(t, resp, http.StatusSeeOther)
			page := b.page("/tasks/" + ref)
			if !strings.Contains(page, step.want) {
				t.Fatalf("the detail screen does not show %q", step.want)
			}
		})
	}
}

func TestDependenciesAndSubtasksAreShown(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "depends on the other")
	second := b.createTask("infra", "the other")

	resp := b.post("/tasks/"+first+"/deps", url.Values{"depends_on": {second}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/tasks/" + first)
	if !strings.Contains(page, "Dependencies") {
		t.Fatalf("the detail screen has no dependency section")
	}

	removed := b.post("/tasks/"+first+"/deps/remove", url.Values{"depends_on": {second}})
	defer func() { _ = removed.Body.Close() }()
	wantStatus(t, removed, http.StatusSeeOther)
}

func TestTaskHistoryListsRecordedChanges(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "audited")

	resp := b.post("/tasks/"+ref+"/transition", url.Values{"to": {"doing"}})
	_ = resp.Body.Close()

	page := b.page("/tasks/" + ref)
	if !strings.Contains(page, "task.transition") {
		t.Fatalf("history does not list the transition:\n%s", page)
	}
	if !strings.Contains(page, ">web<") {
		t.Fatalf("history does not record the web source")
	}
}

func TestTaskDeleteAndRestore(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "temporary")

	deleted := b.post("/tasks/"+ref+"/delete", url.Values{})
	_ = deleted.Body.Close()
	wantStatus(t, deleted, http.StatusSeeOther)

	if strings.Contains(b.page("/tasks"), "temporary") {
		t.Fatalf("a deleted task is still listed")
	}

	restored := b.post("/tasks/"+ref+"/restore", url.Values{})
	_ = restored.Body.Close()
	wantStatus(t, restored, http.StatusSeeOther)
	if !strings.Contains(b.page("/tasks"), "temporary") {
		t.Fatalf("a restored task is not listed")
	}
}

func TestProjectLifecycleFromTheBrowser(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects", url.Values{"key": {"apps"}, "name": {"Applications"},
		"workflow_key": {"default"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	updated := b.post("/projects/apps/update", url.Values{"name": {"Renamed"},
		"description": {"a description"}})
	_ = updated.Body.Close()
	wantStatus(t, updated, http.StatusSeeOther)
	if !strings.Contains(b.page("/projects/apps"), "Renamed") {
		t.Fatalf("the project rename did not apply")
	}

	archived := b.post("/projects/apps/archive", url.Values{})
	_ = archived.Body.Close()
	wantStatus(t, archived, http.StatusSeeOther)
	if !strings.Contains(b.page("/projects"), "archived") {
		t.Fatalf("the archived project is not shown as archived")
	}

	deleted := b.post("/projects/apps/delete", url.Values{})
	_ = deleted.Body.Close()
	wantStatus(t, deleted, http.StatusSeeOther)
}

func TestFieldDefinitionEditor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects/infra/fields", url.Values{"key": {"severity"},
		"label": {"Severity"}, "type": {"enum"}, "enum_options": {"low, high"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/projects/infra/fields")
	if !strings.Contains(page, "Severity") {
		t.Fatalf("the new field is not listed")
	}

	ref := b.createTask("infra", "carries a custom field")
	detail := b.page("/tasks/" + ref)
	if !strings.Contains(detail, "Severity") {
		t.Fatalf("the task detail screen omits the custom field")
	}

	invalid := b.post("/projects/infra/fields", url.Values{"key": {"broken"}, "type": {"enum"}})
	defer func() { _ = invalid.Body.Close() }()
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an enum with no options", invalid.StatusCode)
	}

	removed := b.post("/projects/infra/fields/delete", url.Values{"key": {"severity"}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)
}

func TestWorkflowEditor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	saved := b.post("/workflows", url.Values{
		"key": {"review"}, "name": {"Review"}, "initial": {"open"},
		"states":      {"open|Open|open\nchecking|Checking|open\nclosed|Closed|terminal"},
		"transitions": {"open>checking\nchecking>closed\nopen>closed"},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	page := b.page("/workflows/review")
	for _, want := range []string{"open&gt;checking", "closed|Closed|terminal"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the workflow editor does not round-trip %q", want)
		}
	}

	project := b.post("/projects", url.Values{"key": {"rev"}, "name": {"Reviews"},
		"workflow_key": {"review"}})
	_ = project.Body.Close()
	wantStatus(t, project, http.StatusSeeOther)

	board := b.page("/projects/rev")
	if !strings.Contains(board, `data-state="checking"`) {
		t.Fatalf("the board does not show the new workflow column")
	}

	invalid := b.post("/workflows", url.Values{"key": {"broken"}, "name": {"Broken"},
		"initial": {"missing"}, "states": {"open|Open|terminal"}})
	defer func() { _ = invalid.Body.Close() }()
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an undefined initial state", invalid.StatusCode)
	}

	deleted := b.post("/workflows/review/delete", url.Values{})
	defer func() { _ = deleted.Body.Close() }()
	if deleted.StatusCode == http.StatusSeeOther {
		t.Fatalf("a workflow still in use was deleted")
	}
}

func TestAdministrationScreensAcceptSubmissions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	steps := []struct {
		name   string
		path   string
		form   url.Values
		verify string
		want   string
	}{
		{"tenant", "/admin/tenant", url.Values{"name": {"Acme Renamed"}},
			"/admin/tenant", "Acme Renamed"},
		{"retention", "/admin/tenant/retention", url.Values{"events": {"48h"},
			"audit_entries": {"240h"}, "webhook_deliveries": {"24h"}}, "/admin/tenant", "48h"},
		{"prune", "/admin/tenant/prune", url.Values{"dry_run": {"1"}}, "/admin/tenant", "Tenant"},
		{"domain", "/admin/domains", url.Values{"hostname": {"acme.example"},
			"cert_mode": {"none"}}, "/admin/domains", "acme.example"},
		{"user", "/admin/users", url.Values{"email": {"new@example.test"},
			"password": {"correct-horse-battery"}, "role": {"member"}},
			"/admin/users", "new@example.test"},
		{"webhook", "/admin/webhooks", url.Values{"url": {"https://hooks.example/x"},
			"event_types": {"task.created"}, "active": {"1"}}, "/admin/webhooks", "hooks.example"},
		{"sync source", "/sync/sources", url.Values{"name": {"jira import"},
			"system": {"jira"}, "config": {"url=https://jira.example"}}, "/sync", "jira import"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			resp := b.post(step.path, step.form)
			defer func() { _ = resp.Body.Close() }()
			wantStatus(t, resp, http.StatusSeeOther)
			if page := b.page(step.verify); !strings.Contains(page, step.want) {
				t.Fatalf("%s does not show %q", step.verify, step.want)
			}
		})
	}

	removed := b.post("/admin/domains/remove", url.Values{"hostname": {"acme.example"}})
	_ = removed.Body.Close()
	wantStatus(t, removed, http.StatusSeeOther)
}

func TestTokenValueIsShownOnceOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/admin/tokens", url.Values{"name": {"agent"}, "scopes": {"task:read", "task:write"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	first := b.page("/admin/tokens")
	if !strings.Contains(first, "shown once") {
		t.Fatalf("the issued token was not shown:\n%s", first)
	}
	secret := between(t, first, "<pre>", "</pre>")
	if secret == "" {
		t.Fatalf("no token value was rendered")
	}

	second := b.page("/admin/tokens")
	if strings.Contains(second, secret) {
		t.Fatalf("the token value is retrievable after the first view")
	}
	if !strings.Contains(second, "agent") {
		t.Fatalf("the token is not listed")
	}

	revoked := b.post("/admin/tokens/revoke", url.Values{"id": {tokenID(t, second)}})
	defer func() { _ = revoked.Body.Close() }()
	wantStatus(t, revoked, http.StatusSeeOther)
}

// between returns the text between two markers.
func between(t *testing.T, page, start, end string) string {
	t.Helper()
	from := strings.Index(page, start)
	if from < 0 {
		return ""
	}
	rest := page[from+len(start):]
	to := strings.Index(rest, end)
	if to < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:to])
}

// tokenID reads the identifier of the first listed token.
func tokenID(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<input type="hidden" name="id" value="`, `"`)
}

func TestImportRunsAsADryRunAndReportsIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.postMultipart("/transfer/import",
		map[string]string{"mode": "merge", "dry_run": "1"},
		"snapshot.ndjson", `{"kind":"header","header":{"version":1,"tenant_key":"acme"}}`)
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	for _, want := range []string{"dry run", "task 2"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the import result does not report %q:\n%s", want, page)
		}
	}
	if !f.svc.lastImport.DryRun {
		t.Fatalf("the import was not run as a dry run")
	}
}

func TestImportAcceptsPastedSnapshotAndRefusesAnEmptyOne(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	pasted := b.post("/transfer/import", url.Values{"mode": {"merge"},
		"snapshot_text": {`{"kind":"header","header":{"version":1}}`}})
	wantStatus(t, pasted, http.StatusOK)
	_ = pasted.Body.Close()

	empty := b.post("/transfer/import", url.Values{"mode": {"merge"}})
	defer func() { _ = empty.Body.Close() }()
	if empty.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an empty import", empty.StatusCode)
	}

	noMode := b.post("/transfer/import", url.Values{"snapshot_text": {"{}"}})
	defer func() { _ = noMode.Body.Close() }()
	if noMode.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when no import mode is chosen", noMode.StatusCode)
	}
}

func TestExportDownloadsASnapshot(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/transfer/export", url.Values{"include_comments": {"1"}})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("the export is not offered as a download")
	}
	if !strings.Contains(page, `"kind":"header"`) {
		t.Fatalf("the export carries no snapshot")
	}
}

func TestSyncRunReportsItsResult(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/sync/sources", url.Values{"name": {"upstream"}, "system": {"generic"}})
	_ = created.Body.Close()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/sync")
	id := between(t, page, `<input type="hidden" name="source_id" value="`, `"`)
	resp := b.post("/sync/run", url.Values{"source_id": {id}, "dry_run": {"1"}})
	result := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	for _, want := range []string{"dry run", "task 1", "generic"} {
		if !strings.Contains(result, want) {
			t.Fatalf("the sync result does not report %q:\n%s", want, result)
		}
	}

	deleted := b.post("/sync/sources/delete", url.Values{"id": {id}})
	defer func() { _ = deleted.Body.Close() }()
	wantStatus(t, deleted, http.StatusSeeOther)
}

func TestSignOutClearsTheSessionCookie(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/logout", url.Values{})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/login" {
		t.Fatalf("location = %q, want /login", got)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "tix_session=") {
		t.Fatalf("the session cookie was not cleared")
	}
}

// plainForm matches a form that posts to the given action, whatever other
// attributes it carries. The property under test is that the control works
// with scripting turned off, not how the markup is spelled.
func hasPlainForm(page, action string) bool {
	for _, tag := range formTags.FindAllString(page, -1) {
		if strings.Contains(tag, `method="post"`) && strings.Contains(tag, `action="`+action+`"`) {
			return true
		}
	}
	return false
}

var formTags = regexp.MustCompile(`<form[^>]*>`)

// The tick box on the task list means "this is finished". A workflow rarely
// allows a jump from the opening state straight to a terminal one, so the
// action has to walk the path; if it only did the direct transition it would
// work for almost no task on the list.
func TestOneClickCompleteWalksTheWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "finish me")

	resp := b.post("/tasks/"+ref+"/complete", url.Values{"csrf_token": {b.csrf()}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("complete = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}

	detail := b.page("/tasks/" + ref)
	if !strings.Contains(detail, `<span class="pill status done">done</span>`) {
		t.Fatalf("the task did not reach a terminal state:\n%s", detail)
	}
}

// The control is a tick box, so it toggles. Clicking it on a finished task
// used to post "complete" again and change nothing, which reads as a broken
// control rather than as a deliberate no-op.
func TestUntickingAFinishedTaskReopensIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "finish then reopen")

	resp := b.post("/tasks/"+ref+"/complete", url.Values{"csrf_token": {b.csrf()}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("complete = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}
	if detail := b.page("/tasks/" + ref); !strings.Contains(detail, `<span class="pill status done">done</span>`) {
		t.Fatalf("the task did not reach a terminal state")
	}

	again := b.post("/tasks/"+ref+"/complete", url.Values{"csrf_token": {b.csrf()}})
	if again.StatusCode != http.StatusSeeOther {
		t.Fatalf("untick = %d, want 303: %s", again.StatusCode, body(t, again))
	}
	detail := b.page("/tasks/" + ref)
	if strings.Contains(detail, `<span class="pill status done">done</span>`) {
		t.Fatalf("unticking a finished task left it finished")
	}
	if !strings.Contains(detail, `<span class="pill status todo">todo</span>`) {
		t.Fatalf("the reopened task is not back in the starting state")
	}
}
