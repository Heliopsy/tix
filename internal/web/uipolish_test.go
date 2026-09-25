// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// pagedListings are the six screens that page through a keyset. They all
// hand-rolled their own control before this: five bare paragraphs holding a
// link, one div.actions, two different words for the same direction, no
// styling on any of them and no way back from any of them.
var pagedListings = []string{"/tasks", "/projects", "/actors", "/activity", "/admin/users", "/sync/runs"}

func TestEveryPagedListingRendersTheOnePagerPartial(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, path := range pagedListings {
		page := b.page(path)
		for _, gone := range []string{">Next page<", ">Older<"} {
			if strings.Contains(page, gone) {
				t.Errorf("%s still carries its own hand-rolled pager (%s)", path, gone)
			}
		}
	}
}

// The control has to exist, be styled and say where the reader is. The task
// list is the one listing a fixture can fill past a page without minutes of
// seeding, so it is where the rendered control is checked.
func TestThePagerSaysWhereTheReaderIsAndOffersBothDirections(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range core.DefaultPageLimit + 5 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	first := b.page("/tasks")
	if !strings.Contains(first, `class="pager"`) {
		t.Fatalf("the task list renders no pager:\n%s", tail(first))
	}
	if !strings.Contains(first, "Page 1") {
		t.Error("the pager does not say which page the reader is on")
	}
	if strings.Contains(first, "pager-prev") {
		t.Error("the first page offers a way back to nowhere")
	}
	next := hrefWithClass(t, first, "pager-next")

	second := b.page(next)
	if !strings.Contains(second, "Page 2") {
		t.Error("the second page does not say it is the second page")
	}
	prev := hrefWithClass(t, second, "pager-prev")

	back := b.page(prev)
	if !strings.Contains(back, "Page 1") || strings.Contains(back, "pager-prev") {
		t.Errorf("walking back from page two did not land on page one:\n%s", tail(back))
	}
}

// The trail is untrusted input. A value this build did not issue costs the
// reader a return to the first page and never a page of somebody else's rows
// or a failed screen.
func TestAnInventedTrailIsRefusedRatherThanWalked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range core.DefaultPageLimit + 5 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}
	next := hrefWithClass(t, b.page("/tasks"), "pager-next")

	for _, trail := range []string{"!!!!", "YWJj", strings.Repeat("A", 8000), "../../etc/passwd"} {
		resp := b.get(next + "&" + url.Values{"trail": {trail}}.Encode())
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("a trail of %q took the screen down with %d", trail, resp.StatusCode)
		}
		page := body(t, resp)
		prev := hrefWithClass(t, page, "pager-prev")
		if strings.Contains(prev, "trail=") {
			t.Errorf("a trail of %q survived into %q", trail, prev)
		}
	}
}

// A filter or a sort is a different result set, or a different ordering of
// one, so the cursors of a walk through the old one address rows the reader
// was never shown. The filter form submits neither, which is what resets it.
func TestTheFilterFormCarriesNoPosition(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range core.DefaultPageLimit + 5 {
		b.createTask("infra", "row "+strconv.Itoa(i))
	}

	// Deep in a walk, where a filter change is exactly the moment a stale
	// cursor would resume somebody mid-list.
	page := b.page(hrefWithClass(t, b.page("/tasks"), "pager-next"))
	form := between(t, page, `<form method="get" action="/tasks"`, "</form>")
	if form == "" {
		t.Fatal("the task list has no filter form")
	}
	for _, gone := range []string{"cursor", "trail"} {
		if strings.Contains(form, `name="`+gone+`"`) {
			t.Errorf("the filter form carries the reader's %s forward, so a new filter would resume mid-walk", gone)
		}
	}
}

// The listing showed everything about a task except whose it was, which is
// the one question somebody opens a shared list to answer.
func TestTheTaskListShowsAssigneeHandles(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "somebody owns this")
	assign(t, b, ref, "alice")
	b.createTask("infra", "nobody owns this")

	page := b.page("/tasks")
	if !strings.Contains(page, `class="who-ref"`) {
		t.Fatalf("the task list shows no assignee:\n%s", tail(page))
	}
	if !strings.Contains(page, ">alice<") {
		t.Error("the assignee is not named by handle")
	}
	if strings.Contains(page, f.actorA.ID) {
		t.Error("the assignee is shown as an identifier rather than a handle")
	}
	if !strings.Contains(page, "unassigned") {
		t.Error("a row nobody owns says nothing, so the column reads as missing data")
	}
}

// The column is one of the listing's own, so it can be turned off like any
// other, and it is on by default because it was the column that was asked for.
func TestTheAssigneeColumnIsOnByDefaultAndCanBeTurnedOff(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "somebody owns this")
	assign(t, b, ref, "alice")

	if !strings.Contains(b.page("/tasks"), `class="who-ref"`) {
		t.Fatal("the assignee column is off for a browser that has chosen nothing")
	}
	resp := b.post("/columns", url.Values{
		"page": {"tasks"}, "next": {"/tasks"},
		"column": {"status", "priority", "tags", "project", "ref"},
	})
	_ = resp.Body.Close()
	if strings.Contains(b.page("/tasks"), `class="who-ref"`) {
		t.Error("turning the assignee column off left it on screen")
	}
}

// The detail page listed a raw ULID directly above a field placeholdered
// "infra-2", so the screen contradicted itself about what identifies a task.
func TestTaskDependenciesAreNamedTheWayTheFieldAsksForThem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	subject := b.createTask("infra", "waits for another")
	blocker := b.createTask("infra", "the one it waits for")

	resp := b.post("/tasks/"+subject+"/deps", url.Values{"depends_on": {blocker}})
	_ = resp.Body.Close()

	page := b.page("/tasks/" + subject)
	deps := between(t, page, "<h3>Dependencies</h3>", "</ul>")
	if deps == "" {
		t.Fatalf("no dependency list on the task screen:\n%s", tail(page))
	}
	if !strings.Contains(deps, ">"+blocker+"<") {
		t.Errorf("the dependency is not named by reference:\n%s", deps)
	}
	if !strings.Contains(deps, "the one it waits for") {
		t.Errorf("the dependency carries no title, so it is an identifier with no meaning:\n%s", deps)
	}
	if !strings.Contains(deps, `href="/tasks/`+blocker+`"`) {
		t.Errorf("the dependency does not link to the task it names:\n%s", deps)
	}
	if !strings.Contains(deps, `name="depends_on" value="01`) {
		t.Error("the remove form no longer submits the identifier the service takes back")
	}
}

// "Where the work is" answered with core.CategoryInProgress's value, which is
// a Go constant. The value is the contract and does not move; the words do.
func TestStatisticsNamesItsCategoriesInWords(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "somewhere in a category")

	page := b.page("/stats")
	where := between(t, page, "<h2>Where the work is</h2>", "</table>")
	if where == "" {
		t.Fatalf("no category table on the statistics screen:\n%s", tail(page))
	}
	if strings.Contains(where, ">in_progress<") {
		t.Errorf("the category is shown as its wire value:\n%s", where)
	}
	if !strings.Contains(where, "To do") && !strings.Contains(where, "In progress") {
		t.Errorf("no category is named in words:\n%s", where)
	}
}

// A lease that ran out is the one entry in a history nobody performed, and
// the only one that says work was dropped rather than done.
func TestALeaseExpiryIsMarkedApartFromEveryOtherChange(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := expireAClaim(t, f, b)

	page := b.page("/tasks/" + ref)
	if !strings.Contains(page, "act-expire") {
		t.Errorf("a lease expiry still carries the generic mark:\n%s", tail(page))
	}
	if !strings.Contains(page, ">expire<") {
		t.Error("the expiry row is not labelled with what happened")
	}
}

// The sweeper clears the lease columns within a minute of a claim lapsing, so
// the badge that reads them was effectively unreachable: an abandoned task
// looked exactly like one nobody had touched.
func TestTheExpiredClaimBadgeSurvivesTheSweep(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := expireAClaim(t, f, b)

	page := b.page("/tasks")
	row := between(t, page, `id="t-`+ref+`"`, "</li>")
	if row == "" {
		t.Fatalf("no row for %s:\n%s", ref, tail(page))
	}
	if !strings.Contains(row, "claim expired") {
		t.Errorf("the swept task shows no sign that a holder dropped it:\n%s", row)
	}
	if !strings.Contains(row, "ago") {
		t.Errorf("the badge does not say when the claim lapsed:\n%s", row)
	}
	if !strings.Contains(row, "alice") {
		t.Errorf("the badge does not say whose claim it was:\n%s", row)
	}
}

// claim_count is only a signal beside an expiry: seven claims and a lapse is
// an agent that keeps dying, while seven claims and no lapse is ordinary work
// changing hands. It rides on the badge rather than becoming a column.
func TestTheClaimCountRidesOnTheExpiredBadge(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := expireAClaim(t, f, b)
	// A second claim, also abandoned, is what "this keeps happening" looks
	// like in the data.
	expireTheClaimOn(t, f, ref)

	row := between(t, b.page("/tasks"), `id="t-`+ref+`"`, "</li>")
	if !strings.Contains(row, "2 claims") {
		t.Errorf("the row does not say how many times the task has been claimed:\n%s", row)
	}
	if !strings.Contains(b.page("/tasks/"+ref), "claimed 2 times") {
		t.Error("the task screen does not carry the claim count")
	}
}

// A tenant administrator could not find the tenant screen at all: it was
// reachable by typing its URL and by no other means, because the only entry
// to it lived behind a preference on a settings page they had no reason to
// open.
func TestAnAdministratorFindsTheConfigurationScreensWithoutBeingTold(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	admin := nav(t, f.as("alice").page("/tasks"))
	if !strings.Contains(admin, `href="/admin/tenant"`) {
		t.Error("an administrator's navigation does not reach the tenant screen")
	}

	// Every screen in those groups refuses a viewer, and an entry that leads
	// to a refusal is worse than no entry.
	viewer := nav(t, f.as("viewer").page("/tasks"))
	if strings.Contains(viewer, `href="/admin/tenant"`) {
		t.Error("a viewer is offered a screen that will refuse them")
	}
}

func TestBeingOnAHiddenScreenExpandsItsGroup(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/advanced", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()
	if strings.Contains(nav(t, b.page("/tasks")), `href="/admin/tenant"`) {
		t.Fatal("hiding the configuration groups had no effect")
	}
	// The tenant screen's own body links to the other administration screens,
	// so this has to read the navigation and not the page: asserting over the
	// whole page passed with the rule deleted.
	if !strings.Contains(nav(t, b.page("/admin/tenant")), `href="/admin/domains"`) {
		t.Error("a reader on a hidden screen is shown a navigation that denies it exists")
	}
	// The choice still holds everywhere else, rather than the visit undoing it.
	if strings.Contains(nav(t, b.page("/tasks")), `href="/admin/tenant"`) {
		t.Error("visiting a hidden screen turned the groups back on for every other screen")
	}
}

// The sidebar is a grid item and the grid already stretches it to the row.
// Binding it to the viewport made it stop painting at the fold and leave a
// bare strip beside every page taller than the window.
func TestTheSidebarIsNotBoundToTheViewportHeight(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))

	// Every .side block, not the first one: the narrow-viewport rule comes
	// first in the sheet, so reading only one of them watched nothing.
	blocks := strings.Split(sheet, ".side {")
	if len(blocks) < 2 {
		t.Fatal("the stylesheet declares no sidebar")
	}
	for _, block := range blocks[1:] {
		if body, _, _ := strings.Cut(block, "}"); strings.Contains(body, "height: 100vh") {
			t.Errorf("the sidebar is still a viewport tall, so it stops painting at the fold:\n%s", body)
		}
	}
	if !strings.Contains(sheet, ".side-inner {") {
		t.Error("nothing inside the sidebar sticks, so the navigation scrolls away")
	}
	if !strings.Contains(f.as("alice").page("/tasks"), `class="side-inner"`) {
		t.Error("the layout renders no sticky box inside the sidebar")
	}
}

// helpers

// assign sets a task's assignee through the detail form, the way a browser
// with no script does.
func assign(t *testing.T, b *browser, ref, handle string) {
	t.Helper()
	page := b.page("/tasks/" + ref)
	version := between(t, page, `name="version" value="`, `"`)
	resp := b.post("/tasks/"+ref, url.Values{
		"version": {version}, "title": {between(t, page, `id="title" name="title" value="`, `"`)},
		"assignee": {handle},
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("assigning %s to %s = %d: %s", ref, handle, resp.StatusCode, body(t, resp))
	}
}

// expireAClaim creates a task, claims it and lets the lease run out, then
// sweeps, which is what clears the lease columns and leaves the evidence
// behind. The fixture clock is moved next to the wall clock first, because
// the badge is a judgement about how long ago something happened as the page
// renders, and a fake clock nine months away from now would put every expiry
// outside the window whatever the code did.
func expireAClaim(t *testing.T, f *fixture, b *browser) string {
	t.Helper()
	f.clock.Set(time.Now().Add(-2 * time.Hour))
	ref := b.createTask("infra", "an agent took this and stopped answering")
	expireTheClaimOn(t, f, ref)
	return ref
}

// expireTheClaimOn claims the task, advances past the lease and sweeps.
func expireTheClaimOn(t *testing.T, f *fixture, ref string) {
	t.Helper()
	ctx := core.WithActor(core.WithTenant(context.Background(),
		core.TenantScope{TenantID: f.tenantA.ID}), f.actorFor("alice"))
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatalf("parsing %q: %v", ref, err)
	}
	if _, err := f.svc.ClaimTask(ctx, parsed, core.ClaimInput{TTL: core.Duration(15 * time.Minute)}); err != nil {
		t.Fatalf("claiming %s: %v", ref, err)
	}
	f.clock.Advance(30 * time.Minute)
	if _, err := f.svc.SweepLeases(ctx, 100); err != nil {
		t.Fatalf("sweeping: %v", err)
	}
}

// nav is the sidebar navigation of a rendered page. Several administration
// screens link to each other in their own bodies, so a test about what the
// navigation offers has to read the navigation.
func nav(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<nav class="main">`, "</nav>")
}

// hrefPattern reads one anchor's target out of rendered markup.
var hrefPattern = regexp.MustCompile(`<a class="([^"]*)" href="([^"]*)"`)

// hrefWithClass returns the target of the first anchor carrying a class,
// unescaped the way a browser reads it.
func hrefWithClass(t *testing.T, page, class string) string {
	t.Helper()
	for _, m := range hrefPattern.FindAllStringSubmatch(page, -1) {
		if !strings.Contains(m[1], class) {
			continue
		}
		return strings.ReplaceAll(m[2], "&amp;", "&")
	}
	return ""
}

// tail is the end of a rendered page, for a failure message that has to show
// what was on screen without printing the whole layout.
func tail(page string) string {
	if len(page) <= 2000 {
		return page
	}
	return "..." + page[len(page)-2000:]
}
