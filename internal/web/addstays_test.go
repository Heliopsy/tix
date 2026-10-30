// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// postFrom submits a form the way a browser does from a page it is showing,
// carrying that page as the referrer.
func (b *browser) postFrom(referrer, path string, form url.Values) *http.Response {
	b.t.Helper()
	form.Set("csrf_token", b.csrf())
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", b.fix.server.URL+referrer)
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("posting %s: %v", path, err)
	}
	return resp
}

// pageFrom fetches a page as though a link on referrer was followed.
func (b *browser) pageFrom(referrer, path string) string {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodGet, b.fix.server.URL+path, nil)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	if referrer != "" {
		req.Header.Set("Referer", b.fix.server.URL+referrer)
	}
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("getting %s: %v", path, err)
	}
	return body(b.t, resp)
}

// Both add paths, with no preference stored, return to the listing they were
// posted from rather than opening the task. The redirect has to carry the
// filter, the toast's link and the row's anchor, all naming the new task.
func TestAddingATaskStaysOnTheListingByDefault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		form url.Values
	}{
		{"quick add", url.Values{"project_ref": {"infra"}, "title": {"one line"}}},
		{"add with details", url.Values{"project_ref": {"infra"}, "title": {"with details"},
			"body": {"notes"}, "tags": {"ops"}, "repeat": {"weekly"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")
			resp := b.postFrom("/tasks?q=tag%3Aops&sort=created_at", "/tasks", tc.form)
			_ = resp.Body.Close()
			wantStatus(t, resp, http.StatusSeeOther)

			loc, err := url.Parse(resp.Header.Get("Location"))
			if err != nil {
				t.Fatalf("parsing %q: %v", resp.Header.Get("Location"), err)
			}
			if loc.Path != "/tasks" {
				t.Fatalf("adding sent the reader to %q, not back to the listing", loc)
			}
			ref := loc.Query().Get("flashref")
			if ref == "" || loc.Fragment != "t-"+ref {
				t.Errorf("the redirect names no new task to link and mark: %q", loc)
			}
			if loc.Query().Get("q") != "tag:ops" || loc.Query().Get("sort") != "created_at" {
				t.Errorf("the listing lost its filter or sort: %q", loc)
			}
			if loc.Query().Get("flash") != "task created" {
				t.Errorf("the toast does not say the task was created: %q", loc)
			}
		})
	}
}

// The listing the create lands on says what happened, links the task and
// marks its row, and only its row.
func TestTheListingLinksAndMarksTheTaskJustAdded(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "already here")
	resp := b.postFrom("/tasks", "/tasks", url.Values{"project_ref": {"infra"}, "title": {"just added"}})
	_ = resp.Body.Close()
	added := createdRef(resp.Header.Get("Location"))

	page := b.page(resp.Header.Get("Location"))
	toast := between(t, page, `<p class="flash">`, "</p>")
	if !strings.Contains(toast, "task created") || !strings.Contains(toast, `href="/tasks/`+added+`"`) {
		t.Errorf("the toast does not link the new task:\n%s", toast)
	}
	if row := dueRow(t, page, "just added"); !strings.Contains(row, "is-flashed") {
		t.Errorf("the new row is not marked:\n%s", row)
	}
	if row := dueRow(t, page, "already here"); strings.Contains(row, "is-flashed") {
		t.Errorf("a row other than %s is marked as %s:\n%s", first, added, row)
	}

	// Adding publishes an event, so the listing refreshes the moment it is
	// drawn. The refresh has to ask for the same mark, or the ring is gone
	// before anybody sees it.
	live := html.UnescapeString(between(t, page, `data-live="`, `"`))
	if !strings.Contains(live, "flashref="+added) {
		t.Fatalf("the live refresh drops the mark: %q", live)
	}
	if row := dueRow(t, b.page(live), "just added"); !strings.Contains(row, "is-flashed") {
		t.Errorf("the refreshed listing no longer marks the new row:\n%s", row)
	}
}

// Adding a second task from the listing the first one left the reader on
// must name the second. The referrer carries the first toast, and appending
// to it put two flashrefs on the address, of which the page reads the first.
func TestAddingTwiceNamesTheSecondTask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	one := b.postFrom("/tasks", "/tasks", url.Values{"project_ref": {"infra"}, "title": {"first"}})
	_ = one.Body.Close()
	landed := strings.TrimPrefix(one.Header.Get("Location"), f.server.URL)
	landed, _, _ = strings.Cut(landed, "#")

	two := b.postFrom(landed, "/tasks", url.Values{"project_ref": {"infra"}, "title": {"second"}})
	_ = two.Body.Close()
	loc, _ := url.Parse(two.Header.Get("Location"))
	if got := loc.Query()["flashref"]; len(got) != 1 || got[0] != createdRef(two.Header.Get("Location")) || got[0] == createdRef(one.Header.Get("Location")) {
		t.Fatalf("the second add's redirect names %v: %q", got, loc)
	}
	if got := loc.Query()["flash"]; len(got) != 1 {
		t.Errorf("the second add's redirect carries %d toasts: %q", len(got), loc)
	}
}

// The preference still opens the task for a browser that chose it: the "1"
// this build writes, and the empty value earlier builds wrote for "on". A
// browser that turned it off stays, as one that never chose now does.
func TestOpenANewTaskHonoursAStoredChoice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cookie *string
		opens  bool
	}{
		{"never chosen", nil, false},
		{"turned off", ptrString("0"), false},
		{"turned on by this build", ptrString("1"), true},
		{"turned on by an earlier build", ptrString(""), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")
			form := url.Values{"project_ref": {"infra"}, "title": {"t"}, "csrf_token": {b.csrf()}}
			req, err := http.NewRequest(http.MethodPost, f.server.URL+"/tasks", strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Cookie", web.CSRFCookieName+"="+b.cookie(web.CSRFCookieName))
			if tc.cookie != nil {
				req.Header.Set("Cookie", req.Header.Get("Cookie")+"; "+web.OpenAfterAddCookie+"="+*tc.cookie)
			}
			b.stamp(req)
			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			wantStatus(t, resp, http.StatusSeeOther)
			opened := strings.HasPrefix(resp.Header.Get("Location"), "/tasks/")
			if opened != tc.opens {
				t.Errorf("opened = %v, want %v: %q", opened, tc.opens, resp.Header.Get("Location"))
			}
		})
	}
}

// The toggle writes both positions explicitly and flips from the new
// default: the first press from a browser that never chose turns opening on.
func TestTheOpenANewTaskToggleWritesBothPositions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, want := range []string{"1", "0", "1"} {
		resp := b.post(web.RouteOpenAfterAdd, url.Values{"next": {"/settings"}})
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		if got := b.cookie(web.OpenAfterAddCookie); got != want {
			t.Fatalf("after a press the cookie holds %q, want %q", got, want)
		}
	}
}

func ptrString(s string) *string { return &s }
