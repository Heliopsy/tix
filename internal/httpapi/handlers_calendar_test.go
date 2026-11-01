// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// calendarMember makes tenant A's actor a member, which a feed reads as.
func calendarMember(t *testing.T, f *apiFixture) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		return tx.AddMember(ctx, &core.Membership{ActorID: f.actorA.ID, Role: core.RoleMember, CreatedAt: f.clock.Now()})
	}); err != nil {
		t.Fatalf("adding membership: %v", err)
	}
}

func createFeed(t *testing.T, f *apiFixture, filter string) core.IssuedCalendarFeed {
	t.Helper()
	resp := f.call(http.MethodPost, wire.RouteCalendar, core.CalendarFeedInput{Filter: filter})
	mustStatus(t, resp, http.StatusCreated)
	var feed core.IssuedCalendarFeed
	decodeBody(t, resp, &feed)
	return feed
}

// fetchFeed reads a feed as a calendar app does: no credential, the host the
// address names.
func fetchFeed(t *testing.T, f *apiFixture, path string, header http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = f.hostA
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCalendarFeedOverHTTP(t *testing.T) {
	f := newFixture(t)
	calendarMember(t, f)
	due := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	resp := f.call(http.MethodPost, wire.RouteTasks, core.CreateTaskInput{
		ProjectRef: "infra", Title: "renew, the cert", DueAt: &due, AssigneeActorID: f.actorA.ID,
	})
	mustStatus(t, resp, http.StatusCreated)
	var task core.Task
	decodeBody(t, resp, &task)

	feed := createFeed(t, f, "")
	u, err := url.Parse(feed.URL)
	if err != nil || u.Host != f.hostA || u.Path != feed.Path || !strings.HasSuffix(feed.Path, ".ics") ||
		!strings.Contains(feed.Path, feed.Token) {
		t.Fatalf("issued url %q, path %q", feed.URL, feed.Path)
	}

	resp = f.call(http.MethodGet, wire.RouteCalendar, nil)
	mustStatus(t, resp, http.StatusOK)
	if body := readBody(t, resp); strings.Contains(body, feed.Token) || strings.Contains(body, "token") {
		t.Fatalf("GET /calendar disclosed the secret: %s", body)
	}

	resp = fetchFeed(t, f, feed.Path, nil)
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "text/calendar; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("the feed may be sniffed as a page: %q", resp.Header.Get("X-Content-Type-Options"))
	}
	etag := resp.Header.Get("ETag")
	body := readBody(t, resp)
	if !strings.HasPrefix(etag, `"`) || strings.HasPrefix(etag, "W/") {
		t.Fatalf("ETag %q is not a strong tag", etag)
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "DTSTART;VALUE=DATE:20261105\r\n",
		"SUMMARY:" + strings.ToUpper(task.Ref) + ": renew\\, the cert\r\n",
		"URL:http://" + f.hostA + "/tasks/" + task.Ref + "\r\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("feed lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "private") {
		t.Errorf("Cache-Control = %q, want private", resp.Header.Get("Cache-Control"))
	}

	resp = fetchFeed(t, f, feed.Path, http.Header{"If-None-Match": {etag}})
	if resp.StatusCode != http.StatusNotModified || readBody(t, resp) != "" {
		t.Fatalf("a matching If-None-Match answered %d", resp.StatusCode)
	}
	resp = fetchFeed(t, f, feed.Path, http.Header{"If-None-Match": {`"stale"`}})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	if strings.Contains(f.logs.String(), feed.Token) {
		t.Fatal("the request log carries the feed secret")
	}

	resp = f.call(http.MethodPost, wire.RouteCalendarRotate, nil)
	mustStatus(t, resp, http.StatusOK)
	var rotated core.IssuedCalendarFeed
	decodeBody(t, resp, &rotated)
	resp = fetchFeed(t, f, feed.Path, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	resp = fetchFeed(t, f, rotated.Path, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	resp = f.call(http.MethodDelete, wire.RouteCalendar, nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	resp = fetchFeed(t, f, rotated.Path, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
}

func TestCalendarFeedRefusals(t *testing.T) {
	f := newFixture(t)
	calendarMember(t, f)
	feed := createFeed(t, f, "")

	resp := f.call(http.MethodPost, wire.RouteCalendar, core.CalendarFeedInput{})
	mustStatus(t, resp, http.StatusConflict)
	_ = resp.Body.Close()
	resp = f.call(http.MethodPatch, wire.RouteCalendar, core.CalendarFeedInput{Filter: "project:nosuch"})
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	for name, path := range map[string]string{
		"no suffix":    strings.TrimSuffix(feed.Path, ".ics"),
		"malformed":    wire.CalendarFeedPath("tix_cal_short"),
		"an api token": wire.CalendarFeedPath(f.tokenA),
	} {
		resp := fetchFeed(t, f, path, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %s answered %d, want 404", name, path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	resp = fetchFeed(t, f, feed.Path+"/x", nil)
	if resp.StatusCode == http.StatusOK {
		t.Error("a path below a feed address answered as the feed")
	}
	_ = resp.Body.Close()

	resp = f.do(http.MethodGet, feed.Path, f.hostB, "", nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	resp = f.do(http.MethodGet, wire.RouteCalendar, f.hostA, "", nil)
	mustStatus(t, resp, http.StatusUnauthorized)
	_ = resp.Body.Close()
}
