// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// projectList is one section's list on the projects screen, empty when the
// section is not drawn.
func projectList(page, name string) string {
	start := strings.Index(page, `<ul class="projlist rows" data-projlist="`+name+`">`)
	if start < 0 {
		return ""
	}
	list, _, _ := strings.Cut(page[start:], "</ul>")
	return list
}

// TestTheProjectsScreenListsStarredProjectsFirst pins the order and the
// filter: a starred project leads in its own section and is not repeated
// below, and ?starred=1 draws that section alone.
func TestTheProjectsScreenListsStarredProjectsFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := aliceCtx(f)
	for _, key := range []string{"web", "ops"} {
		if _, err := f.svc.CreateProject(ctx, core.CreateProjectInput{Key: key, Name: strings.ToUpper(key), WorkflowKey: "default"}); err != nil {
			t.Fatal(err)
		}
	}
	b := f.as("alice")
	page := b.page("/projects")
	if projectList(page, "starred") != "" || strings.Contains(page, `id="projs-starred"`) {
		t.Error("a reader with no stars gets a Starred section")
	}
	if all := projectList(page, "all"); strings.Count(all, `" id="p-`) != 3 {
		t.Errorf("the projects are not all listed:\n%s", all)
	}

	starProject(t, b, "web", true)
	page = b.page("/projects")
	starred, rest := projectList(page, "starred"), projectList(page, "all")
	if !strings.Contains(starred, `id="p-WEB"`) || strings.Count(starred, `" id="p-`) != 1 {
		t.Errorf("Starred does not hold WEB alone:\n%s", starred)
	}
	if strings.Contains(rest, `id="p-WEB"`) || strings.Count(rest, `" id="p-`) != 2 {
		t.Errorf("the rest repeats the starred project or lost one:\n%s", rest)
	}
	if s, a := strings.Index(page, `data-projlist="starred"`), strings.Index(page, `data-projlist="all"`); s < 0 || a < s {
		t.Errorf("Starred is not drawn first (%d, %d)", s, a)
	}
	if !strings.Contains(between(t, page, `<h2 id="projs-all"`, "</h2>"), "Other projects") {
		t.Error("the rest is not headed Other projects")
	}

	only := b.page("/projects?starred=1")
	if projectList(only, "all") != "" || !strings.Contains(projectList(only, "starred"), `id="p-WEB"`) {
		t.Errorf("Starred only draws more than the starred projects:\n%s", only)
	}
	if !strings.Contains(between(t, only, `<nav class="projfilter"`, "</nav>"), `href="/projects?starred=1" aria-current="page"`) {
		t.Error("Starred only is not marked current")
	}
	if !strings.Contains(starForm(t, only, "web"), `name="next" value="/projects?starred=1"`) {
		t.Error("the toggle under Starred only does not return there")
	}

	empty := f.as("member:someone-else").page("/projects?starred=1")
	if !strings.Contains(projectList(empty, "starred"), "No starred projects yet") {
		t.Errorf("another reader's Starred only is not empty:\n%s", projectList(empty, "starred"))
	}
}

// TestTheProjectPageHeaderStarsTheProject pins the header toggle: drawn
// beside the name, it stars and returns to the project's page.
func TestTheProjectPageHeaderStarsTheProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/projects/infra")
	head := between(t, page, `<div class="headtip"><h1>`, "</div>")
	if !strings.Contains(head, `class="projstar" action="/projects/infra/star"`) || !strings.Contains(head, `aria-pressed="false"`) {
		t.Fatalf("the project header has no star toggle:\n%s", head)
	}
	resp := b.post("/projects/infra/star", map[string][]string{"next": {"/projects/infra"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/projects/infra?flash=") {
		t.Errorf("the header toggle does not return to the project page: %q", loc)
	}
	if head := between(t, b.page("/projects/infra"), `<div class="headtip"><h1>`, "</div>"); !strings.Contains(head, `aria-pressed="true"`) || !strings.Contains(head, `action="/projects/infra/star/remove"`) {
		t.Errorf("the header toggle does not show the star:\n%s", head)
	}
}
