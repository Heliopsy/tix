// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// A project key is drawn upper case and still addressed in lower case: what
// a reader sees changes, the links and the values a form submits do not.
func TestTaskRefsAndKeysAreDrawnUpperCase(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "shout it")
	shown := strings.ToUpper(ref)

	task := b.page("/tasks/" + ref)
	if !strings.Contains(task, "<title>"+shown+" ") {
		t.Errorf("the task screen's title is not %s:\n%s", shown, between(t, task, "<title>", "</title>"))
	}
	if !strings.Contains(task, `<span class="ref">`+shown+`</span>`) {
		t.Errorf("the task screen does not name its task %s", shown)
	}
	if !strings.Contains(task, `<a href="/projects/infra">INFRA (INFRA)</a>`) {
		t.Errorf("the task screen's project link is not drawn with the key INFRA over /projects/infra:\n%s", between(t, task, "<dt>Project</dt>", "</dd>"))
	}

	list := b.page("/tasks")
	row := between(t, list, `<li id="t-`+ref+`"`, "</li>")
	if !strings.Contains(row, `<span class="ref mono">`+shown+`</span>`) {
		t.Errorf("the list row does not draw its ref as %s:\n%s", shown, row)
	}
	if strings.Contains(row, ">"+ref+"<") {
		t.Errorf("the list row still draws %s in lower case:\n%s", ref, row)
	}

	projects := b.page("/projects")
	if !strings.Contains(projects, `<span class="projrow-key mono">INFRA</span>`) {
		t.Errorf("the projects screen does not draw the key INFRA:\n%s", between(t, projects, `class="projrow-title"`, "</span></span>"))
	}
}

// Anything a reader types or follows reaches the same task or project in
// either case: the address, the filter, and the ref.
func TestRefsKeysAndFiltersAnswerInAnyCase(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "either case")
	shown := strings.ToUpper(ref)

	for _, path := range []string{"/tasks/" + shown, "/projects/INFRA", "/projects/Infra"} {
		resp := b.get(path)
		wantStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
	}
	if page := b.page("/tasks/" + shown); !strings.Contains(page, "either case") {
		t.Errorf("/tasks/%s does not show the task", shown)
	}

	for _, q := range []string{"project:INFRA", "project:infra", "project:Infra"} {
		page := b.page("/tasks?q=" + q)
		if !strings.Contains(page, `<li id="t-`+ref+`"`) {
			t.Errorf("the filter %s does not find %s", q, ref)
		}
	}
}
