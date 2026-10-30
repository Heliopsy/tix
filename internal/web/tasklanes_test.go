// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/web"
)

// chooseLanes presses the view switch's Lanes position.
func (b *browser) chooseLanes(next string) {
	b.t.Helper()
	resp := b.post("/taskview", url.Values{"view": {web.TaskViewLanes}, "next": {next}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(b.t, resp, http.StatusSeeOther)
}

var lanePattern = regexp.MustCompile(`<section class="lane[^"]*" id="lane-[^"]*" data-lane="([^"]+)"`)

// laneKeys names the lanes the page draws, in order, read off the lane
// sections and nothing else.
func laneKeys(page string) []string {
	out := []string{}
	for _, m := range lanePattern.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

// laneBlock is one lane's section and nothing else, so what a lane holds is
// read from that lane rather than from anywhere on the page.
func laneBlock(t *testing.T, page, key string) string {
	t.Helper()
	start := strings.Index(page, `data-lane="`+key+`"`)
	if start < 0 {
		t.Fatalf("no lane for %s:\n%s", key, page)
	}
	end := strings.Index(page[start:], "</section>")
	if end < 0 {
		t.Fatalf("the lane for %s is not a closed section", key)
	}
	return page[start : start+end]
}

var laneTaskPattern = regexp.MustCompile(`data-lane-task="([^"]+)"`)

func laneRefs(block string) []string {
	out := []string{}
	for _, m := range laneTaskPattern.FindAllStringSubmatch(block, -1) {
		out = append(out, m[1])
	}
	return out
}

// laneCard is one lane card's article and nothing else.
func laneCard(t *testing.T, page, ref string) string {
	t.Helper()
	start := strings.Index(page, `data-lane-task="`+ref+`"`)
	if start < 0 {
		t.Fatalf("no lane card for %s:\n%s", ref, page)
	}
	open := strings.LastIndex(page[:start], "<article")
	end := strings.Index(page[start:], "</article>")
	if open < 0 || end < 0 {
		t.Fatalf("the lane card for %s is not a closed article", ref)
	}
	return page[open : start+end]
}

// tickRoute is the route a card's tick box submits, or "" when it has none.
var tickRoutePattern = regexp.MustCompile(`<input type="hidden" name="route" value="([^"]*)">`)

func tickRoute(card string) string {
	if !strings.Contains(card, `class="tick"`) {
		return ""
	}
	m := tickRoutePattern.FindStringSubmatch(card)
	if m == nil {
		return ""
	}
	return html.UnescapeString(m[1])
}

// The switch offers lanes beside the list and the board, on both screens that
// render it, and the choice is the cookie, so the server draws lanes on the
// next request and the live refresh with nothing else to read.
func TestTheViewSwitchOffersAndKeepsLanes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "in a lane")

	values := regexp.MustCompile(`name="view" value="([^"]+)"`)
	for _, screen := range []string{"/tasks", "/settings"} {
		var got []string
		for _, m := range values.FindAllStringSubmatch(viewSwitch(t, b.page(screen)), -1) {
			got = append(got, m[1])
		}
		if want := []string{"list", "board", "lanes"}; !slices.Equal(got, want) {
			t.Errorf("%s offers %v, want %v", screen, got, want)
		}
	}
	if keys := laneKeys(b.page("/tasks")); len(keys) != 0 {
		t.Fatalf("an untouched screen drew lanes: %v", keys)
	}

	b.chooseLanes("/tasks")
	if b.cookie(web.TaskViewCookie) != web.TaskViewLanes {
		t.Fatalf("the choice is not stored: %q", b.cookie(web.TaskViewCookie))
	}
	page := b.page("/tasks")
	if keys := laneKeys(page); !slices.Equal(keys, []string{"infra"}) {
		t.Fatalf("lanes = %v, want one lane for infra:\n%s", keys, page)
	}
	if !selected(viewSwitch(t, page), web.TaskViewLanes) {
		t.Error("the switch does not mark lanes as current")
	}
	if !selected(viewSwitch(t, b.page("/settings")), web.TaskViewLanes) {
		t.Error("settings does not show the lanes the task screen switched to")
	}
	if strings.Contains(page, `<ul class="tasklist"`) || len(columnStates(page)) != 0 {
		t.Error("the lanes view also drew the list or the board")
	}

	fragment := b.page(liveAttr(t, page, "data-live"))
	if !slices.Equal(laneRefs(laneBlock(t, fragment, "infra")), []string{ref}) {
		t.Errorf("a live refresh of the lanes did not draw the lane:\n%s", fragment)
	}

	b.setCookie(web.TaskViewCookie, "swimlanes")
	if keys := laneKeys(b.page("/tasks")); len(keys) != 0 {
		t.Errorf("an unrecognised stored view drew lanes: %v", keys)
	}
}

// One lane per project, holding exactly that project's tasks, in the order
// the screen lists projects; visibility applies; a project the filter names
// gets a lane even when empty and an unnamed empty project gets none;
// finished work stays in the lane, ticked.
func TestLanesGroupTasksByProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "ops")
	seedProject(t, f.store, f.tenantA.ID, "quiet")
	b := f.as("alice")
	infra := b.createTask("infra", "infra work")
	ops := b.createTask("ops", "ops work")
	finished := b.createTask("ops", "ops finished")
	done := b.post("/tasks/"+finished+"/complete", url.Values{})
	_ = done.Body.Close()
	b.chooseLanes("/tasks")

	page := b.page("/tasks")
	order := regexp.MustCompile(`<select name="project_ref" aria-label="Project">([\s\S]*?)</select>`).FindStringSubmatch(page)
	if order == nil {
		t.Fatal("no quick-add project menu to read the screen's project order from")
	}
	var want []string
	for _, m := range regexp.MustCompile(`<option value="([^"]+)"`).FindAllStringSubmatch(order[1], -1) {
		if m[1] != "quiet" {
			want = append(want, m[1])
		}
	}
	if got := laneKeys(page); !slices.Equal(got, want) {
		t.Errorf("lanes = %v, want %v: the screen's project order, and no lane for the empty quiet", got, want)
	}
	if got := laneRefs(laneBlock(t, page, "infra")); !slices.Equal(got, []string{infra}) {
		t.Errorf("infra's lane holds %v, want only %s", got, infra)
	}
	opsLane := laneRefs(laneBlock(t, page, "ops"))
	slices.Sort(opsLane)
	wantOps := []string{ops, finished}
	slices.Sort(wantOps)
	if !slices.Equal(opsLane, wantOps) {
		t.Errorf("ops's lane holds %v, want %v including the finished task", opsLane, wantOps)
	}
	card := laneCard(t, page, finished)
	if !strings.Contains(card, "is-finished") || !strings.Contains(card, `class="check is-done"`) || strings.Contains(card, "<form") {
		t.Errorf("a finished card is not drawn ticked and inert:\n%s", card)
	}

	named := b.page("/tasks?q=" + url.QueryEscape("project:quiet"))
	if got := laneKeys(named); !slices.Equal(got, []string{"quiet"}) {
		t.Errorf("a filter naming an empty project drew %v, want its empty lane", got)
	}
	if !strings.Contains(laneBlock(t, named, "quiet"), "No tasks.") {
		t.Error("the named empty lane does not say it is empty")
	}

	b.setCookie(web.HiddenProjectsCookie, "ops")
	away := b.page("/tasks")
	if slices.Contains(laneKeys(away), "ops") || strings.Contains(away, `data-lane-task="`+ops+`"`) {
		t.Error("a project put away still has a lane")
	}
}

// The lane is the project, so the status is on the card: the state's label,
// styled by its category.
func TestALaneCardShowsItsStatus(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	todo := b.createTask("infra", "waiting to start")
	doing := b.createTask("infra", "under way")
	moved := b.post("/tasks/"+doing+"/transition", url.Values{"to": {"doing"}})
	_ = moved.Body.Close()
	b.chooseLanes("/tasks")
	page := b.page("/tasks")

	tests := []struct{ ref, class, label string }{
		{todo, "cat-todo", "To do"},
		{doing, "cat-in-progress", "Doing"},
	}
	for _, tt := range tests {
		badge := regexp.MustCompile(`<span class="pill status lane-state ([^"]*)"[^>]*>([^<]*)</span>`).
			FindStringSubmatch(laneCard(t, page, tt.ref))
		if badge == nil {
			t.Errorf("the card for %s carries no status badge", tt.ref)
			continue
		}
		if badge[1] != tt.class || badge[2] != tt.label {
			t.Errorf("%s badge = %q %q, want %q %q", tt.ref, badge[1], badge[2], tt.class, tt.label)
		}
	}
}

// The tick box submits the one route into done to the transition route, as a
// plain form with CSRF, and posting it finishes the task and returns to the
// listing.
func TestTickingALaneCardCompletesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "tick me")
	b.chooseLanes("/tasks")
	card := laneCard(t, b.page("/tasks"), ref)

	if got := tickRoute(card); got != "doing>done" {
		t.Fatalf("the tick box submits %q, want doing>done:\n%s", got, card)
	}
	for _, want := range []string{`action="/tasks/` + ref + `/transition"`, `name="csrf_token"`, `name="next" value="/tasks"`, "Doing"} {
		if !strings.Contains(card, want) {
			t.Errorf("the tick form carries no %s:\n%s", want, card)
		}
	}

	resp := b.post("/tasks/"+ref+"/transition", url.Values{"route": {tickRoute(card)}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks") || strings.HasPrefix(loc, "/tasks/") {
		t.Errorf("ticking sent the reader to %q, not back to the listing", loc)
	}
	after := laneCard(t, b.page("/tasks"), ref)
	if !strings.Contains(after, "is-finished") || !strings.Contains(after, ">Done</span>") {
		t.Errorf("the ticked task is not finished:\n%s", after)
	}
}

// No box where ticking would be a guess, would need a comment, or would be
// refused: the card links to the task instead and says why.
func TestALaneCardWithholdsTheBoxWhenCompletingIsNotOneMove(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	twoDone := service.BuiltinWorkflow()
	twoDone.States = append(twoDone.States, core.State{Key: "shipped", Label: "Shipped", Category: core.CategoryDone, Terminal: true})
	twoDone.Transitions = append(twoDone.Transitions, core.Transition{From: "doing", To: "shipped"})
	seedWorkflowProject(t, f, "twodone", "twodone", "Two done", twoDone)
	comment := service.BuiltinWorkflow()
	for i, tr := range comment.Transitions {
		if tr.To == "done" {
			comment.Transitions[i].RequiresComment = true
		}
	}
	seedWorkflowProject(t, f, "noted", "noted", "Noted", comment)

	b := f.as("alice")
	ambiguous := b.createTask("twodone", "two ways to finish")
	noted := b.createTask("noted", "needs a word")
	plain := b.createTask("infra", "plain")
	b.chooseLanes("/tasks")
	page := b.page("/tasks")

	viewer := f.as("viewer")
	viewer.chooseLanes("/tasks")
	viewerPage := viewer.page("/tasks")

	tests := []struct {
		name, page, ref, reason string
	}{
		{"two done states", page, ambiguous, "more than one way"},
		{"a comment is required", page, noted, "needs a comment"},
		{"the reader cannot move tasks", viewerPage, plain, "cannot move tasks"},
	}
	for _, tt := range tests {
		card := laneCard(t, tt.page, tt.ref)
		if strings.Contains(card, "<form") || tickRoute(card) != "" {
			t.Errorf("%s: the card offers a tick box:\n%s", tt.name, card)
		}
		if !strings.Contains(card, `class="check is-locked" href="/tasks/`+tt.ref) || !strings.Contains(card, tt.reason) {
			t.Errorf("%s: the card does not link to the task saying %q:\n%s", tt.name, tt.reason, card)
		}
	}
	if tickRoute(laneCard(t, page, plain)) == "" {
		t.Error("an ordinary task has no tick box for a reader who may finish it")
	}
}

// Show completed is a script over a class, and the markup it needs is there:
// the toggle, hidden until the script reveals it, and the finished class on
// the cards it hides. The script and the stylesheet rule are what the jstest
// suite and this check pin.
func TestShowCompletedHasWhatItNeeds(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "anything")
	b.chooseLanes("/tasks")
	page := b.page("/tasks")

	toggle := regexp.MustCompile(`<button[^>]*data-lanes-completed[^>]*>`).FindString(page)
	if toggle == "" || !strings.Contains(toggle, " hidden") || !strings.Contains(toggle, `aria-pressed="true"`) {
		t.Errorf("the Show completed toggle is missing, or shown without scripting:\n%s", toggle)
	}
	head, _, found := strings.Cut(page, "</head>")
	if !found {
		t.Fatal("the page has no head")
	}
	if !strings.Contains(head, `<script src="/assets/lanes.js"></script>`) {
		t.Error("lanes.js is not loaded undeferred from the head, so a hidden card would flash in")
	}
	css := body(t, b.get("/assets/app.css"))
	if !strings.Contains(css, ".lanes-hide-finished .lane .lane-card.is-finished { display: none; }") {
		t.Error("the stylesheet has no rule hiding finished lane cards under the root class")
	}
	js := body(t, b.get("/assets/lanes.js"))
	if !strings.Contains(js, `"lanes-hide-finished"`) || !strings.Contains(js, `"tix.lanes.showCompleted"`) {
		t.Error("the served lanes.js does not set the class the stylesheet reads")
	}
}
