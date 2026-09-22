package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// liveOn registers a connection on the fixture's registry and reports the
// reason it was closed with, which stays empty until something ends it.
func liveOn(f *apiFixture, tenantID, actorID string, surface core.ConnectionSurface, closed *string) *connections.Handle {
	return f.live.Register(connections.Entry{
		Surface: surface, TenantID: tenantID, ActorID: actorID, ActorHandle: actorID, Remote: "203.0.113.11",
	}, func(reason string) error {
		*closed = reason
		return nil
	})
}

func TestConnectionRouteListsThisServersConnections(t *testing.T) {
	f := newFixture(t)
	var closed string
	liveOn(f, f.tenantA.ID, f.actorA.ID, core.ConnectionEvents, &closed)

	resp := f.call(http.MethodGet, httpapi.RouteConnections, nil)
	mustStatus(t, resp, http.StatusOK)
	var got core.ConnectionList
	decodeBody(t, resp, &got)

	if got.ServerID != "srv-test" {
		t.Errorf("the response does not name the server that answered: %q", got.ServerID)
	}
	if len(got.Connections) != 1 || got.Connections[0].Surface != core.ConnectionEvents {
		t.Fatalf("listed %+v", got.Connections)
	}
	if got.Counts.Events != 1 || got.Counts.Tenant != 1 || got.Counts.Process != 1 {
		t.Errorf("counts = %+v", got.Counts)
	}
}

func TestConnectionRouteAnswersEmptyWhenNothingIsLive(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodGet, httpapi.RouteConnections, nil)
	mustStatus(t, resp, http.StatusOK)
	var got core.ConnectionList
	decodeBody(t, resp, &got)
	if len(got.Connections) != 0 {
		t.Errorf("an idle server listed %+v", got.Connections)
	}
}

func TestConnectionRouteEndsOneConnection(t *testing.T) {
	f := newFixture(t)
	var ended, spared string
	target := liveOn(f, f.tenantA.ID, f.actorA.ID, core.ConnectionEvents, &ended)
	other := liveOn(f, f.tenantA.ID, f.actorA.ID, core.ConnectionSSH, &spared)

	resp := f.call(http.MethodDelete, strings.Replace(httpapi.RouteConnection, "{id}", target.ID(), 1), nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()

	if ended == "" {
		t.Error("the named connection was not closed")
	}
	if spared != "" {
		t.Errorf("another connection was closed with %q", spared)
	}
	if _, ok := f.live.Lookup(f.tenantA.ID, other.ID()); !ok {
		t.Error("a connection nobody named is gone")
	}
}

func TestConnectionRouteReportsAnUnknownIdentifierAsNotFound(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodDelete, strings.Replace(httpapi.RouteConnection, "{id}", "01JNOTHING", 1), nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
}

// Another tenant's identifier is not found over HTTP too, and the body does
// not disclose that it exists.
func TestConnectionRouteHidesAnotherTenantsConnection(t *testing.T) {
	f := newFixture(t)
	var theirs string
	other := liveOn(f, f.tenantB.ID, f.actorB.ID, core.ConnectionSSH, &theirs)

	resp := f.call(http.MethodGet, httpapi.RouteConnections, nil)
	mustStatus(t, resp, http.StatusOK)
	var got core.ConnectionList
	decodeBody(t, resp, &got)
	if len(got.Connections) != 0 {
		t.Errorf("another tenant's connections are listed: %+v", got.Connections)
	}
	if got.Counts.Tenant != 0 || got.Counts.SSH != 0 {
		t.Errorf("the tenant counts include another tenant's connections: %+v", got.Counts)
	}
	if got.Counts.Process != 1 {
		t.Errorf("process total = %d, want every connection the server holds", got.Counts.Process)
	}

	resp = f.call(http.MethodDelete, strings.Replace(httpapi.RouteConnection, "{id}", other.ID(), 1), nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("ending another tenant's connection = %d, want 404", resp.StatusCode)
	}
	body := readBody(t, resp)
	for _, secret := range []string{f.tenantB.ID, f.actorB.ID, "forbidden"} {
		if strings.Contains(body, secret) {
			t.Errorf("the refusal discloses %q: %s", secret, body)
		}
	}
	if theirs != "" {
		t.Error("another tenant's connection was closed")
	}
}

func TestConnectionRoutesRequireACredential(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodGet, httpapi.RouteConnections, f.hostA, "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("listing without a credential = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}
