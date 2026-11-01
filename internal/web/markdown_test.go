// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// elementByID returns the element carrying id, up to its own closing tag,
// counting the nested elements of the same name so the cut is that element
// and not the first closing tag after it.
func elementByID(t *testing.T, page, tag, id string) string {
	t.Helper()
	attr := strings.Index(page, ` id="`+id+`"`)
	start := strings.LastIndex(page[:max(attr, 0)], "<"+tag)
	if attr < 0 || start < 0 || strings.Contains(page[start:attr], ">") {
		t.Fatalf("the page has no <%s id=%q>", tag, id)
	}
	depth, at := 0, start
	for {
		open := strings.Index(page[at:], "<"+tag)
		closing := strings.Index(page[at:], "</"+tag+">")
		if closing < 0 {
			t.Fatalf("<%s id=%q> is never closed", tag, id)
		}
		if open >= 0 && open < closing {
			depth++
			at += open + len(tag) + 1
			continue
		}
		depth--
		at += closing + len(tag) + 3
		if depth == 0 {
			return page[start:at]
		}
	}
}

func setBody(t *testing.T, f *fixture, ref, body string) {
	t.Helper()
	if _, err := f.svc.UpdateTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)},
		core.UpdateTaskInput{Body: &body}); err != nil {
		t.Fatalf("writing the description of %s: %v", ref, err)
	}
}

func taskOf(t *testing.T, f *fixture, ref string) *core.Task {
	t.Helper()
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)})
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task
}

const checklistBody = "Ship it **carefully**, see infra-1 and ask @alice.\n\n" +
	"<script>alert(1)</script>\n\n- [x] write the plan\n- [ ] get a review\n- [ ] deploy\n"

func TestTheDescriptionReadsAsMarkdown(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "rendered")
	setBody(t, f, ref, checklistBody)

	for _, who := range []string{"alice", "viewer"} {
		t.Run(who, func(t *testing.T) {
			page := f.as(who).page("/tasks/" + ref)
			if n := strings.Count(page, `id="task-desc"`); n != 1 {
				t.Errorf("the screen draws the description %d times, want once", n)
			}
			live := f.as(who).page("/tasks/" + ref + "/live")
			if !strings.Contains(elementByID(t, live, "div", "task-desc"), "<strong>carefully</strong>") {
				t.Errorf("the live refresh does not carry the description region")
			}
			desc := elementByID(t, page, "div", "task-desc")
			for _, want := range []string{
				"<strong>carefully</strong>",
				`<a href="/tasks/infra-1" class="taskref">infra-1</a>`,
				`<a href="/tasks?q=assignee%3Aalice" class="mention">@alice</a>`,
				"&lt;script&gt;alert(1)&lt;/script&gt;",
				`<span class="checkmark" title="1 of 3 checklist items done">`,
			} {
				if !strings.Contains(desc, want) {
					t.Errorf("the description lacks %q:\n%s", want, desc)
				}
			}
			if strings.Contains(desc, "<script>") {
				t.Errorf("the description let a script through:\n%s", desc)
			}
		})
	}
}

func TestAWriterTicksAChecklistInPlaceAndAReaderCannot(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "tickable")
	setBody(t, f, ref, checklistBody)
	action := "/tasks/" + ref + "/checklist"

	page := b.page("/tasks/" + ref)
	form := formAt(t, elementByID(t, page, "div", "task-desc"), action)
	for _, want := range []string{
		`name="version" value="` + strconv.Itoa(taskOf(t, f, ref).Version) + `"`,
		`id="` + ref + `-check-1" role="checkbox" aria-checked="true" name="uncheck" value="1" aria-label="write the plan"`,
		`id="` + ref + `-check-2" role="checkbox" aria-checked="false" name="check" value="2" aria-label="get a review"`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the checklist form lacks %q:\n%s", want, form)
		}
	}

	viewer := elementByID(t, f.as("viewer").page("/tasks/"+ref), "div", "task-desc")
	if strings.Contains(viewer, action) || strings.Contains(viewer, `class="check"`) {
		t.Errorf("a reader without task:write is offered a tick:\n%s", viewer)
	}
	if !strings.Contains(viewer, `<input type="checkbox" disabled aria-label="get a review">`) {
		t.Errorf("a reader does not see the items as checkboxes:\n%s", viewer)
	}

	version := strconv.Itoa(taskOf(t, f, ref).Version)
	resp := b.post(action, url.Values{"check": {"2"}, "version": {version}})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("ticking = %d, want 303", resp.StatusCode)
	}
	task := taskOf(t, f, ref)
	if !strings.Contains(task.Body, "- [x] get a review\n- [ ] deploy") {
		t.Fatalf("the tick did not land on item 2:\n%s", task.Body)
	}
	if task.Checklist == nil || task.Checklist.Done != 2 || task.Checklist.Total != 3 {
		t.Fatalf("checklist = %+v, want 2/3", task.Checklist)
	}

	stale := b.post(action, url.Values{"uncheck": {"1"}, "version": {version}})
	body := readAll(t, stale)
	if stale.StatusCode != http.StatusConflict || !strings.Contains(body, "has moved on") {
		t.Fatalf("a tick against an old version = %d, want the conflict refusal:\n%s", stale.StatusCode, body)
	}
	if got := taskOf(t, f, ref).Body; !strings.Contains(got, "- [x] write the plan") {
		t.Fatalf("a refused tick still changed the text:\n%s", got)
	}

	cleared := b.post(action, url.Values{"uncheck": {"1"}, "version": {strconv.Itoa(task.Version)}})
	_ = cleared.Body.Close()
	if got := taskOf(t, f, ref).Body; !strings.HasSuffix(got, "- [ ] write the plan\n- [x] get a review\n- [ ] deploy\n") {
		t.Fatalf("unchecking item 1 did not clear it:\n%s", got)
	}

	refused := f.as("viewer").post(action, url.Values{"check": {"3"}})
	_ = refused.Body.Close()
	if refused.StatusCode != http.StatusForbidden {
		t.Fatalf("a reader's tick = %d, want 403", refused.StatusCode)
	}
}

func TestEveryListingCarriesTheChecklistCount(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "counted")
	setBody(t, f, ref, checklistBody)
	plain := b.createTask("infra", "no list")

	board := f.as("alice")
	board.chooseBoard("/tasks")
	lanes := f.as("alice")
	lanes.chooseLanes("/tasks")
	cases := []struct {
		name, page, start, end string
		bar                    bool
	}{
		{"the list row", b.page("/tasks"), `<li id="t-` + ref + `"`, "</li>", false},
		{"the board card", board.page("/tasks"), `<article id="t-` + ref + `"`, "</article>", true},
		{"the lane card", lanes.page("/tasks"), `data-lane-task="` + ref + `"`, `class="lanemore"`, false},
		{"the project board", b.page("/projects/infra"), `data-task="` + ref + `"`, "</article>", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := between(t, tc.page, tc.start, tc.end)
			if item == "" {
				t.Fatalf("%s does not draw %s", tc.name, ref)
			}
			if !strings.Contains(item, `title="1 of 3 checklist items done"`) || !strings.Contains(item, " 1/3</span>") {
				t.Errorf("%s has no 1/3 checklist badge:\n%s", tc.name, item)
			}
			if tc.bar && !strings.Contains(item, `<progress class="checkbar" value="1" max="3"`) {
				t.Errorf("%s has no checklist progress bar:\n%s", tc.name, item)
			}
			other := between(t, tc.page, strings.Replace(tc.start, ref, plain, 1), tc.end)
			if other == "" || strings.Contains(other, "checkmark") {
				t.Errorf("%s badges a task with no checklist, or does not draw it:\n%s", tc.name, other)
			}
		})
	}
}

func TestPreviewRendersWithoutSaving(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "previewed")
	before := taskOf(t, f, ref).Version

	resp := b.post("/markdown/preview", url.Values{"body": {"# Plan\n\n- [ ] one\n\n<img src=x onerror=alert(1)> [x](javascript:alert(1))"}})
	out := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview = %d:\n%s", resp.StatusCode, out)
	}
	for _, want := range []string{"<h3>Plan</h3>", `<input type="checkbox" disabled aria-label="one">`, "&lt;img src=x onerror=alert(1)&gt;"} {
		if !strings.Contains(out, want) {
			t.Errorf("the preview lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<img") || strings.Contains(out, "javascript:") || strings.Contains(out, `class="check"`) {
		t.Errorf("the preview is not inert:\n%s", out)
	}
	if taskOf(t, f, ref).Version != before {
		t.Errorf("a preview wrote the task")
	}

	forged := b.postRaw("/markdown/preview", url.Values{"body": {"x"}})
	_ = forged.Body.Close()
	if forged.StatusCode == http.StatusOK {
		t.Errorf("a preview without the CSRF token was answered")
	}
}

func TestTheEditorOffersAPreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "editable")
	page := b.page("/tasks/" + ref)
	form := between(t, page, `action="/tasks/`+ref+`" data-dirty-save>`, "</form>")
	if !strings.Contains(form, `data-md-preview="body" data-md-pane="body-preview"`) ||
		!strings.Contains(form, `<div class="md mdpreview" id="body-preview"`) {
		t.Errorf("the description editor has no Preview toggle:\n%s", form)
	}
	if !strings.Contains(page, `<script src="/assets/markdown.js" defer></script>`) {
		t.Errorf("the page does not load the script that reveals Preview")
	}
}

func TestCommentsReadAsMarkdown(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "discussed")
	if _, err := f.svc.AddComment(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)},
		"done with `make check`\n<b onclick=x>bold</b>\n- [x] ran it"); err != nil {
		t.Fatal(err)
	}
	thread := elementByID(t, b.page("/tasks/"+ref), "ul", "task-comments")
	for _, want := range []string{"<code>make check</code>", "&lt;b onclick=x&gt;bold&lt;/b&gt;",
		`<input type="checkbox" disabled checked aria-label="ran it">`} {
		if !strings.Contains(thread, want) {
			t.Errorf("the comment lacks %q:\n%s", want, thread)
		}
	}
	if strings.Contains(thread, `class="check"`) {
		t.Errorf("a comment's checklist is tickable; only the description's is:\n%s", thread)
	}
}
