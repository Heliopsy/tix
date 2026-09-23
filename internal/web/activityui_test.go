// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/url"
	"strings"
	"testing"
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
