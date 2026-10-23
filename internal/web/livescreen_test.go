// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// A change made anywhere -- the terminal, another tab, an agent claiming a
// task -- reached only the activity feed: it was the one screen carrying a
// live hook at all, so a task moved from todo to done over SSH left an open
// task list, board and task screen showing todo until the reader reloaded.
// These guards hold the three screens to the same arrangement the feed uses,
// and hold the refresh to the state the reader is actually in.

// liveAttr reads one attribute of the binding element a screen renders. It
// reads the element itself rather than the page, so an attribute that happens
// to appear somewhere else cannot stand in for one that is missing.
func liveAttr(t *testing.T, page, name string) string {
	t.Helper()
	el := between(t, page, `<div id="live-screen"`, `>`)
	if el == "" {
		t.Fatal("the screen renders no live binding, so nothing on it ever updates")
	}
	// A browser hands the script the attribute's value, not its markup, so
	// the escaping the page carries is undone here too. A test that fetched
	// the raw attribute asked for "limit" under the name "amp;limit" and got
	// a page of a different size back.
	return strings.ReplaceAll(between(t, el+">", name+`="`, `"`), "&amp;", "&")
}

// liveFilterOf reads the subscription filter a screen asks the event stream
// for. The attribute is HTML-escaped in the page and unescaped by the
// browser, so the quotes are restored before it is parsed.
func liveFilterOf(t *testing.T, page string) core.EventFilter {
	t.Helper()
	raw := strings.ReplaceAll(liveAttr(t, page, "data-live-filter"), "&#34;", `"`)
	var filter core.EventFilter
	if err := json.Unmarshal([]byte(raw), &filter); err != nil {
		t.Fatalf("the subscription filter %q is not valid json: %v", raw, err)
	}
	return filter
}

func TestEveryTaskScreenCarriesALiveBinding(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "watch me")

	for name, path := range map[string]string{
		"the task list":    "/tasks",
		"a project board":  "/projects/infra",
		"one task":         "/tasks/" + ref,
		"a filtered list":  "/tasks?q=" + url.QueryEscape("project:infra"),
		"the task list, b": "/tasks?sort=updated_at",
	} {
		page := b.page(path)
		if href := liveAttr(t, page, "data-live"); href == "" {
			t.Errorf("%s names no fragment to refresh from", name)
		}
		if regions := liveAttr(t, page, "data-live-regions"); regions == "" {
			t.Errorf("%s names no region a refresh replaces", name)
		}
		if events := liveAttr(t, page, "data-events"); events != "/api/v1/events" {
			t.Errorf("%s subscribes at %q, want the configured event route", name, events)
		}
	}
}

func TestTheTaskListFragmentKeepsTheFilterAndTheSort(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	kept := b.createTask("infra", "tagged work")
	dropped := b.createTask("infra", "untagged work")
	tagged := b.post("/tasks/"+kept+"/tags", url.Values{"tag": {"ops"}})
	_ = tagged.Body.Close()

	page := b.page("/tasks?q=" + url.QueryEscape("tag:ops") + "&sort=updated_at")
	href := liveAttr(t, page, "data-live")
	if !strings.HasPrefix(href, web.RouteTasksLive+"?") {
		t.Fatalf("the refresh points at %q, not at the list fragment", href)
	}
	if !strings.Contains(href, "q=tag%3Aops") || !strings.Contains(href, "sort=updated_at") {
		t.Fatalf("the refresh would drop the reader's filter or sort: %q", href)
	}

	fragment := b.page(href)
	if !strings.Contains(fragment, kept) {
		t.Error("the refreshed listing lost the task the filter selects")
	}
	if strings.Contains(fragment, dropped) {
		t.Error("the refreshed listing widened back out past the reader's filter")
	}
}

func TestTheTaskListFragmentKeepsThePageTheReaderIsOn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, title := range []string{"first row", "second row", "third row"} {
		b.createTask("infra", title)
	}

	first := b.page("/tasks?limit=1")
	next := strings.ReplaceAll(between(t, first, `class="pager-step pager-next" href="`, `"`), "&amp;", "&")
	if next == "" {
		t.Fatal("the list does not page, so there is no cursor to preserve")
	}
	second := b.page(next)
	href := liveAttr(t, second, "data-live")
	if !strings.Contains(href, "cursor=") || !strings.Contains(href, "limit=1") {
		t.Fatalf("a refresh from page two would jump back to page one: %q", href)
	}
	if !sameRows(b.page(href), second) {
		t.Error("the refreshed listing is not the page of rows the reader is on")
	}
}

// sameRows reports whether two renderings carry the same task rows, which is
// what "the same page of the listing" means here.
func sameRows(fragment, page string) bool {
	return strings.Join(rowIDs(fragment), ",") == strings.Join(rowIDs(page), ",")
}

func rowIDs(html string) []string {
	var out []string
	for _, part := range strings.Split(html, `<li id="t-`)[1:] {
		if id, _, ok := strings.Cut(part, `"`); ok {
			out = append(out, id)
		}
	}
	return out
}

func TestTheTaskListFragmentKeepsTheBoardSwitchAndTheHiddenProjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "on the board")

	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)
	fragment := b.page(liveAttr(t, b.page("/tasks"), "data-live"))
	if !strings.Contains(fragment, `class="board`) {
		t.Error("a reader on the board was refreshed with the list instead")
	}
	if !strings.Contains(fragment, ref) {
		t.Error("the refreshed board lost the task that was on it")
	}

	b.setCookie(web.HiddenProjectsCookie, "infra")
	hidden := b.page(liveAttr(t, b.page("/tasks"), "data-live"))
	if strings.Contains(hidden, ref) {
		t.Error("the refresh brought back a project the reader had put away")
	}
}

func TestTheTaskListFragmentIsOnlyTheRegionsItReplaces(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one row")

	fragment := b.page(web.RouteTasksLive)
	for _, region := range []string{`id="tasks-summary"`, `id="tasks-listing"`} {
		if !strings.Contains(fragment, region) {
			t.Errorf("the fragment carries no %s, so a refresh would replace nothing", region)
		}
	}
	if !strings.Contains(fragment, ref) {
		t.Error("the fragment carries no rows")
	}
	// The quick-add box and the filter bar are what the reader types into. A
	// fragment that carried them would replace them, which is the refresh
	// taking the screen away from the person using it.
	for _, kept := range []string{`class="quickadd"`, `class="filterbar"`, "<nav", "<html"} {
		if strings.Contains(fragment, kept) {
			t.Errorf("the fragment carries %s, which a refresh would swap under the reader", kept)
		}
	}
}

func TestTheBoardFragmentIsOnlyTheColumns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "a card")

	page := b.page("/projects/infra")
	href := liveAttr(t, page, "data-live")
	if href != "/projects/infra/live" {
		t.Fatalf("the board refreshes from %q", href)
	}
	fragment := b.page(href)
	if !strings.Contains(fragment, `id="project-board"`) || !strings.Contains(fragment, ref) {
		t.Fatal("the board fragment does not carry the columns")
	}
	for _, kept := range []string{"Project settings", "Danger zone"} {
		if strings.Contains(fragment, kept) {
			t.Errorf("the board fragment carries %q, which is a form the reader may be in", kept)
		}
	}
}

func TestTheTaskFragmentRefreshesTheRecordAndNeverTheForms(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one task")
	commented := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"said something"}})
	_ = commented.Body.Close()

	page := b.page("/tasks/" + ref)
	href := liveAttr(t, page, "data-live")
	if href != "/tasks/"+ref+"/live" {
		t.Fatalf("the task screen refreshes from %q", href)
	}
	fragment := b.page(href)
	for _, region := range []string{`id="task-pills"`, `id="task-comments"`, `id="task-history"`} {
		if !strings.Contains(fragment, region) {
			t.Errorf("the fragment carries no %s", region)
		}
	}
	if !strings.Contains(fragment, "said something") {
		t.Error("the fragment does not carry the comments")
	}
	// The forms the reader works in -- the detail editor, the move control,
	// the comment box, the danger zone -- are outside every refreshed region.
	// What is inside one is a comment's own Edit disclosure, which the
	// busy rule holds a refresh back for while it is being typed in.
	for _, kept := range []string{
		`<input type="hidden" name="version"`,
		`id="comment-body"`,
		`action="/tasks/` + ref + `/transition"`,
		`action="/tasks/` + ref + `/delete"`,
	} {
		if strings.Contains(fragment, kept) {
			t.Errorf("the fragment carries %s, which a refresh would swap under the reader", kept)
		}
	}
}

func TestTheTaskFragmentShowsAStatusChangedElsewhere(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "moved behind the reader's back")

	before := b.page("/tasks/" + ref + "/live")
	moved := b.post("/tasks/"+ref+"/transition", url.Values{"route": {"doing"}})
	_ = moved.Body.Close()
	after := b.page("/tasks/" + ref + "/live")

	if !strings.Contains(before, `<span class="pill status todo">todo</span>`) {
		t.Fatalf("the fragment did not start at todo: %s", before)
	}
	if !strings.Contains(after, `<span class="pill status doing">doing</span>`) {
		t.Error("a transition made elsewhere does not reach the open task screen")
	}
}

func TestALiveSubscriptionNamesOnlyWhatTheScreenCanShow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "subscribed to")

	list := liveFilterOf(t, b.page("/tasks"))
	if len(list.Types) == 0 {
		t.Error("the task list subscribes to every tenant event, including ones it cannot draw")
	}
	for _, typ := range list.Types {
		if typ == "*" {
			t.Error("the task list subscribes to the whole stream")
		}
	}

	board := liveFilterOf(t, b.page("/projects/infra"))
	if len(board.ProjectIDs) != 1 || board.ProjectIDs[0] != f.project.ID {
		t.Errorf("the board subscribes to %v, not to the project it draws", board.ProjectIDs)
	}

	page := b.page("/tasks/" + ref)
	detail := liveFilterOf(t, page)
	if len(detail.ProjectIDs) != 1 || detail.ProjectIDs[0] != f.project.ID {
		t.Errorf("the task screen subscribes to %v, not to its own project", detail.ProjectIDs)
	}
	if !hasType(detail.Types, core.EventCommentAdded) {
		t.Error("the task screen draws comments but does not subscribe to them")
	}
	// The stream has no filter for one record, so the screen says which
	// record it is about and decides the rest itself (decide.js wantsEvent).
	if subject := liveAttr(t, page, "data-live-subject"); subject == "" {
		t.Error("the task screen names no subject, so a change to any task in the project rebuilds it")
	}
}

func hasType(types []core.EventType, want core.EventType) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

func TestALiveFragmentRendersThroughTheScreensOwnBlocks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one row")

	page := b.page("/tasks")
	fragment := b.page(web.RouteTasksLive)
	row := between(t, page, `<li id="t-`+ref+`"`, "</li>")
	if row == "" {
		t.Fatal("the listing has no row to compare")
	}
	if !strings.Contains(fragment, row) {
		t.Error("a refreshed row is not the row the page renders, so one of them is a second copy of the rule")
	}
}

func TestEveryLiveScreenCanSayItIsNotLive(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "watched")

	// The feed included: a screen that has quietly stopped updating looks
	// exactly like one that is up to date, which is the defect this whole
	// mechanism exists to remove.
	for _, path := range []string{"/tasks", "/projects/infra", "/tasks/" + ref, "/activity"} {
		page := b.page(path)
		if !strings.Contains(page, `id="live-stale"`) {
			t.Errorf("%s can never say that it has stopped receiving events", path)
		}
		if !strings.Contains(page, `class="livestale"`) {
			t.Errorf("%s renders the notice unstyled", path)
		}
	}
}

func TestOneTaskRedrawsSoonerThanAWholeListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one task")

	// The first event is drawn at once either way (decide.js coalescer), so
	// this is only how long a screen then waits for the rest of a burst. A
	// listing waits longer because a burst there is routine and a re-render
	// is a page of rows; one task's screen waits half as long, because there
	// the wait is the whole of the delay the reader perceives.
	list := liveAttr(t, b.page("/tasks"), "data-live-window")
	detail := liveAttr(t, b.page("/tasks/"+ref), "data-live-window")
	if list != "300" || detail != "150" {
		t.Errorf("coalescing windows are list %q, task %q; want 300 and 150", list, detail)
	}
}
