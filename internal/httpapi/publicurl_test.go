// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/wire"
)

// sharedHost is a name no domain claims, so it serves the default tenant the
// way a server reached by an internal name or a proxy's backend address does.
const sharedHost = "tix.lan"

const publicURL = "https://tix.example.com/tix"

// newPublicFixture serves tenant A by default, on a server configured with
// server.public_url, or with none when public is empty.
func newPublicFixture(t *testing.T, public string) *apiFixture {
	t.Helper()
	return newFixtureOver(t, []service.Option{service.WithPublicURL(public)},
		func(f *apiFixture, cfg *httpapi.Config) {
			cfg.PublicURL = public
			cfg.DefaultTenantID = f.tenantA.ID
		})
}

// taskJSON reads one task's raw response, so an absent field is told from an
// empty one.
func taskJSON(f *apiFixture, method, path, host string, body any) string {
	f.t.Helper()
	resp := f.do(method, path, host, f.tokenA, body)
	if resp.StatusCode/100 != 2 {
		f.t.Fatalf("%s %s on %s: status %d: %s", method, path, host, resp.StatusCode, readBody(f.t, resp))
	}
	return readBody(f.t, resp)
}

func TestTaskResponsesCarryTheirWebAddress(t *testing.T) {
	f := newPublicFixture(t, publicURL)
	created := taskJSON(f, http.MethodPost, wire.RouteTasks, sharedHost, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	want := `"web_url":"https://tix.example.com/tix/tasks/infra-1"`
	if !strings.Contains(created, want) {
		t.Fatalf("a created task on a shared host carries no %s:\n%s", want, created)
	}
	for _, path := range []string{"/api/v1/tasks/infra-1", wire.RouteTasks + "?project=infra"} {
		if body := taskJSON(f, http.MethodGet, path, sharedHost, nil); !strings.Contains(body, want) {
			t.Errorf("GET %s on a shared host carries no %s:\n%s", path, want, body)
		}
	}
	own := taskJSON(f, http.MethodGet, "/api/v1/tasks/infra-1", f.hostA, nil)
	if want := `"web_url":"https://acme.test/tix/tasks/infra-1"`; !strings.Contains(own, want) {
		t.Errorf("on the tenant's own domain the task carries no %s:\n%s", want, own)
	}
}

func TestTaskResponsesOmitTheWebAddressWithoutAPublicURL(t *testing.T) {
	f := newPublicFixture(t, "")
	created := taskJSON(f, http.MethodPost, wire.RouteTasks, sharedHost, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	listed := taskJSON(f, http.MethodGet, wire.RouteTasks, f.hostA, nil)
	for _, body := range []string{created, listed} {
		if strings.Contains(body, "web_url") {
			t.Fatalf("a server without server.public_url answered web_url:\n%s", body)
		}
	}
}

// The addresses the API builds from a request use the public address on a
// shared host, and the request's own host on a tenant's domain or with no
// public address at all.
func TestRequestLinksPreferThePublicAddressOnASharedHost(t *testing.T) {
	cases := []struct {
		name, public, host, want string
	}{
		{"shared host", publicURL, sharedHost, publicURL + "/"},
		{"own domain", publicURL, "acme.test", "http://acme.test/"},
		{"unset", "", sharedHost, "http://" + sharedHost + "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPublicFixture(t, tc.public)
			calendarMember(t, f)
			resp := f.do(http.MethodPost, wire.RouteInvites, tc.host, f.tokenA,
				core.CreateInviteInput{Email: "ada@example.com", Role: core.RoleMember})
			mustStatus(t, resp, http.StatusCreated)
			var invite core.IssuedInvite
			decodeBody(t, resp, &invite)
			if !strings.HasPrefix(invite.URL, tc.want+"invite/") {
				t.Errorf("invite url %q, want it under %q", invite.URL, tc.want)
			}
			resp = f.do(http.MethodPost, wire.RouteCalendar, tc.host, f.tokenA, core.CalendarFeedInput{})
			mustStatus(t, resp, http.StatusCreated)
			var feed core.IssuedCalendarFeed
			decodeBody(t, resp, &feed)
			if !strings.HasPrefix(feed.URL, tc.want+strings.TrimPrefix(wire.CalendarFeedPrefix, "/")) {
				t.Errorf("feed url %q, want it under %q", feed.URL, tc.want)
			}
		})
	}
}
