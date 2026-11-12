// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// manageGroup is the sidebar's folded group, from its opening tag to the
// end of the navigation.
func manageGroup(t *testing.T, page string) string {
	t.Helper()
	links := nav(t, page)
	start := strings.Index(links, `data-navgroup="admin"`)
	if start < 0 {
		t.Fatalf("the sidebar draws no Manage group:\n%s", links)
	}
	return links[strings.LastIndex(links[:start], "<"):]
}

// TestTemplatesFollowsProjectsOutsideManage pins C3: Templates is a main
// entry right after Projects and its starred list, drawn without Advanced
// screens, and the folded group under it is headed Manage while keeping the
// id the fold cookie stores.
func TestTemplatesFollowsProjectsOutsideManage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	member := f.as("member:plain")
	member.setCookie(web.AdvancedCookie, "0")
	page := member.page("/tasks")
	links := nav(t, page)
	order := []string{`href="/inbox"`, `href="/mine"`, `href="/tasks"`, `href="/projects"`, `href="/templates"`, `href="/activity"`, `href="/stats"`}
	last := -1
	for _, href := range order {
		at := strings.Index(links, href)
		if at <= last {
			t.Fatalf("%s is not after the entry before it in %v:\n%s", href, order, links)
		}
		last = at
	}
	if pins := strings.Index(links, `class="side-pins"`); pins >= 0 && pins > strings.Index(links, `href="/templates"`) {
		t.Errorf("Templates comes before the starred projects:\n%s", links)
	}
	resp := member.get("/templates")
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusOK)

	alice := f.as("alice")
	alice.setCookie(web.AdvancedCookie, "1")
	group := manageGroup(t, alice.page("/tasks"))
	if strings.Contains(group, `href="/templates"`) {
		t.Errorf("Templates is still inside the folded group:\n%s", group)
	}
	head := between(t, group, `<p class="group navgroup-head`, "</p>")
	for _, want := range []string{`<span class="navgroup-name">Manage</span>`, `<span class="navgroup-label">Manage</span>`, `data-navgroup-toggle="admin"`} {
		if !strings.Contains(head, want) {
			t.Errorf("the group heading lacks %s:\n%s", want, head)
		}
	}
	if strings.Contains(head, "Admin") {
		t.Errorf("the group is still headed Admin:\n%s", head)
	}
	if n := strings.Count(nav(t, alice.page("/tasks")), `href="/templates"`); n != 1 {
		t.Errorf("Templates is drawn %d times, want once", n)
	}
}

// TestTheActionsMenuCopiesTheReference pins C10: the task's Actions menu
// copies the reference as it is displayed, and the ULID is copied only from
// the Id row under the properties.
func TestTheActionsMenuCopiesTheReference(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "copy my reference")
	page := b.page("/tasks/" + ref)
	menu := actionsMenu(t, page, ref)
	if want := `data-copy-text="` + strings.ToUpper(ref) + `">Copy reference</button>`; !strings.Contains(menu, want) {
		t.Errorf("the actions menu does not copy the reference, want %s:\n%s", want, menu)
	}
	id := between(t, page, `id="task-id-`+ref+`" title="`, `"`)
	if id == "" {
		t.Fatal("the task screen draws no Id row")
	}
	if strings.Contains(menu, id) || strings.Contains(menu, "Copy the task id") {
		t.Errorf("the actions menu still copies the task id %s:\n%s", id, menu)
	}
	if !strings.Contains(page, `data-copy-target="task-id-`+ref+`" aria-label="Copy the task id"`) {
		t.Error("the Id row lost its own Copy button")
	}
}

// TestQuickAddPlaceholderShowsTheSyntax pins C12: the box's placeholder is
// the hint line both surfaces show, with only "Fix login" translated.
func TestQuickAddPlaceholderShowsTheSyntax(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	box := quickBox(t, b.page("/tasks"))
	for _, want := range []string{`placeholder="Fix login !high #auth @raj due:fri &#43;web ^web-3"`, `aria-label="New task"`, `<details class="field-info tip">`} {
		if !strings.Contains(box, want) {
			t.Errorf("the add box lacks %s:\n%s", want, box)
		}
	}
	b.lang = "bg"
	if box := quickBox(t, b.page("/tasks")); !strings.Contains(box, ` !high #auth @raj due:fri &#43;web ^web-3"`) || strings.Contains(box, `placeholder="Fix login`) {
		t.Errorf("the Bulgarian placeholder is not the translated words and the same syntax:\n%s", box)
	}
}
