// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// Every row of the feed used to be a dead end: the actor was plain text and a
// comment was the words "a comment" with nowhere to go.
func TestEveryActivityRowLeadsToWhatItIsAbout(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "has something to say")

	commented := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"worth reading"}})
	_ = commented.Body.Close()

	page := b.page("/activity")

	actorHref := `href="/activity?actor=` + f.actorA.ID + `"`
	if !strings.Contains(page, actorHref) {
		t.Errorf("the actor on a row is not a link to that actor's trail:\n%s", page)
	}
	if !strings.Contains(page, `href="/tasks/`+ref+`"`) {
		t.Errorf("a task row does not link to the task:\n%s", page)
	}
	if !strings.Contains(page, "a comment on "+ref) {
		t.Errorf("a comment row does not say which task it is on:\n%s", page)
	}
	if !strings.Contains(page, `href="/tasks/`+ref+`#comment-`) {
		t.Errorf("a comment row does not link to the comment itself:\n%s", page)
	}
}

// A link that lands on a record inside a closed disclosure has to open it, or
// the reader arrives at a page with no sign of what they followed.
func TestALinkedRecordIsAddressableOnItsOwnScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "carries a comment")

	commented := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"landing here"}})
	_ = commented.Body.Close()

	detail := b.page("/tasks/" + ref)
	if !strings.Contains(detail, `<li class="comment" id="comment-`) {
		t.Errorf("a comment carries no anchor for a link to land on:\n%s", detail)
	}
	if !strings.Contains(detail, `data-open-for="artifact"`) {
		t.Errorf("the artifacts disclosure cannot be opened by a link addressing one")
	}

	script := body(t, b.get("/assets/copy.js"))
	for _, want := range []string{"targetKind", "data-open-for", "is-target", "htmx:load"} {
		if !strings.Contains(script, want) {
			t.Errorf("the script that reveals a linked record is missing %q", want)
		}
	}
}

// The filter has to live in the address, or it cannot be shared, bookmarked or
// restored, and it has to narrow what is actually rendered.
func TestTheActivityFilterIsAddressableAndNarrows(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	kept := b.createTask("infra", "kept by the filter")
	dropped := b.createTask("infra", "dropped by the filter")

	all := b.page("/activity")
	if !strings.Contains(all, kept) || !strings.Contains(all, dropped) {
		t.Fatalf("the unfiltered feed does not carry both tasks")
	}

	narrowed := b.page("/activity?q=" + url.QueryEscape(kept))
	if !strings.Contains(narrowed, kept) {
		t.Errorf("the search dropped the record it names:\n%s", narrowed)
	}
	if strings.Contains(narrowed, ">"+dropped+"<") {
		t.Errorf("the search kept a record it does not match:\n%s", narrowed)
	}
	if !strings.Contains(narrowed, `class="activechips"`) {
		t.Errorf("an active filter is not shown back to the reader:\n%s", narrowed)
	}
	if !strings.Contains(narrowed, `href="/activity"`) {
		t.Error("there is no way to clear the filter")
	}

	// The control is a plain GET form: no script, and its state is the URL.
	if !strings.Contains(all, `<form method="get" action="/activity"`) {
		t.Errorf("the filter is not a plain form:\n%s", all)
	}

	// A live refresh must not widen the feed back out.
	if !strings.Contains(narrowed, `data-feed="/activity/feed?q=`) {
		t.Errorf("the live refresh does not carry the filter:\n%s", narrowed)
	}
	fragment := b.page("/activity/feed?q=" + url.QueryEscape(kept))
	if strings.Contains(fragment, ">"+dropped+"<") {
		t.Errorf("the live fragment ignores the filter:\n%s", fragment)
	}
}

// Structured terms are answered by the store, so each one has to actually
// narrow the feed rather than being decoration on the form.
func TestActivityFilterTermsNarrowIndependently(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "a task, not a session")

	tasksOnly := b.page("/activity?kind=task")
	if !strings.Contains(tasksOnly, ref) {
		t.Errorf("the task kind dropped a task:\n%s", tasksOnly)
	}

	sessionsOnly := b.page("/activity?kind=session")
	if strings.Contains(sessionsOnly, ">"+ref+"<") {
		t.Errorf("the session kind kept a task:\n%s", sessionsOnly)
	}

	elsewhere := b.page("/activity?source=cli")
	if strings.Contains(elsewhere, ">"+ref+"<") {
		t.Errorf("a task created over the web appears under the cli surface:\n%s", elsewhere)
	}

	nothing := b.page("/activity?q=" + url.QueryEscape("no-record-says-this"))
	if !strings.Contains(nothing, "matches that filter") {
		t.Errorf("a search matching nothing reads as an empty tenant:\n%s", nothing)
	}
	if !strings.Contains(nothing, "records searched") {
		t.Errorf("a search matching nothing does not say how much it looked at:\n%s", nothing)
	}
}

// A filter belongs to the reader who set it and must not reach across a
// tenant boundary, whichever term carries it.
func TestTheActivityFilterCannotReachAnotherTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	other := f.as("bob")
	secret := other.createTask("other", "bob's private work")

	// The search term itself is echoed back in the form and in the chip that
	// offers to remove it, which is the reader's own input; what must never
	// appear is a row. So this reads the rendered feed, not the whole page.
	page := f.as("alice").page("/activity?q=" + url.QueryEscape(secret))
	if rows := between(t, page, `id="live-feed"`, "</ul>"); strings.Contains(rows, ">"+secret+"<") {
		t.Fatalf("a search reached another tenant's record:\n%s", rows)
	}

	byThem := f.as("alice").page("/activity?actor=" + url.QueryEscape(f.actorB.ID))
	rows := between(t, byThem, `id="live-feed"`, "</ul>")
	if strings.Contains(rows, "bob") || strings.Contains(rows, secret) {
		t.Fatalf("filtering by another tenant's actor disclosed their work:\n%s", rows)
	}
	if !strings.Contains(rows, "matches that filter") {
		t.Errorf("filtering by an actor of another tenant did not come back empty:\n%s", rows)
	}
}

// Four kinds of fact on one row, four shapes. Colour alone is not a
// distinction: a reader who cannot separate the hues still has to be able to
// tell a tag from a project from a priority.
func TestATaskRowDistinguishesItsMarkingsByShape(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "wears every marking")

	tagged := b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}})
	_ = tagged.Body.Close()
	raised := b.post("/tasks/"+ref, url.Values{
		"title": {"wears every marking"}, "priority": {"1"}, "version": {"1"}})
	_ = raised.Body.Close()

	page := b.page("/tasks")
	shapes := map[string]string{
		"status":   `<span class="pill status todo">`,
		"priority": `<span class="prio p-highest"`,
		"tag":      `<a class="tagref" href="/tasks?q=tag:ops"`,
		"project":  `href="/tasks?q=project:infra"`,
	}
	for kind, want := range shapes {
		if !strings.Contains(page, want) {
			t.Errorf("the %s marking is missing %q:\n%s", kind, want, page)
		}
	}
	if strings.Contains(page, `<span class="badge todo">`) {
		t.Error("a status still renders as the generic badge every other marking used")
	}

	// The same fact has to take the same shape on the task's own screen and
	// on the board, or the distinction stops meaning anything one click in.
	detail := b.page("/tasks/" + ref)
	for _, want := range []string{`<span class="pill status todo">`, `<span class="prio p-highest"`} {
		if !strings.Contains(detail, want) {
			t.Errorf("the task screen renders %q differently from the list:\n%s", want, detail)
		}
	}
	board := b.page("/projects/infra")
	if !strings.Contains(board, `<span class="prio p-highest"`) {
		t.Errorf("the board renders a priority differently from the list:\n%s", board)
	}
}

// The summary is the only place the list says how much work is on it, and
// every figure in it is counted rather than rendered from a row. The project
// count below is one of five and says nothing about the other four.
func TestTheTaskListSummaryCountsWhatIsLeftAndWhatIsDone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range 3 {
		b.createTask("infra", fmt.Sprintf("still open %d", i))
	}
	finished := b.createTask("infra", "already finished")
	done := b.post("/tasks/"+finished+"/complete", url.Values{})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)

	summary := between(t, b.page("/tasks"), `class="lede summary"`, "</p>")
	if !strings.Contains(summary, "<strong>3</strong> to go") {
		t.Errorf("the summary does not count what is left:\n%s", summary)
	}
	if !strings.Contains(summary, ", 1 done") {
		t.Errorf("the summary does not count what is finished:\n%s", summary)
	}
	if strings.Contains(summary, "All clear") {
		t.Errorf("a list with open work reads as finished:\n%s", summary)
	}
}

// The other end of the same count: a list holding nothing but finished work
// says so instead of reading "0 to go", and it takes the Total and the Done
// agreeing to get there.
func TestATaskListWithNothingLeftReadsAsFinished(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "the only one")
	done := b.post("/tasks/"+ref+"/complete", url.Values{})
	_ = done.Body.Close()
	wantStatus(t, done, http.StatusSeeOther)

	summary := between(t, b.page("/tasks"), `class="lede summary"`, "</p>")
	if !strings.Contains(summary, "All clear") {
		t.Errorf("a list with nothing left does not say so:\n%s", summary)
	}
}

// A list that only ever shows rows says nothing about the shape of the work.
func TestTheTaskListSaysHowManyProjectsItSpans(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "one project")

	single := b.page("/tasks")
	if !strings.Contains(single, "across 1 project") || strings.Contains(single, "across 1 projects") {
		t.Errorf("the summary does not count a single project:\n%s",
			between(t, single, `class="lede summary"`, "</p>"))
	}

	made := b.post("/projects", url.Values{"key": {"second"}, "name": {"Second"}})
	_ = made.Body.Close()
	b.createTask("second", "another project")

	both := b.page("/tasks")
	if !strings.Contains(both, "across 2 projects") {
		t.Errorf("the summary does not count both projects:\n%s",
			between(t, both, `class="lede summary"`, "</p>"))
	}
}

// activeChips is the row of removable filter chips, so an assertion about
// what a filter shows back to the reader reads that row rather than finding
// the same words in the filter form above it.
func activeChips(t *testing.T, page string) string {
	t.Helper()
	chips := between(t, page, `<p class="activechips">`, "</p>")
	if chips == "" {
		t.Fatalf("the feed shows no active filter at all:\n%s", page)
	}
	return chips
}

// activityQuery.Active gates the whole chip row, including the only control
// that clears the filter. It was only ever exercised with a free-text
// search, so a feed narrowed by kind or by source could render no chips and
// no way back to everything while the narrowing tests still passed: they
// read the rows, which were narrowed correctly either way.
func TestEveryActivityFilterTermIsShownBackToTheReader(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "filtered by every term")

	for _, tc := range []struct{ name, query, chip string }{
		{"text", "q=filtered", "matching"},
		{"kind", "kind=task", "task"},
		{"source", "source=web", "from web"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chips := activeChips(t, b.page("/activity?"+tc.query))
			if !strings.Contains(chips, tc.chip) {
				t.Errorf("the %s filter has no chip of its own:\n%s", tc.name, chips)
			}
			if !strings.Contains(chips, `href="/activity"`) {
				t.Errorf("the %s filter offers no way to clear it:\n%s", tc.name, chips)
			}
		})
	}
}

// The actor chip names the person, not the identifier the address bar
// carries. That name is a lookup of its own, because an actor with nothing
// on the page is named by no row of it, and a filter matching nothing is
// exactly when the reader most needs telling whose trail they are on.
func TestTheActorChipNamesAnActorWithNoActivity(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	quiet := seedActor(t, f.store, f.tenantA.ID, "quiet", core.RoleMember)
	b := f.as("alice")
	b.createTask("infra", "by somebody else")

	chips := activeChips(t, b.page("/activity?actor="+url.QueryEscape(quiet.ID)))
	if !strings.Contains(chips, "by quiet") {
		t.Errorf("the actor chip does not name the actor it filters by:\n%s", chips)
	}
	if strings.Contains(chips, quiet.ID) {
		t.Errorf("the actor chip shows the raw identifier:\n%s", chips)
	}
}

// The live fragment was asserted only on what it must not contain, and an
// empty body satisfies that. It has to carry the rows, and carry each of
// them once: the scan loop re-reads store pages until it holds a screenful,
// and a loop that stops on the wrong condition reads the same page again.
func TestTheActivityFragmentCarriesEachRowExactlyOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "shown live")

	fragment := b.page("/activity/feed")
	if !strings.Contains(fragment, ref) {
		t.Fatalf("the live fragment carries no rows at all:\n%s", fragment)
	}
	if n := strings.Count(fragment, ">"+ref+"<"); n != 1 {
		t.Errorf("the live fragment names %s %d times, want once:\n%s", ref, n, fragment)
	}
}

// A full page of the feed has to offer the one behind it. The scan loop
// stops as soon as it holds a screenful, and stopping a page late throws
// away the cursor it should have stopped on, which leaves the oldest records
// unreachable rather than merely mispaged.
func TestAFullActivityPageOffersTheOneBehindIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	// One audit entry per task, one more than a single page holds.
	var oldest string
	for i := range 51 {
		ref := b.createTask("infra", fmt.Sprintf("entry %d", i))
		if i == 0 {
			oldest = ref
		}
	}

	first := b.page("/activity")
	if strings.Contains(first, ">"+oldest+"<") {
		t.Fatalf("the oldest entry is already on the first page")
	}
	next := hrefWithClass(t, first, "pager-next")
	if next == "" {
		t.Fatalf("a full page of the feed offers no way to the one behind it:\n%s",
			between(t, first, `<nav class="pager"`, "</nav>"))
	}
	if !strings.Contains(b.page(next), ">"+oldest+"<") {
		t.Errorf("the page behind the first one does not carry the oldest entry %s", oldest)
	}
}

// A feed of nothing but workflow rows still has to name the workflows. They
// are resolved in one call made only when a row needs one, so whether any
// row does is what the naming hangs on, and a page of them all is the case
// that answer gets wrong.
func TestAWorkflowOnlyFeedStillNamesTheWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	saved := b.post("/workflows", url.Values{
		"key": {"review"}, "name": {"Review"}, "initial": {"open"},
		"states":      {"open|Open|open\nclosed|Closed|terminal"},
		"transitions": {"open>closed"},
	})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	rows := between(t, b.page("/activity?kind=workflow"), `id="live-feed"`, "</ul>")
	if !strings.Contains(rows, "the workflow Review") {
		t.Errorf("a workflow row does not name the workflow:\n%s", rows)
	}
	if !strings.Contains(rows, `href="/workflows/review"`) {
		t.Errorf("a workflow row does not link to the workflow:\n%s", rows)
	}
}

// A membership row is the one whose subject is an actor somebody else acted
// on, so that identifier is resolved alongside the actor who performed the
// row rather than only through it. Unresolved it falls back to a generated
// name, which reads like a person's name and is nobody's.
func TestAMembershipRowNamesTheMemberNotAGeneratedName(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	quiet := seedActor(t, f.store, f.tenantA.ID, "quiet", core.RoleMember)
	b := f.as("alice")

	added := b.post("/admin/tenant/members", url.Values{
		"actor_id": {quiet.ID}, "role": {"member"}})
	_ = added.Body.Close()
	wantStatus(t, added, http.StatusSeeOther)

	rows := between(t, b.page("/activity?kind=membership"), `id="live-feed"`, "</ul>")
	if !strings.Contains(rows, ">quiet</a> to the tenant") {
		t.Errorf("a membership row does not name the member it is about:\n%s", rows)
	}
}
