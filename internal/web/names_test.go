// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// An actor with a handle is shown by that handle: a name somebody chose beats
// any name a machine can invent.
func TestScreensShowTheHandleOfAnActorThatHasOne(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "who did this")
	if resp := b.post("/tasks/"+ref+"/comments", url.Values{"body": {"a remark"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("commenting = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}

	page := b.page("/tasks/" + ref)
	id := f.actorA.ID
	if strings.Count(page, ">alice<") < 2 {
		t.Errorf("the task screen does not name the actor by handle:\n%s", page)
	}
	if strings.Contains(page, ">"+core.FriendlyName(id)+"<") {
		t.Errorf("a generated name was used where a handle exists:\n%s", page)
	}
	// The identifier stays reachable, which is what a person quotes in a bug
	// report and what a script is given.
	if !strings.Contains(page, `title="`+id+`"`) {
		t.Errorf("the real identifier is not on the element's title:\n%s", page)
	}
}

// An actor with no directory entry still reads as words rather than as a
// twenty-six character identifier.
func TestScreensGenerateANameForAnUnresolvableActor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "assigned to a stranger")

	stranger := f.actorB.ID
	if resp := b.post("/tasks/"+ref, url.Values{
		"title": {"assigned to a stranger"}, "assignee": {stranger}, "version": {"1"},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("assigning = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}

	page := b.page("/tasks/" + ref)
	want := core.FriendlyName(stranger)
	if !strings.Contains(page, want) {
		t.Errorf("an actor of another tenant did not fall back to %q:\n%s", want, page)
	}
	if !strings.Contains(page, `title="`+stranger+`"`) {
		t.Errorf("the generated name hid the identifier instead of standing beside it:\n%s", page)
	}
	if strings.Contains(page, ">"+stranger+"<") {
		t.Errorf("the bare identifier is still rendered as the label:\n%s", page)
	}
}

// Resolving a name must not become a way of reading another tenant's
// directory: the handle of an actor elsewhere never reaches the page.
func TestActorResolutionDoesNotCrossTheTenantBoundary(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "cross tenant check")
	if resp := b.post("/tasks/"+ref, url.Values{
		"title": {"cross tenant check"}, "assignee": {f.actorB.ID}, "version": {"1"},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("assigning = %d, want 303", resp.StatusCode)
	}

	if page := b.page("/tasks/" + ref); strings.Contains(page, ">bob<") {
		t.Errorf("another tenant's handle reached the page:\n%s", page)
	}
}

// The activity feed was the worst offender for raw identifiers: an actor and
// a subject on every row.
func TestActivityFeedNamesItsActorsAndSubjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "shows up in activity")

	page := b.page("/activity")
	for _, want := range []string{`class="lede"`, `class="card"`, `class="plain feed"`,
		`id="live-feed"`, `data-events=`, `class="badge act-create"`, `class="who-name"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the activity feed is missing %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, `class="meta"`) {
		t.Error("the activity feed still uses a class the stylesheet dropped")
	}
	if !strings.Contains(page, ">"+ref+"<") {
		t.Errorf("the feed does not name the task by its reference %q:\n%s", ref, page)
	}
	if !strings.Contains(page, ">alice<") {
		t.Errorf("the feed does not name the actor:\n%s", page)
	}
}
