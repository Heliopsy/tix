// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// leaseRow is the task screen's Lease property, cut at the next property, so
// the sentence under it is read from that row and not from the page.
func leaseRow(t *testing.T, page string) string {
	t.Helper()
	open := `<div class="prop" data-prop="lease">`
	start := strings.Index(page, open)
	if start < 0 {
		t.Fatalf("the task screen has no Lease row")
	}
	row := page[start+len(open):]
	if end := strings.Index(row, `<div class="prop"`); end >= 0 {
		row = row[:end]
	}
	return row
}

func TestTheBoardAndTheTaskScreenSetLeaseLength(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "long build")

	if row := leaseRow(t, b.page("/tasks/"+ref)); !strings.Contains(row, "Claims last 30m, from the workflow") {
		t.Fatalf("a fresh task's lease row does not name the fixture workflow's 30m:\n%s", row)
	}

	formAt(t, b.page("/projects/infra"), "/projects/infra/lease")
	saved := b.post("/projects/infra/lease", url.Values{"lease_default": {"2h"}, "lease_max": {""}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	if got := inputValue(t, b.page("/projects/infra"), "lease-default"); got != "2h" {
		t.Fatalf("the Claims panel holds %q, want 2h", got)
	}
	page := b.page("/tasks/" + ref)
	if row := leaseRow(t, page); !strings.Contains(row, "Claims last 2h, from the project") {
		t.Fatalf("the lease row does not name the project's 2h:\n%s", row)
	}

	form := formAt(t, page, "/tasks/"+ref)
	v := between(t, form, `name="version" value="`, `"`)
	set := b.post("/tasks/"+ref, url.Values{"version": {v}, "title": {"long build"}, "lease": {"4h"}})
	_ = set.Body.Close()
	wantStatus(t, set, http.StatusSeeOther)
	page = b.page("/tasks/" + ref)
	row := leaseRow(t, page)
	if got := inputValue(t, row, "lease"); got != "4h" || !strings.Contains(row, "Claims last 4h, set on this task") {
		t.Fatalf("after setting 4h the row holds %q:\n%s", got, row)
	}

	v = between(t, formAt(t, page, "/tasks/"+ref), `name="version" value="`, `"`)
	other := b.post("/tasks/"+ref, url.Values{"version": {v}, "title": {"long build"}, "assignee": {""}})
	_ = other.Body.Close()
	if row := leaseRow(t, b.page("/tasks/"+ref)); !strings.Contains(row, "set on this task") {
		t.Fatalf("saving another property cleared the lease:\n%s", row)
	}

	tooLong := b.post("/projects/infra/lease", url.Values{"lease_max": {"48h"}})
	_ = tooLong.Body.Close()
	if tooLong.StatusCode == http.StatusSeeOther {
		t.Error("a project maximum above the tenant's was saved")
	}
}

func TestAddWithDetailsSetsTheLease(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"long build"}, "lease": {"2h"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	ref := createdRef(resp.Header.Get("Location"))
	row := leaseRow(t, b.page("/tasks/"+ref))
	if got := inputValue(t, row, "lease"); got != "2h" || !strings.Contains(row, "Claims last 2h, set on this task") {
		t.Fatalf("a task added with 2h holds %q:\n%s", got, row)
	}
}
