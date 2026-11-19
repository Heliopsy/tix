// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var menuItemPattern = regexp.MustCompile(`data-menu-item="([a-z-]+)"`)

// menuItems are the items one task menu draws, in order, read from that
// popover alone.
func menuItems(t *testing.T, page, id string) []string {
	t.Helper()
	menu := popover(t, page, id)
	if menu == "" {
		t.Fatalf("the page has no task menu %s", id)
	}
	out := []string{}
	for _, m := range menuItemPattern.FindAllStringSubmatch(menu, -1) {
		out = append(out, m[1])
	}
	return out
}

// without is items less the ones a view adds or leaves out on purpose.
func without(items []string, drop ...string) []string {
	return slices.DeleteFunc(slices.Clone(items), func(s string) bool { return slices.Contains(drop, s) })
}

// The three dots draw one menu wherever a task is shown: the same items in
// the same order on a lane card, a board card, a list row and the task
// screen, for each level of authority. Open the task is left off the task
// screen, which is the task, and Select is the list's alone.
func TestEveryViewDrawsTheSameTaskMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "one menu everywhere")
	member := "member:" + f.actorFor("alice").ID

	for _, tt := range []struct {
		name  string
		actor string
		want  []string
	}{
		{"full", "alice", []string{"open", "copy-link", "copy-ref", "complete", "move", "priority", "assignee", "due", "tags",
			"rename", "snooze", "star", "copy-to", "move-to", "save-template", "delete"}},
		{"write without delete", member, []string{"open", "copy-link", "copy-ref", "complete", "move", "priority", "assignee",
			"due", "tags", "rename", "snooze", "star", "copy-to", "move-to", "save-template"}},
		{"read only", "viewer", []string{"open", "copy-link", "copy-ref", "star"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := f.as(tt.actor)
			list := menuItems(t, b.page("/tasks"), "rmore-"+ref)
			board := menuItems(t, b.page("/projects/infra?view=board"), "bmore-"+ref)
			screen := menuItems(t, b.page("/tasks/"+ref), "taskmenu-"+ref)
			b.chooseLanes("/tasks")
			lane := menuItems(t, b.page("/tasks"), "lmore-"+ref)

			if !slices.Equal(lane, tt.want) {
				t.Errorf("the lane card's menu is %v, want %v", lane, tt.want)
			}
			if !slices.Equal(board, tt.want) {
				t.Errorf("the board card's menu is %v, want %v", board, tt.want)
			}
			if got := without(list, "select"); !slices.Equal(got, tt.want) {
				t.Errorf("the list row's menu is %v, want %v and Select", list, tt.want)
			}
			if at := slices.Index(list, "select"); at < 0 || at < len(list)-2 {
				t.Errorf("the list row's Select is not just before Delete, or missing: %v", list)
			}
			if got, want := screen, without(tt.want, "open"); !slices.Equal(got, want) {
				t.Errorf("the task screen's menu is %v, want %v", got, want)
			}
		})
	}
}

// A list row's Delete is the lane card's: with the trash on it moves the task
// to the trash and the toast offers Undo; with the trash off it asks first,
// on the confirmation every other delete uses.
func TestAListRowDeletesThroughTheOneDeletePath(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "deleted from its row")

	action := between(t, popover(t, b.page("/tasks"), "rmore-"+ref), `<form method="post" action="/tasks/`+ref+`/delete`, `"`)
	action = "/tasks/" + ref + "/delete" + strings.ReplaceAll(action, "&amp;", "&")
	page := followed(t, b, b.post(action, url.Values{}))
	if m := undoForm.FindStringSubmatch(page); m == nil || m[1] != "/tasks/"+ref+"/restore" {
		t.Errorf("deleting from the row offers no Undo: %v", m)
	}
	if strings.Contains(page, `id="t-`+ref+`"`) {
		t.Error("the deleted task is still in the listing")
	}

	turnTrashOff(t, b)
	other := b.createTask("infra", "deleted for good from its row")
	action = between(t, popover(t, b.page("/tasks"), "rmore-"+other), `<form method="post" action="/tasks/`+other+`/delete`, `"`)
	action = "/tasks/" + other + "/delete" + strings.ReplaceAll(action, "&amp;", "&")
	asked := b.post(action, url.Values{})
	wantStatus(t, asked, http.StatusOK)
	if form := confirmForm(t, body(t, asked)); !strings.Contains(form, `name="confirm" value="1"`) {
		t.Errorf("with the trash off the row's delete does not ask first:\n%s", form)
	}
	if !strings.Contains(b.page("/tasks"), `id="t-`+other+`"`) {
		t.Error("the delete went through before it was confirmed")
	}
}

// Rename in a task menu is a link that, without scripting, opens the task
// screen with its title field showing; the screen's own menu leads there too.
func TestRenameWithoutScriptingOpensTheTitleField(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "renamed from a menu")

	if strings.Contains(b.page("/tasks/"+ref), `<details class="titleedit" open>`) {
		t.Fatal("the title field is open before anybody asked for it")
	}
	if !strings.Contains(popover(t, b.page("/tasks"), "rmore-"+ref), `href="/tasks/`+ref+`?next=%2Ftasks&amp;rename=1"`) {
		t.Error("the row's Rename does not lead to the task screen's title field")
	}
	if !strings.Contains(b.page("/tasks/"+ref+"?rename=1"), `<details class="titleedit" open>`) {
		t.Error("the task screen asked for at Rename does not open its title field")
	}
}
