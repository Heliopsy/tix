// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/csv"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// bulkBar is the selection bar's form, and fails when the list draws none.
func bulkBar(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<form id="bulkbar"`, "</form>")
}

func readTask(t *testing.T, f *fixture, ref string) *core.Task {
	t.Helper()
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)})
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task
}

func TestEveryListRowCarriesASelectionBoxForTheBar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	refs := []string{b.createTask("infra", "one"), b.createTask("infra", "two")}

	page := b.page("/tasks")
	for _, ref := range refs {
		row := between(t, page, `<li id="t-`+ref+`"`, "</li>")
		box := `<input type="checkbox" class="rowpick" name="ref" value="` + ref + `" form="bulkbar"`
		if !strings.Contains(row, box) {
			t.Errorf("row %s has no selection box belonging to the bar:\n%s", ref, row)
		}
	}
	bar := bulkBar(t, page)
	for _, action := range []string{"move", "priority", "tag_add", "tag_remove", "assign", "star", "unstar", "copy", "delete"} {
		if !strings.Contains(bar, `formaction="/tasks/bulk?action=`+action+`"`) {
			t.Errorf("the bar offers no %s", action)
		}
	}
	if !strings.Contains(bar, `<button type="reset"`) {
		t.Error("the bar has no Clear that works without scripting")
	}

	board := f.as("alice")
	board.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	if strings.Contains(board.page("/tasks"), `class="rowpick"`) {
		t.Error("the board draws selection boxes; only the list selects")
	}
}

func TestAReaderWhoMayDoNothingGetsNoSelection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.as("alice").createTask("infra", "one")
	page := f.as("viewer").page("/tasks")
	bar := bulkBar(t, page)
	for _, action := range []string{"move", "priority", "delete", "copy"} {
		if strings.Contains(bar, `action=`+action+`"`) {
			t.Errorf("a viewer is offered a bulk %s", action)
		}
	}
	if !strings.Contains(bar, `action=star"`) {
		t.Error("a viewer may star and is not offered it")
	}
}

func TestBulkActionChangesEverySelectedTaskAndNamesRefusals(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	one, two, three := b.createTask("infra", "one"), b.createTask("infra", "two"), b.createTask("infra", "three")

	resp := b.post("/tasks/bulk?action=priority", url.Values{
		"ref": {one, two, "infra-99"}, "priority": {"1"}, "next": {"/tasks?q=tag:x"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/tasks" || loc.Query().Get("q") != "tag:x" {
		t.Errorf("returned to %s, want the listing it came from", loc)
	}
	flash := loc.Query().Get("flash")
	if !strings.Contains(flash, "Set 2 tasks to highest priority") || !strings.Contains(flash, "1 refused: infra-99 (") {
		t.Errorf("flash = %q, want the count and the refusal named", flash)
	}
	for ref, want := range map[string]core.Priority{one: core.PriorityHighest, two: core.PriorityHighest, three: core.PriorityNormal} {
		if got := readTask(t, f, ref).Priority; got != want {
			t.Errorf("%s priority = %d, want %d", ref, got, want)
		}
	}
}

func TestBulkDeleteOffersOneUndoForTheBatch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	refs := []string{b.createTask("infra", "one"), b.createTask("infra", "two"), b.createTask("infra", "three")}

	resp := b.post("/tasks/bulk?action=delete", url.Values{"ref": refs[:2], "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	location := resp.Header.Get("Location")
	if !strings.Contains(location, "flashundo=") {
		t.Fatalf("location = %q, want an undo", location)
	}
	page := b.page(location)
	undo := between(t, page, `<form id="flash-undo" method="post" action="/tasks/bulk"`, "</form>")
	for _, want := range []string{
		`name="action" value="restore"`,
		`name="ref" value="` + refs[0] + `"`,
		`name="ref" value="` + refs[1] + `"`,
	} {
		if !strings.Contains(undo, want) {
			t.Errorf("the undo form lacks %s:\n%s", want, undo)
		}
	}
	if strings.Contains(undo, refs[2]) {
		t.Errorf("the undo names %s, which was not deleted", refs[2])
	}

	restored := b.post("/tasks/bulk", url.Values{"action": {"restore"}, "ref": refs[:2], "next": {"/tasks"}})
	defer func() { _ = restored.Body.Close() }()
	wantStatus(t, restored, http.StatusSeeOther)
	for _, ref := range refs[:2] {
		if readTask(t, f, ref).DeletedAt != nil {
			t.Errorf("%s is still deleted after Undo", ref)
		}
	}
}

func TestARefusedBulkRequestSaysWhyOnTheList(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")
	resp := b.post("/tasks/bulk?action=move", url.Values{"ref": {ref}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if flash := resp.Header.Get("Location"); !strings.Contains(flash, "flash=a+move+needs") {
		t.Errorf("location = %q, want the refusal as a toast", flash)
	}
}

func TestExportCSVFollowsTheFilterAndTheColumnPicker(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	tagged := b.createTask("infra", "=cmd|' /C calc'!A0")
	b.createTask("infra", "untagged")
	if err := f.svc.AddTag(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, tagged)}, "ops"); err != nil {
		t.Fatal(err)
	}

	page := b.page("/tasks?q=tag:ops")
	if !strings.Contains(page, `href="/tasks/export.csv?q=tag%3Aops" download`) {
		t.Fatalf("the View menu has no export of this listing")
	}

	resp := b.post("/columns", url.Values{"page": {"tasks"}, "column": {"ref", "status"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	got := b.get("/tasks/export.csv?q=tag%3Aops")
	defer func() { _ = got.Body.Close() }()
	wantStatus(t, got, http.StatusOK)
	if ct := got.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content type = %q", ct)
	}
	records, err := csv.NewReader(strings.NewReader(readAll(t, got))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(records[0], ",") != "title,status,ref" {
		t.Errorf("header = %v, want the title and the two ticked columns", records[0])
	}
	if len(records) != 2 || records[1][0] != "'=cmd|' /C calc'!A0" || records[1][2] != tagged {
		t.Errorf("rows = %v, want only the tagged task with its formula defused", records)
	}
}
