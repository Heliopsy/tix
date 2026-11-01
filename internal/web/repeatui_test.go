// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// aboutRow returns the value of one property row of the task screen, found
// by its key, so an assertion about the rule cannot pass on the due date.
func aboutRow(t *testing.T, page, key string) string {
	t.Helper()
	return propValue(t, page, key)
}

func TestTheBrowserSetsShowsAndClearsARepeatRule(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "water plants")

	page := b.page("/tasks/" + ref)
	if row := propRow(t, page, "repeat"); !strings.Contains(row, `name="repeat"`) {
		t.Fatalf("the Repeats row offers no field:\n%s", row)
	}
	if n := strings.Count(page, `name="repeat"`); n != 1 {
		t.Fatalf("%d forms name the rule, so saving another property would rewrite it", n)
	}

	save := func(fields url.Values) *http.Response {
		t.Helper()
		current := b.page("/tasks/" + ref)
		fields.Set("version", between(t, propRow(t, current, "repeat"), `name="version" value="`, `"`))
		fields.Set("title", "water plants")
		return b.post("/tasks/"+ref, fields)
	}

	resp := save(url.Values{"repeat": {"weekly on thu,mon"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	page = b.page("/tasks/" + ref)
	if row := aboutRow(t, page, "repeat"); !strings.Contains(row, "every week on mon,thu") {
		t.Fatalf("the task screen does not state the rule:\n%s", row)
	}
	if got := inputValue(t, page, "repeat"); got != "every week on mon,thu" {
		t.Fatalf("the Repeat field holds %q", got)
	}

	refused := save(url.Values{"repeat": {"every fortnight"}})
	body := readAll(t, refused)
	_ = refused.Body.Close()
	if refused.StatusCode == http.StatusSeeOther && !strings.Contains(refused.Header.Get("Location"), "fortnight") {
		t.Fatalf("an unparsable rule was accepted silently: %d %s", refused.StatusCode, refused.Header.Get("Location"))
	}
	if refused.StatusCode != http.StatusSeeOther && !strings.Contains(body, "fortnight") {
		t.Fatalf("the refusal does not name the rule:\n%s", tail(body))
	}
	if row := aboutRow(t, b.page("/tasks/"+ref), "repeat"); !strings.Contains(row, "every week on mon,thu") {
		t.Fatalf("a refused rule changed the task:\n%s", row)
	}

	withoutField := save(url.Values{})
	_ = withoutField.Body.Close()
	if row := aboutRow(t, b.page("/tasks/"+ref), "repeat"); !strings.Contains(row, "every week on mon,thu") {
		t.Fatalf("a save that did not name the rule cleared it:\n%s", row)
	}

	cleared := save(url.Values{"repeat": {""}})
	_ = cleared.Body.Close()
	if row := aboutRow(t, b.page("/tasks/"+ref), "repeat"); strings.Contains(row, "every") {
		t.Fatalf("emptying the field left the rule:\n%s", row)
	}
}

func TestTheAddFormTakesARepeatRule(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	form := formAt(t, b.page("/tasks"), "/tasks")
	if !strings.Contains(form, `name="repeat"`) {
		t.Fatalf("the add form offers no Repeat field:\n%s", form)
	}
	resp := b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"pay rent"}, "repeat": {"monthly"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	page, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{})
	if err != nil || len(page.Tasks) != 1 || page.Tasks[0].Recurrence != "every month" {
		t.Fatalf("the added task = %+v, %v; want it carrying every month", page.Tasks, err)
	}
}

func TestRowsAndCardsMarkARecurringTask(t *testing.T) {
	f := newFixture(t)
	refs := map[string]string{}
	for _, in := range []core.CreateTaskInput{
		{ProjectRef: "infra", Title: "rotate the keys", Recurrence: "every 3 months"},
		{ProjectRef: "infra", Title: "replace the racks"},
	} {
		task, err := f.svc.CreateTask(f.ctx(), in)
		if err != nil {
			t.Fatalf("creating %q: %v", in.Title, err)
		}
		refs[in.Title] = task.Ref
	}
	b := f.as("alice")
	list := b.page("/tasks")
	if row := dueRow(t, list, "rotate the keys"); !strings.Contains(row, `class="repeatmark"`) || !strings.Contains(row, "Repeats every 3 months") {
		t.Errorf("the recurring row carries no repeat marker naming its rule:\n%s", row)
	}
	if row := dueRow(t, list, "replace the racks"); strings.Contains(row, "repeatmark") {
		t.Errorf("a one-off row carries a repeat marker:\n%s", row)
	}

	board := b.page("/projects/infra")
	for _, c := range []struct {
		title  string
		marked bool
	}{{"rotate the keys", true}, {"replace the racks", false}} {
		card := boardCard(t, board, c.title)
		if strings.Contains(card, "repeatmark") != c.marked {
			t.Errorf("card %q marked = %v, want %v:\n%s", c.title, !c.marked, c.marked, card)
		}
	}

	b.setCookie(web.TaskViewCookie, web.TaskViewLanes)
	lanes := b.page("/tasks")
	for _, c := range []struct {
		title  string
		marked bool
	}{{"rotate the keys", true}, {"replace the racks", false}} {
		card := laneCard(t, lanes, refs[c.title])
		if strings.Contains(card, "repeatmark") != c.marked {
			t.Errorf("lane card %q marked = %v, want %v:\n%s", c.title, !c.marked, c.marked, card)
		}
	}
}

// boardCard returns the one card on a board that names a task.
func boardCard(t *testing.T, page, title string) string {
	t.Helper()
	for _, card := range strings.Split(page, `<article class="card`)[1:] {
		card, _, _ = strings.Cut(card, "</article>")
		if strings.Contains(card, ">"+title+"<") {
			return card
		}
	}
	t.Fatalf("no card names %q", title)
	return ""
}

func TestCompletingFromTheBrowserSpawnsTheNextOccurrence(t *testing.T) {
	f := newFixture(t)
	task, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "standup", Recurrence: "daily"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	b := f.as("alice")
	resp := b.post("/tasks/"+task.Ref+"/transition", url.Values{"route": {"doing" + core.RouteSep + "done"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	page := b.page("/tasks/" + task.Ref)
	next := between(t, aboutRow(t, page, "repeat"), `href="/tasks/`, `"`)
	if next == "" || next == task.Ref {
		t.Fatalf("the finished task links no successor:\n%s", aboutRow(t, page, "repeat"))
	}
	if !strings.Contains(b.page("/tasks/"+next), "standup") {
		t.Fatalf("the successor %s is not the same task again", next)
	}
}
