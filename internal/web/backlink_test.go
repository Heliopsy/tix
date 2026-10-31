// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// backLink is the task screen's way back, href and words, and nothing else.
func backLink(t *testing.T, page string) (href, label string) {
	t.Helper()
	link := between(t, page, `<p class="backlink"><a href="`, "</a></p>")
	if link == "" {
		t.Fatalf("the task screen has no back link")
	}
	href, label, _ = strings.Cut(link, `">`)
	return html.UnescapeString(href), strings.TrimSpace(strings.TrimPrefix(html.UnescapeString(label), "←"))
}

// Where the back link goes, for every way the screen can be reached. The
// referrer cases are the ones that used to fall to the plain task list.
func TestTheTaskScreenGoesBackToTheListingItCameFrom(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "find my way back")
	filtered := "/tasks?q=tag%3Aops&sort=created_at"

	for _, tc := range []struct {
		name, path, referrer, href, label string
	}{
		{"the link carries the listing", "/tasks/" + ref + "?next=" + url.QueryEscape(filtered), "", filtered, "Back to tasks"},
		{"a listing referred it", "/tasks/" + ref, filtered + "&flash=x&flashref=" + ref, "/tasks?q=tag%3Aops&sort=created_at", "Back to tasks"},
		{"a project board referred it", "/tasks/" + ref, "/projects/infra", "/projects/infra", "Back to the infra board"},
		{"its own screen referred it", "/tasks/" + ref + "?flash=tag+added", "/tasks/" + ref + "?next=" + url.QueryEscape(filtered), filtered, "Back to tasks"},
		{"nothing says where", "/tasks/" + ref, "", "/tasks", "Back to tasks"},
		{"a screen that is not a listing referred it", "/tasks/" + ref, "/settings", "/tasks", "Back to tasks"},
		{"an off-site next", "/tasks/" + ref + "?next=" + url.QueryEscape("//evil.example"), "", "/tasks", "Back to tasks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			href, label := backLink(t, b.pageFrom(tc.referrer, tc.path))
			if href != tc.href || label != tc.label {
				t.Errorf("back link = %q %q, want %q %q", href, label, tc.href, tc.label)
			}
		})
	}
}

// A save on the task screen redirects to the task screen, and that redirect
// used to be the bare address, so the second save onwards went back to the
// unfiltered list. Walked through two saves the way a browser makes them,
// each posted from the page the previous one landed on.
func TestTheBackLinkSurvivesSavesOnTheTaskScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "edit me twice")
	filtered := "/tasks?q=tag%3Aops"
	on := "/tasks/" + ref + "?next=" + url.QueryEscape(filtered)

	for _, step := range []struct {
		action string
		form   url.Values
	}{
		{"/tasks/" + ref + "/tags", url.Values{"tag": {"ops"}}},
		{"/tasks/" + ref + "/comments", url.Values{"body": {"a note"}}},
	} {
		resp := b.postFrom(on, step.action, step.form)
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		on = resp.Header.Get("Location")
		if href, _ := backLink(t, b.page(on)); href != filtered {
			t.Fatalf("after %s the back link leads to %q, want %q", step.action, href, filtered)
		}
	}
}

// Deleting the task leaves its screen, and goes to the listing the screen
// would have gone back to rather than to the plain list.
func TestDeletingATaskReturnsToItsListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "delete me")
	resp := b.postFrom("/tasks/"+ref+"?next="+url.QueryEscape("/tasks?q=tag%3Aops"), "/tasks/"+ref+"/delete", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks?q=tag%3Aops") {
		t.Errorf("deleting went to %q", loc)
	}
}

// A failed page reached from a filtered listing leads back to that listing,
// and still offers the task list, never the root.
func TestAFailedPageLeadsBackToTheListingThatLinkedIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.pageFrom("/tasks?q=tag%3Aops", "/tasks/infra-999")
	ways := between(t, page, `<div class="actions">`, `</div>`)
	if !strings.Contains(ways, `<a href="/tasks?q=tag%3aops">Back to where you were</a>`) &&
		!strings.Contains(ways, `<a href="/tasks?q=tag%3Aops">Back to where you were</a>`) {
		t.Errorf("the error screen does not lead back to the filtered listing:\n%s", ways)
	}
	if !strings.Contains(ways, `<a href="/tasks">Go to tasks</a>`) {
		t.Errorf("the error screen does not offer the task list:\n%s", ways)
	}
	if strings.Contains(ways, `href="/"`) {
		t.Errorf("the error screen offers the root:\n%s", ways)
	}
}
