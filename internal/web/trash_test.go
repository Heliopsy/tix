// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// trashPath is where the sidebar's Trash entry leads.
const trashPath = "/tasks?q=is:trashed"

// rowOf is one row of a listing, cut at its own closing tag, so an
// assertion about a row reads that row rather than the page around it.
func rowOf(t *testing.T, page, ref string) string {
	t.Helper()
	return between(t, page, `<li id="t-`+ref+`"`, "</li>")
}

// navLink is the one sidebar anchor whose label is name.
func navLink(t *testing.T, page, name string) string {
	t.Helper()
	for _, link := range strings.Split(nav(t, page), "<a ")[1:] {
		if strings.Contains(link, "<span>"+name+"</span>") {
			return link
		}
	}
	t.Fatalf("the sidebar has no %s entry:\n%s", name, nav(t, page))
	return ""
}

// deleteTask soft deletes a task from the browser.
func deleteTask(t *testing.T, b *browser, ref string) {
	t.Helper()
	resp := b.post("/tasks/"+ref+"/delete", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
}

func TestTheSidebarLeadsToTheTrash(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	tasks := b.page("/tasks")
	trash := navLink(t, tasks, "Trash")
	if !strings.Contains(trash, `href="`+trashPath+`"`) {
		t.Fatalf("Trash does not lead to %s: %s", trashPath, trash)
	}
	if strings.Contains(trash, "aria-current") {
		t.Error("Trash is marked current on the task list")
	}
	if !strings.Contains(navLink(t, tasks, "Tasks"), "aria-current") {
		t.Error("Tasks is not marked current on the task list")
	}

	inTrash := b.page(trashPath)
	if !strings.Contains(navLink(t, inTrash, "Trash"), `aria-current="page"`) {
		t.Error("Trash is not marked current on the trash")
	}
	if strings.Contains(navLink(t, inTrash, "Tasks"), "aria-current") {
		t.Error("Tasks is marked current on the trash as well")
	}
}

func TestTheTrashListsDeletedTasksAndRestoresThem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	live := b.createTask("infra", "still here")
	first := b.createTask("infra", "thrown out")
	second := b.createTask("infra", "also thrown out")
	deleteTask(t, b, first)
	deleteTask(t, b, second)

	page := b.page(trashPath)
	if h1 := between(t, page, "<h1>", "</h1>"); h1 != "Trash" {
		t.Errorf("the trash is headed %q", h1)
	}
	if note := between(t, page, `<p class="lede summary trashnote">`, "</p>"); !strings.Contains(note, "2 deleted tasks") {
		t.Errorf("the trash does not say its rows are deleted: %q", note)
	}
	if strings.Contains(page, `id="t-`+live+`"`) {
		t.Error("a live task is listed in the trash")
	}
	if strings.Contains(page, `class="quickadd"`) {
		t.Error("the trash offers the quick-add row")
	}
	nexts := map[string]string{}
	for _, ref := range []string{first, second} {
		row := rowOf(t, page, ref)
		if !strings.Contains(row, "is-deleted") || !strings.Contains(row, `class="deletedmark"`) {
			t.Errorf("the row for %s is not marked deleted: %s", ref, row)
		}
		restore := formAt(t, row, "/tasks/"+ref+"/restore")
		next, _ := url.QueryUnescape(between(t, restore, `name="next" value="`, `"`))
		if next != trashPath {
			t.Errorf("Restore on %s returns to %q, want the trash", ref, next)
		}
		nexts[ref] = next
	}

	resp := b.post("/tasks/"+first+"/restore", url.Values{"next": {nexts[first]}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	where, _ := url.Parse(resp.Header.Get("Location"))
	if where.Path != web.RouteTasks || where.Query().Get("q") != "is:trashed" || where.Query().Get("flash") != "task restored" {
		t.Errorf("restoring went to %s, want the trash with a task restored toast", where)
	}

	after := b.page(trashPath)
	if strings.Contains(after, `id="t-`+first+`"`) {
		t.Error("the restored task is still in the trash")
	}
	if !strings.Contains(after, `id="t-`+second+`"`) {
		t.Error("the task left deleted fell out of the trash")
	}
	if !strings.Contains(b.page("/tasks"), `id="t-`+first+`"`) {
		t.Error("the restored task is not back on the task list")
	}
}

func TestADeletedRowOffersOnlyRestore(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	live := b.createTask("infra", "live")
	gone := b.createTask("infra", "gone")
	deleteTask(t, b, gone)

	page := b.page("/tasks?q=is:deleted")
	row := rowOf(t, page, gone)
	for _, refused := range []string{
		`action="/tasks/` + gone + `/complete"`,
		`popovertarget="flow-` + gone + `"`,
		`popovertarget="prio-` + gone + `"`,
	} {
		if strings.Contains(row, refused) {
			t.Errorf("the deleted row offers %s", refused)
		}
	}
	if !strings.Contains(row, `action="/tasks/`+gone+`/restore"`) {
		t.Error("the deleted row on an is:deleted listing offers no Restore")
	}
	liveRow := rowOf(t, page, live)
	if !strings.Contains(liveRow, `action="/tasks/`+live+`/complete"`) {
		t.Fatalf("the live row has no tick either, so this test reads nothing: %s", liveRow)
	}
	if strings.Contains(liveRow, "/restore") || strings.Contains(liveRow, "deletedmark") {
		t.Error("a live row offers Restore or is marked deleted")
	}
}

func TestTheTrashIsTheListWhateverTheView(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	gone := b.createTask("infra", "gone")
	deleteTask(t, b, gone)

	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	if !strings.Contains(b.page("/tasks"), `class="board`) {
		t.Fatal("the board preference draws no board on the task list, so this test reads nothing")
	}
	page := b.page(trashPath)
	if strings.Contains(page, `class="board`) {
		t.Error("the trash is drawn as the board")
	}
	if !strings.Contains(rowOf(t, page, gone), `/restore"`) {
		t.Error("the trash on a board reader's screen has no row with Restore")
	}
}

func TestTheTrashKeepsFinishedTasks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	done := b.createTask("infra", "finished then deleted")
	for _, route := range []string{"doing", "done"} {
		resp := b.post("/tasks/"+done+"/transition", url.Values{"route": {route}})
		_ = resp.Body.Close()
	}
	deleteTask(t, b, done)
	page := b.page(trashPath)
	if strings.Contains(page, "open work only") {
		t.Error("the trash narrows to open work")
	}
	if !strings.Contains(page, `id="t-`+done+`"`) {
		t.Error("a finished task that was deleted is missing from the trash")
	}
}
