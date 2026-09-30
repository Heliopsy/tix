// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// seedProjectsPastAPage fills the tenant past one listing page and returns the
// key of a project that cannot be on the first one.
func seedProjectsPastAPage(t *testing.T, f *fixture) string {
	t.Helper()
	all := map[string]bool{f.project.Key: true}
	for i := range core.DefaultPageLimit + 10 {
		key := fmt.Sprintf("far%03d", i)
		seedProject(t, f.store, f.tenantA.ID, key)
		all[key] = true
	}
	listed, _, err := f.svc.ListProjects(f.ctx(), core.ProjectFilter{})
	if err != nil {
		t.Fatalf("listing projects: %v", err)
	}
	if len(listed) >= len(all) {
		t.Fatalf("the default listing returned %d of %d projects, so nothing is past its first page",
			len(listed), len(all))
	}
	onPage := map[string]bool{}
	for _, p := range listed {
		onPage[p.Key] = true
	}
	for key := range all {
		if !onPage[key] {
			return key
		}
	}
	t.Fatal("every project fitted on the first listing page")
	return ""
}

// A control that cannot name a project cannot put it away, and a reader
// looking for it there is told, silently, that it does not exist. The control
// is rendered from the project listing, so it inherited whatever that listing
// stopped at.
func TestTheVisibilityControlOffersEveryProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	far := seedProjectsPastAPage(t, f)

	offered := projectCheckboxes(t, b.page("/tasks"))
	if len(offered) != core.DefaultPageLimit+11 {
		t.Errorf("the visibility control offers %d projects, want %d",
			len(offered), core.DefaultPageLimit+11)
	}
	if !slices.Contains(offered, far) {
		t.Errorf("the control offers no box for %q, which is past the first listing page", far)
	}
}

// The tenant diagram prints this figure as the tenant's project count, with
// no note saying it is a sample. A capped listing made it a sample anyway.
func TestTheTenantDiagramCountsEveryProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedProjectsPastAPage(t, f)

	row := shapeRow(t, b.page("/admin/tenant"), "Projects")
	want := strconv.Itoa(core.DefaultPageLimit + 11)
	if !strings.Contains(row, ">"+want+"<") {
		t.Errorf("the Projects row does not show %s:\n%s", want, row)
	}
}

// Ticking a task done resolves its project out of the same listing, to reach
// the workflow that says where done is. Past the first page that resolution
// failed, so the tick box answered Not found for a task that plainly exists.
func TestTickingATaskDoneInAProjectPastTheFirstListingPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	far := seedProjectsPastAPage(t, f)

	ref := b.createTask(far, "work out here")
	done := b.post("/tasks/"+ref+"/complete", url.Values{"next": {"/tasks"}})
	defer func() { _ = done.Body.Close() }()
	if done.StatusCode != http.StatusSeeOther {
		t.Fatalf("ticking %s done answered %d, want 303", ref, done.StatusCode)
	}
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: far, Seq: seqOf(t, ref)})
	if err != nil {
		t.Fatalf("reading %s back: %v", ref, err)
	}
	if task.CompletedAt == nil {
		t.Errorf("the task was not finished: status %q", task.Status)
	}
}
