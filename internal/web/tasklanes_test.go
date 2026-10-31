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
	if !strings.Contains(card, "is-finished") || !strings.Contains(card, `class="check is-done"`) {
		t.Errorf("a finished card is not drawn ticked:\n%s", card)
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

// tickPanel is the popover the tick opens when it is not a box, and nothing
// else of the card.
func tickPanel(t *testing.T, card, ref string) string {
	t.Helper()
	start := strings.Index(card, `<div id="ltick-`+ref+`"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(card[start:], `popovertargetaction="hide"`)
	if end < 0 {
		t.Fatalf("the tick panel for %s is not closed", ref)
	}
	return card[start : start+end]
}

// popover is one of a card's menus by its id, and nothing else of the card.
func popover(t *testing.T, card, id string) string {
	t.Helper()
	start := strings.Index(card, `<div id="`+id+`"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(card[start:], "</div>")
	if end < 0 {
		t.Fatalf("the popover %s is not closed", id)
	}
	return card[start : start+end]
}

var routeButtonPattern = regexp.MustCompile(`<button type="submit" name="route" value="([^"]*)"`)

// routeButtons are the routes a menu offers, in order.
func routeButtons(menu string) []string {
	out := []string{}
	for _, m := range routeButtonPattern.FindAllStringSubmatch(menu, -1) {
		out = append(out, html.UnescapeString(m[1]))
	}
	return out
}

// A finished card's tick reopens the task along the workflow's nearest route
// back to an open state.
func TestUntickingALaneCardReopensIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "untick me")
	done := b.post("/tasks/"+ref+"/complete", url.Values{})
	_ = done.Body.Close()
	b.chooseLanes("/tasks")
	card := laneCard(t, b.page("/tasks"), ref)

	if got := tickRoute(card); got != "todo" {
		t.Fatalf("the finished card's tick submits %q, want todo:\n%s", got, card)
	}
	if !strings.Contains(card, `class="check is-done"`) || !strings.Contains(card, "Reopen "+ref) {
		t.Errorf("the reopening box is not drawn ticked and named as reopening:\n%s", card)
	}

	resp := b.post("/tasks/"+ref+"/transition", url.Values{"route": {tickRoute(card)}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	after := laneCard(t, b.page("/tasks"), ref)
	if strings.Contains(after, "is-finished") || !strings.Contains(after, ">To do</span>") {
		t.Errorf("the unticked task is not open again:\n%s", after)
	}
	if tickRoute(after) != "doing>done" {
		t.Errorf("the reopened card does not offer completing again: %q", tickRoute(after))
	}
}

// Where the tick is a choice it is a menu of the routes; where it cannot
// apply any it opens a panel that says why in words and links to the task.
func TestALaneCardOffersAChoiceOrSaysWhy(t *testing.T) {
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
	oneWay := service.BuiltinWorkflow()
	oneWay.Transitions = slices.DeleteFunc(oneWay.Transitions, func(tr core.Transition) bool { return tr.From == "done" })
	seedWorkflowProject(t, f, "oneway", "oneway", "One way", oneWay)

	b := f.as("alice")
	ambiguous := b.createTask("twodone", "two ways to finish")
	moved := b.post("/tasks/"+ambiguous+"/transition", url.Values{"to": {"doing"}})
	_ = moved.Body.Close()
	noted := b.createTask("noted", "needs a word")
	final := b.createTask("oneway", "no way back")
	closed := b.post("/tasks/"+final+"/complete", url.Values{})
	_ = closed.Body.Close()
	plain := b.createTask("infra", "plain")
	b.chooseLanes("/tasks")
	page := b.page("/tasks")

	viewer := f.as("viewer")
	viewer.chooseLanes("/tasks")
	viewerPage := viewer.page("/tasks")

	choice := tickPanel(t, laneCard(t, page, ambiguous), ambiguous)
	if got := routeButtons(choice); !slices.Equal(got, []string{"done", "shipped"}) {
		t.Errorf("the choice offers %v, want done and shipped:\n%s", got, choice)
	}
	if tickRoute(laneCard(t, page, ambiguous)) != "" {
		t.Error("a choice was drawn as a box that applies one route by itself")
	}

	tests := []struct {
		name, page, ref, reason string
	}{
		{"a comment is required", page, noted, "finishing it needs a comment"},
		{"no way back", page, final, "no route leads back to an open state"},
		{"the reader cannot move tasks", viewerPage, plain, "you cannot move tasks"},
	}
	for _, tt := range tests {
		card := laneCard(t, tt.page, tt.ref)
		panel := tickPanel(t, card, tt.ref)
		if tickRoute(card) != "" || len(routeButtons(panel)) != 0 {
			t.Errorf("%s: the tick offers a route:\n%s", tt.name, card)
		}
		if !strings.Contains(card, `is-locked" popovertarget="ltick-`+tt.ref+`"`) {
			t.Errorf("%s: the tick does not open its panel:\n%s", tt.name, card)
		}
		if !strings.Contains(panel, `<p class="flowwhy">Not from the lane: `+tt.reason+`.</p>`) ||
			!strings.Contains(panel, `href="/tasks/`+tt.ref+`?next=`) {
			t.Errorf("%s: the panel does not say %q and link to the task:\n%s", tt.name, tt.reason, panel)
		}
	}
	if !strings.Contains(laneCard(t, page, final), `class="check is-done is-locked"`) {
		t.Error("a finished card that cannot reopen is not drawn ticked")
	}
	if tickRoute(laneCard(t, page, plain)) == "" {
		t.Error("an ordinary task has no tick box for a reader who may finish it")
	}
}

// The badge opens the workflow's moves and the priority opens the
// priorities, each a form posting to the existing route, and choosing one
// changes the task. A reader who may not change them sees labels.
func TestALaneCardsStatusAndPriorityAreMenus(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	blockedNeedsWord := service.BuiltinWorkflow()
	for i, tr := range blockedNeedsWord.Transitions {
		if tr.To == "blocked" {
			blockedNeedsWord.Transitions[i].RequiresComment = true
		}
	}
	seedWorkflowProject(t, f, "careful", "careful", "Careful", blockedNeedsWord)
	b := f.as("alice")
	ref := b.createTask("infra", "menus")
	careful := b.createTask("careful", "no quick block")
	b.chooseLanes("/tasks")
	page := b.page("/tasks")
	card := laneCard(t, page, ref)

	status := popover(t, card, "lflow-"+ref)
	if got := routeButtons(status); !slices.Equal(got, []string{"doing", "blocked", "cancelled", "doing>done"}) {
		t.Errorf("the status menu offers %v:\n%s", got, status)
	}
	if !strings.Contains(card, `class="statuspill lanepill" popovertarget="lflow-`+ref+`"`) {
		t.Errorf("the badge does not open the status menu:\n%s", card)
	}
	prio := popover(t, card, "lprio-"+ref)
	for _, want := range []string{`action="/tasks/` + ref + `/priority"`, `name="csrf_token"`, `name="next" value="/tasks"`, `name="priority" value="2"`} {
		if !strings.Contains(prio, want) {
			t.Errorf("the priority menu carries no %s:\n%s", want, prio)
		}
	}
	if !strings.Contains(status, `action="/tasks/`+ref+`/transition"`) || !strings.Contains(status, `name="csrf_token"`) {
		t.Errorf("the status menu is not a transition form with CSRF:\n%s", status)
	}
	if got := routeButtons(popover(t, laneCard(t, page, careful), "lflow-"+careful)); slices.Contains(got, "blocked") {
		t.Errorf("a move needing a comment is offered from the lane: %v", got)
	}

	moved := b.post("/tasks/"+ref+"/transition", url.Values{"route": {"blocked"}, "next": {"/tasks"}})
	wantStatus(t, moved, http.StatusSeeOther)
	_ = moved.Body.Close()
	raised := b.post("/tasks/"+ref+"/priority", url.Values{"priority": {"2"}, "next": {"/tasks"}})
	wantStatus(t, raised, http.StatusSeeOther)
	_ = raised.Body.Close()
	after := laneCard(t, b.page("/tasks"), ref)
	if !strings.Contains(after, `data-state="blocked">Blocked</span>`) || !strings.Contains(after, `<span class="prio p-high"><span class="caret" aria-hidden="true"></span>high</span>`) {
		t.Errorf("the card does not show blocked and high after choosing them:\n%s", after)
	}

	viewer := f.as("viewer")
	viewer.chooseLanes("/tasks")
	read := laneCard(t, viewer.page("/tasks"), ref)
	if strings.Contains(read, "/transition\"") || strings.Contains(read, "/priority\"") || strings.Contains(read, "popovertarget=\"lflow-") {
		t.Errorf("a reader who may only read is offered a status or priority form:\n%s", read)
	}
	if !strings.Contains(read, `<span class="pill status lane-state`) || !strings.Contains(read, `<a class="prio p-high"`) {
		t.Errorf("a reader who may only read does not see the status and priority as labels:\n%s", read)
	}
}

var hxPattern = regexp.MustCompile(`<form method="post" action="([^"]+)"[^>]*>`)

// Every lane form swaps only its own card, from the listing the redirect
// lands on, and that listing draws the card under the id the form selects.
func TestALaneActionSwapsOnlyItsCard(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "in place")
	b.chooseLanes("/tasks")
	card := laneCard(t, b.page("/tasks"), ref)

	forms := 0
	for _, m := range hxPattern.FindAllStringSubmatch(card, -1) {
		if strings.HasSuffix(m[1], "/star") {
			continue
		}
		forms++
		for _, want := range []string{`hx-post="` + m[1] + `"`, `hx-target="#t-` + ref + `"`, `hx-select="#t-` + ref + `"`,
			`hx-swap="outerHTML"`, `hx-indicator="#t-` + ref + `"`, `data-lane-control="`} {
			if !strings.Contains(m[0], want) {
				t.Errorf("the form posting to %s does not carry %s:\n%s", m[1], want, m[0])
			}
		}
	}
	if forms != 3 {
		t.Errorf("found %d lane forms, want the tick, the status and the priority", forms)
	}

	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+"/tasks/"+ref+"/transition",
		strings.NewReader(url.Values{"route": {"doing"}, "next": {"/tasks"}, "csrf_token": {b.csrf()}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/tasks?") || !strings.HasSuffix(loc, "#t-"+ref) {
		t.Fatalf("the swap's redirect goes to %q, not this listing at the card", loc)
	}
	landed := b.page(strings.TrimSuffix(loc, "#t-"+ref))
	if n := strings.Count(landed, `id="t-`+ref+`"`); n != 1 {
		t.Fatalf("the listing the swap selects from draws %d elements with the card's id", n)
	}
	if after := laneCard(t, landed, ref); !strings.Contains(after, `data-state="doing">Doing</span>`) {
		t.Errorf("the card the swap selects does not show the move:\n%s", after)
	}

	stale := b.post("/tasks/"+ref+"/transition", url.Values{"route": {"doing"}, "next": {"/tasks"}})
	defer func() { _ = stale.Body.Close() }()
	if stale.StatusCode < 400 {
		t.Fatalf("a route that no longer applies was accepted: %d", stale.StatusCode)
	}
	if page := body(t, stale); !strings.Contains(page, `class="failure-message"`) {
		t.Errorf("a refusal carries no failure message for lanes.js to show:\n%s", page)
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

// A lane card carries the same star toggle as a list row, and a starred task
// leads its own lane: the lanes are the listing split by project, so the
// reader's starred tasks come first in each.
func TestALaneCardStarsAndAStarredTaskLeadsItsLane(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "first in")
	second := b.createTask("infra", "second in")
	b.chooseLanes("/tasks")

	page := b.page("/tasks")
	card := laneCard(t, page, second)
	if !strings.Contains(card, `action="/tasks/`+second+`/star"`) || !strings.Contains(card, `aria-pressed="false"`) {
		t.Fatalf("the lane card offers no unpressed star:\n%s", card)
	}
	if got := laneRefs(laneBlock(t, page, "infra")); !slices.Equal(got, []string{first, second}) {
		t.Fatalf("before starring the lane holds %v, want %v", got, []string{first, second})
	}

	resp := b.post("/tasks/"+second+"/star", url.Values{"next": {"/tasks"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()

	page = b.page("/tasks")
	if got := laneRefs(laneBlock(t, page, "infra")); !slices.Equal(got, []string{second, first}) {
		t.Errorf("after starring the lane holds %v, want the starred %s first", got, second)
	}
	card = laneCard(t, page, second)
	if !strings.Contains(card, `action="/tasks/`+second+`/star/remove"`) || !strings.Contains(card, `aria-pressed="true"`) {
		t.Errorf("the starred lane card does not offer a pressed unstar:\n%s", card)
	}
}

// rulesFor are the bodies of every rule under one at-rule whose selector list
// names the selector exactly, so a guard reads the rule it means and not one
// that merely mentions the same class elsewhere.
func rulesFor(sheet, media, selector string) string {
	var out []string
	for _, r := range cssRules(sheet) {
		if r.media != media {
			continue
		}
		for _, one := range strings.Split(r.selector, ",") {
			if strings.TrimSpace(one) == selector {
				out = append(out, r.body)
			}
		}
	}
	return strings.Join(out, ";")
}

// On a phone the page scrolls and the lanes snap sideways at their natural
// height, and on a touch screen the tick and the two menus answer a 40px
// press. Read off the rules that say so, under the at-rule that scopes them.
func TestLanesFitAPhoneAndAFinger(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	css := body(t, f.as("alice").get("/assets/app.css"))
	phone := "@media (max-width: 639px)"
	checks := []struct{ media, selector, want string }{
		{phone, ".lanes", "height: auto"},
		{phone, ".lanes", "scroll-snap-type: x mandatory"},
		{phone, ".lane", "overflow-y: visible"},
		{phone, ".lane", "flex: 0 0 86vw"},
		{phone, ".lane", "scroll-snap-stop: always"},
		{"@media (pointer: coarse)", ".lane button.check::before", "inset: -11px"},
		{"@media (pointer: coarse)", ".lane .lanepill::before", "inset: -11px -2px"},
		{"@media (pointer: coarse)", ".lane button.check", "position: relative"},
		{"", ".lane .lane-card.htmx-request", "cursor: progress"},
		{"", ".lane .lane-card.htmx-request *", "pointer-events: none"},
	}
	for _, c := range checks {
		if got := rulesFor(css, c.media, c.selector); !strings.Contains(got, c.want) {
			t.Errorf("%s %s does not declare %q: %q", c.media, c.selector, c.want, got)
		}
	}
}
