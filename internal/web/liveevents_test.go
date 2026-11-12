// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"html"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// liveEventsRoot is the opening tag of the element events.js binds to.
func liveEventsRoot(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section id="live-events"`, ">")
}

// The screen draws the stream's element with where to connect and how many
// rows it keeps, and loads its own script.
func TestTheLiveEventsScreenFollowsTheStream(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page(web.RouteLiveEvents)
	root := liveEventsRoot(t, page)
	if root == "" {
		t.Fatalf("the screen has no live events element:\n%s", page)
	}
	if got := attr(t, root, "data-events"); got != "/api/v1/events" {
		t.Errorf("data-events = %q, want the WebSocket route", got)
	}
	if got := attr(t, root, "data-cap"); got != "500" {
		t.Errorf("data-cap = %q, want 500", got)
	}
	if !strings.Contains(page, "Keeping the latest 500 events; older ones are dropped.") {
		t.Error("the screen does not say how many events it keeps")
	}
	if got := attr(t, root, "data-filter"); got != "" {
		t.Errorf("an unfiltered screen sends filter %q, want none", got)
	}
	if !strings.Contains(page, `<script src="/assets/events.js" defer></script>`) {
		t.Error("the screen does not load assets/events.js")
	}
	if strings.Contains(between(t, page, `<section id="live-events"`, "</section>"), "<details") {
		t.Error("the stream hides part of itself in a fold")
	}
}

// The filter lives in the query string: the form reads it back, the chips
// remove one part each, and the subscription carries the server half.
func TestTheLiveEventsFilterRoundTripsThroughTheAddress(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	q := url.Values{"type": {"task.*, comment.added"}, "project": {"infra"}, "actor": {"@alice"}, "since": {"7"}}
	page := f.as("alice").page(web.RouteLiveEvents + "?" + q.Encode())

	form := between(t, page, `action="/events"`, "</form>")
	for id, want := range map[string]string{"le-type": "task.*, comment.added", "le-actor": "@alice", "le-since": "7"} {
		if got := html.UnescapeString(inputValue(t, form, id)); got != want {
			t.Errorf("%s reads %q, want %q", id, got, want)
		}
	}
	if !strings.Contains(between(t, form, `<select id="le-project"`, "</select>"), `<option value="infra" selected>`) {
		t.Errorf("the project picker does not keep infra:\n%s", form)
	}

	root := liveEventsRoot(t, page)
	var filter struct {
		Types      []string `json:"types"`
		ProjectIDs []string `json:"project_ids"`
	}
	if err := json.Unmarshal([]byte(attr(t, root, "data-filter")), &filter); err != nil {
		t.Fatalf("data-filter is not a filter: %v\n%s", err, root)
	}
	if strings.Join(filter.Types, " ") != "task.* comment.added" {
		t.Errorf("types sent = %q", filter.Types)
	}
	if len(filter.ProjectIDs) != 1 || filter.ProjectIDs[0] != f.project.ID {
		t.Errorf("project ids sent = %q, want infra's id %q", filter.ProjectIDs, f.project.ID)
	}
	if got := attr(t, root, "data-actors"); got != `["@alice"]` {
		t.Errorf("data-actors = %q", got)
	}
	if got := attr(t, root, "data-since"); got != "7" {
		t.Errorf("data-since = %q, want 7", got)
	}

	chips := strings.Split(between(t, page, `<p class="activechips">`, "</p>"), "<a ")
	var chip string
	for _, c := range chips {
		if strings.Contains(c, ">project INFRA<") {
			chip = c
		}
	}
	href, err := url.Parse(html.UnescapeString(between(t, chip, `href="`, `"`)))
	if err != nil || href.Path != "/events" {
		t.Fatalf("the project chip links to %q (%v)", href, err)
	}
	if got, want := href.Query().Encode(), (url.Values{"type": {"task.*, comment.added"}, "actor": {"@alice"}, "since": {"7"}}).Encode(); got != want {
		t.Errorf("the project chip keeps %q, want %q", got, want)
	}
}

// A filter the page knows is wrong is refused with the reason and a way
// back, and the page does not subscribe to it.
func TestTheLiveEventsScreenRefusesAFilterItCannotFollow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for path, want := range map[string]string{
		"/events?project=nope": "No project has the key nope.",
		"/events?since=-3":     "The sequence number -3 is not a whole number above zero.",
		"/events?since=abc":    "The sequence number abc is not a whole number above zero.",
	} {
		page := b.page(path)
		problem := between(t, page, `<p class="flash error liveevents-problem"`, "</p>")
		if !strings.Contains(problem, want) {
			t.Errorf("%s: the screen does not say %q:\n%s", path, want, problem)
		}
		if !strings.Contains(problem, `href="/events" data-way-back`) {
			t.Errorf("%s: no way back to the whole stream:\n%s", path, problem)
		}
		if strings.Contains(page, `id="live-events"`) {
			t.Errorf("%s: the page still subscribes", path)
		}
	}
}

// The screen is in the sidebar's Manage group and in the palette, both only
// with Advanced screens on, like the other configuration screens.
func TestLiveEventsIsListedWithAdvancedScreens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, advanced := range []bool{true, false} {
		b := f.as("alice")
		value := "0"
		if advanced {
			value = "1"
		}
		b.setCookie(web.AdvancedCookie, value)
		page := b.page("/tasks")
		admin := between(t, nav(t, page), `id="navgroup-admin"`, "</div>")
		listed := strings.Contains(admin, `<a href="/events" data-rail-name="Live events"`)
		palette := strings.Contains(between(t, page, `id="tix-palette-data"`, "</script>"), `"href":"/events"`)
		if listed != advanced || palette != advanced {
			t.Errorf("advanced=%v: sidebar lists Live events %v, palette offers it %v", advanced, listed, palette)
		}
	}
}
