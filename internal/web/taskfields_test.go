// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// hideTaskFields submits the View picker with every task field ticked except
// the ones named.
func (b *browser) hideTaskFields(hidden ...string) {
	b.t.Helper()
	keep := []string{}
	for _, key := range []string{"star", "status", "priority", "tags", "assignee", "project", "due", "repeat", "updated", "ref"} {
		drop := false
		for _, h := range hidden {
			drop = drop || h == key
		}
		if !drop {
			keep = append(keep, key)
		}
	}
	resp := b.post("/columns", url.Values{"page": {"tasks"}, "column": keep, "next": {"/tasks"}})
	_ = resp.Body.Close()
	wantStatus(b.t, resp, http.StatusSeeOther)
}

// View offers Star and Repeats, both on for a browser that never chose.
func TestViewOffersStarAndRepeats(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "anything")
	picker := between(t, b.page("/tasks"), `id="viewprefs"`, "</form>")
	for _, key := range []string{"star", "repeat"} {
		if !strings.Contains(picker, `value="`+key+`" checked`) {
			t.Errorf("View does not offer %s, ticked:\n%s", key, picker)
		}
	}
}

// The row writes the rule out beside the glyph, and hiding Repeats takes the
// whole marker off rows, lane cards and board cards.
func TestTheRepeatsFieldShowsTheRuleAndHidesEverywhere(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	task, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "water plants", Recurrence: "every week", Tags: []string{"home"}})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	b := f.as("alice")
	row := dueRow(t, b.page("/tasks"), "water plants")
	if rule := between(t, row, `<span class="repeat-rule">`, "</span>"); rule != "every week" {
		t.Errorf("the row does not write the rule out, it says %q:\n%s", rule, row)
	}

	b.hideTaskFields("repeat", "tags")
	if row := dueRow(t, b.page("/tasks"), "water plants"); strings.Contains(row, "repeatmark") {
		t.Errorf("hiding Repeats left the marker on the row:\n%s", row)
	}
	b.setCookie(web.TaskViewCookie, web.TaskViewLanes)
	card := laneCard(t, b.page("/tasks"), task.Ref)
	if strings.Contains(card, "repeatmark") || strings.Contains(card, "tagref") {
		t.Errorf("hiding Repeats and Tags left them on the lane card:\n%s", card)
	}
	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	card = taskBoardCard(t, b.page("/tasks"), task.Ref)
	if strings.Contains(card, "repeatmark") || strings.Contains(card, "tagref") {
		t.Errorf("hiding Repeats and Tags left them on the board card:\n%s", card)
	}
}

// The same cards draw the fields when nothing is hidden, so the hidden case
// above is the choice taking effect and not a card that never drew them.
func TestCardsDrawTheChosenFields(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	task, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "water plants", Recurrence: "every week", Tags: []string{"home"}})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	b := f.as("alice")
	b.setCookie(web.TaskViewCookie, web.TaskViewLanes)
	card := laneCard(t, b.page("/tasks"), task.Ref)
	for _, want := range []string{"repeatmark", "tagref", "lane-state", `action="/tasks/` + task.Ref + `/star"`} {
		if !strings.Contains(card, want) {
			t.Errorf("the lane card draws no %s:\n%s", want, card)
		}
	}
	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	card = taskBoardCard(t, b.page("/tasks"), task.Ref)
	for _, want := range []string{"repeatmark", "tagref", `action="/tasks/` + task.Ref + `/star"`} {
		if !strings.Contains(card, want) {
			t.Errorf("the board card draws no %s:\n%s", want, card)
		}
	}
}

// Hiding Star removes the toggle and nothing else: the starred task still
// comes first, and its own screen still has the star.
func TestHidingTheStarKeepsTheOrder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "first made")
	second := b.createTask("infra", "starred one")
	resp := b.post("/tasks/"+second+"/star", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	b.hideTaskFields("star")
	list := between(t, b.page("/tasks"), `<ul class="tasklist">`, "</ul>")
	if strings.Contains(list, `class="starform"`) {
		t.Errorf("hiding Star left a toggle on the list:\n%s", list)
	}
	if strings.Index(list, "starred one") > strings.Index(list, "first made") {
		t.Errorf("hiding Star changed the order:\n%s", list)
	}
	if !strings.Contains(b.page("/tasks/"+second), `action="/tasks/`+second+`/star/remove"`) {
		t.Errorf("hiding Star took the toggle off the task's own screen")
	}
}

// The properties: the star in words on its control, the parent, and the next occurrence the
// rule would create. The due date is far enough out that "finished now" does
// not depend on the day the suite runs.
func TestTheTaskScreenStatesStarParentAndNextOccurrence(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	due := time.Date(2099, 1, 5, 9, 0, 0, 0, time.UTC)
	parent, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "the whole job"})
	if err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	weekly, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "weekly", Recurrence: "every week",
		DueAt: &due, ParentRef: parent.Ref})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	last, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "once more", Recurrence: "every week for 1 time", DueAt: &due})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	b := f.as("alice")
	resp := b.post("/tasks/"+weekly.Ref+"/star", url.Values{"next": {"/tasks/" + weekly.Ref}})
	_ = resp.Body.Close()

	page := b.page("/tasks/" + weekly.Ref)
	if star := between(t, header(t, page), `<button type="submit" class="star`, ">"); !strings.Contains(star, `aria-pressed="true"`) || !strings.Contains(star, `aria-label="Unstar `+weekly.Ref+`"`) {
		t.Errorf("the starred task does not say so:\n%s", star)
	}
	if row := aboutRow(t, page, "parent"); !strings.Contains(row, `href="/tasks/`+parent.Ref+`"`) {
		t.Errorf("the subtask does not link its parent:\n%s", row)
	}
	if row := aboutRow(t, page, "repeat"); !strings.Contains(row, "next due") || !strings.Contains(row, "2099-01-12") {
		t.Errorf("the weekly task does not give the next occurrence's date:\n%s", row)
	}
	if star := between(t, header(t, b.page("/tasks/"+parent.Ref)), `<button type="submit" class="star`, ">"); !strings.Contains(star, `aria-pressed="false"`) || !strings.Contains(star, `aria-label="Star `+parent.Ref+`"`) {
		t.Errorf("an unstarred task does not say so:\n%s", star)
	}
	if row := aboutRow(t, b.page("/tasks/"+last.Ref), "repeat"); !strings.Contains(row, "last occurrence") || strings.Contains(row, "next due") {
		t.Errorf("the rule that has run out does not say so:\n%s", row)
	}
}

// taskBoardCard is one card of the task screen's board, found by its ref.
func taskBoardCard(t *testing.T, page, ref string) string {
	t.Helper()
	start := strings.Index(page, `<article id="t-`+ref+`"`)
	if start < 0 {
		t.Fatalf("no board card for %s", ref)
	}
	card, _, _ := strings.Cut(page[start:], "</article>")
	return card
}
