// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// Putting one project away must not put every project the listing did not
// reach away with it.
//
// The task screen turns the visibility choice into an explicit ProjectKeys
// filter built from one page of ListProjects. That call asks for no limit, so
// the store answers with core.DefaultPageLimit rows and the cursor is thrown
// away: past that many projects, "hide one" quietly becomes "show only the
// first page", and the tasks in every other project leave the screen with no
// pager, no count and no message saying so.
func TestHidingOneProjectKeepsTasksFromProjectsPastTheFirstListingPage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	all := map[string]bool{f.project.Key: true}
	for i := 0; i < core.DefaultPageLimit+10; i++ {
		key := fmt.Sprintf("proj%03d", i)
		seedProject(t, f.store, f.tenantA.ID, key)
		all[key] = true
	}

	listed, _, err := f.svc.ListProjects(f.ctx(), core.ProjectFilter{})
	if err != nil {
		t.Fatalf("listing projects: %v", err)
	}
	if len(listed) >= len(all) {
		t.Fatalf("the listing returned %d of %d projects, so nothing is past its first page",
			len(listed), len(all))
	}
	onPage := map[string]bool{}
	for _, p := range listed {
		onPage[p.Key] = true
	}
	far := ""
	for key := range all {
		if !onPage[key] {
			far = key
			break
		}
	}
	if far == "" {
		t.Fatal("every project fitted on the first listing page")
	}

	ref := b.createTask(far, "work in a project past the first page")
	if page := b.page("/tasks"); !strings.Contains(page, ref) {
		t.Fatalf("the task list does not show %s before anything is hidden:\n%s", ref, page)
	}

	// Submit exactly what the control offers, minus one tick: the gesture is
	// "put this one away", and a browser can only send back the boxes the
	// screen actually rendered.
	offered := projectCheckboxes(t, b.page("/tasks"))
	if len(offered) < 2 {
		t.Fatalf("the visibility control offers %d projects", len(offered))
	}
	shown := url.Values{}
	for _, key := range offered[1:] {
		shown.Add("project", key)
	}
	shown.Set("next", "/tasks")
	saved := b.post("/visibility", shown)
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	after := b.page("/tasks")
	if !strings.Contains(after, ref) {
		t.Errorf("hiding one project also removed %s, which lives in project %q and was never offered as a choice",
			ref, far)
	}
}

// projectCheckboxes names the projects the visibility control renders a box
// for, in the order it renders them, which is everything a browser can send
// back.
func projectCheckboxes(t *testing.T, page string) []string {
	t.Helper()
	const open = `<input type="checkbox" name="project" value="`
	var out []string
	for rest := page; ; {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		end := strings.Index(rest, `"`)
		if end < 0 {
			t.Fatalf("a project checkbox has no closing quote")
		}
		out = append(out, rest[:end])
	}
}
