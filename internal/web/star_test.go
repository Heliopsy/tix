// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

var (
	rowRefPattern  = regexp.MustCompile(`<li id="t-([^"]+)" class="task`)
	cardRefPattern = regexp.MustCompile(`<article (?:id="t-[^"]+" )?class="card[^"]*"[^>]*data-task="([^"]+)"`)
)

// refsIn lists the task references a region draws, in the order it draws them.
func refsIn(region string, pattern *regexp.Regexp) []string {
	var out []string
	for _, m := range pattern.FindAllStringSubmatch(region, -1) {
		out = append(out, m[1])
	}
	return out
}

// starButton reads the one star form posting to action, and fails when the
// screen draws none.
func starButton(t *testing.T, page, action string) string {
	t.Helper()
	form := formAt(t, page, action)
	if !strings.Contains(form, `<button type="submit" class="star`) {
		t.Fatalf("the form posting to %s is not a star button:\n%s", action, form)
	}
	return form
}

func isStarred(t *testing.T, f *fixture, ref string) bool {
	t.Helper()
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)})
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task.Starred
}

func seqOfRef(t *testing.T, ref string) int64 {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatalf("parsing %q: %v", ref, err)
	}
	return parsed.Seq
}

func TestEveryTaskScreenOffersAStarButton(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "worth watching")
	action := "/tasks/" + ref + "/star"

	board := f.as("alice")
	board.setCookie(web.TaskViewCookie, web.TaskViewBoard)

	cases := []struct {
		name string
		page string
		next string
	}{
		{"the task list", b.page("/tasks"), "/tasks"},
		{"the task screen's board", board.page("/tasks"), "/tasks"},
		{"the project board", b.page("/projects/infra"), "/projects/infra"},
		{"the task screen", b.page("/tasks/" + ref), "/tasks/" + ref},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := starButton(t, tc.page, action)
			for _, want := range []string{
				`aria-pressed="false"`, `aria-label="Star ` + strings.ToUpper(ref) + `"`,
				`name="csrf_token"`, `name="next" value="` + tc.next,
			} {
				if !strings.Contains(form, want) {
					t.Errorf("star form lacks %s:\n%s", want, form)
				}
			}
		})
	}
}

func TestPressingTheStarStarsTheTaskAndReturnsToTheScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "first")
	second := b.createTask("infra", "second")
	next := "/tasks?q=project%3Ainfra"

	resp := b.post("/tasks/"+second+"/star", url.Values{"next": {next}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, next+"&flash=") || !strings.HasSuffix(loc, "#t-"+second) {
		t.Errorf("redirected to %q, want back to %s at the row", loc, next)
	}
	if !isStarred(t, f, second) {
		t.Fatal("pressing star did not star the task")
	}

	list := b.page("/tasks")
	if got := refsIn(list, rowRefPattern); len(got) < 2 || got[0] != second || got[1] != first {
		t.Errorf("rows drawn as %v, want the starred %s first", got, second)
	}
	pressed := starButton(t, list, "/tasks/"+second+"/star/remove")
	if !strings.Contains(pressed, `aria-pressed="true"`) || !strings.Contains(pressed, `aria-label="Unstar `+strings.ToUpper(second)+`"`) {
		t.Errorf("the starred row's button is not pressed:\n%s", pressed)
	}

	boardPage := b.page("/projects/infra")
	if got := refsIn(boardPage, cardRefPattern); len(got) < 2 || got[0] != second {
		t.Errorf("project board column drawn as %v, want the starred %s first", got, second)
	}
	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	if got := refsIn(b.page("/tasks"), cardRefPattern); len(got) < 2 || got[0] != second {
		t.Errorf("task screen board drawn as %v, want the starred %s first", got, second)
	}

	screen := b.page("/tasks/" + second)
	starButton(t, screen, "/tasks/"+second+"/star/remove")
	resp = b.post("/tasks/"+second+"/star/remove", url.Values{"next": {"/tasks/" + second}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if isStarred(t, f, second) {
		t.Fatal("pressing the pressed star did not unstar the task")
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks/"+second) {
		t.Errorf("unstarring from the task screen went to %q", loc)
	}
	off := starButton(t, b.page("/tasks/"+second), "/tasks/"+second+"/star")
	if !strings.Contains(off, `aria-pressed="false"`) {
		t.Errorf("the task screen still draws the star pressed:\n%s", off)
	}
}

func TestAStarPressWithoutAuthorityIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "not yours to star")

	resp := f.as("useradmin").post("/tasks/"+ref+"/star", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusSeeOther {
		t.Fatalf("a reader without task:read starred: status %d", resp.StatusCode)
	}
	if isStarred(t, f, ref) {
		t.Fatal("the refused press starred the task anyway")
	}
}
