// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

func TestATrashRowDeletesPermanentlyOnlyFromItsDialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "thrown out")
	second := b.createTask("infra", "also thrown out")
	deleteTask(t, b, first)
	deleteTask(t, b, second)

	page := b.page(trashPath)
	id := "task-purge-" + first
	row := rowOf(t, page, first)
	if !strings.Contains(row, `data-dlg-open="`+id+`"`) || !strings.Contains(row, `class="btn danger small purgeopen"`) {
		t.Fatalf("the trash row offers no red Delete permanently opener: %s", row)
	}
	if strings.Contains(row, "/tasks/"+first+"/purge") {
		t.Error("the row itself carries the purge form; it belongs only in the dialog")
	}
	dialog := dialogBlock(page, id)
	if dialog == "" {
		t.Fatalf("no dialog %s on the trash", id)
	}
	form := formAt(t, dialog, "/tasks/"+first+"/purge")
	if !strings.Contains(form, `<button class="danger" type="submit">Delete permanently</button>`) {
		t.Errorf("the dialog's form has no red Delete permanently button: %s", form)
	}
	next, _ := url.QueryUnescape(between(t, form, `name="next" value="`, `"`))
	if next != trashPath {
		t.Errorf("the purge returns to %q, want the trash", next)
	}

	resp := b.post("/tasks/"+first+"/purge", url.Values{"next": {next}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	where, _ := url.Parse(resp.Header.Get("Location"))
	if where.Path != web.RouteTasks || where.Query().Get("q") != "is:trashed" ||
		where.Query().Get("flash") != "task deleted permanently" {
		t.Errorf("purging went to %s, want the trash with a task deleted permanently toast", where)
	}
	after := b.page(trashPath)
	if strings.Contains(after, `id="t-`+first+`"`) {
		t.Error("the purged task is still in the trash")
	}
	if !strings.Contains(after, `id="t-`+second+`"`) {
		t.Error("the other task fell out of the trash")
	}
	if strings.Contains(b.page("/tasks?q=is:deleted"), `id="t-`+first+`"`) {
		t.Error("the purged task can still be listed with deleted tasks included")
	}
}
