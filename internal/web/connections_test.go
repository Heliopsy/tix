package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
)

// liveOn registers a connection on the fixture's registry and reports the
// reason it was closed with, which stays empty until something ends it.
func liveOn(f *fixture, tenantID, actorID string, surface core.ConnectionSurface, closed *string) *connections.Handle {
	return f.live.Register(connections.Entry{
		Surface: surface, TenantID: tenantID, ActorID: actorID, ActorHandle: actorID,
		Remote: "203.0.113.12", Fingerprint: "SHA256:theirs",
	}, func(reason string) error {
		*closed = reason
		return nil
	})
}

func TestConnectionScreenShowsWhatThisServerHolds(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	var closed string
	liveOn(f, f.tenantA.ID, f.actorA.ID, core.ConnectionSSH, &closed)

	page := f.as("alice").page("/admin/connections")
	for _, want := range []string{"srv-test", "ssh", "alice", "203.0.113.12", "SHA256:theirs", "End"} {
		if !strings.Contains(page, want) {
			t.Errorf("the connection screen omits %q:\n%s", want, page)
		}
	}
	if !strings.Contains(page, "is not revocation") {
		t.Error("the screen does not say that ending is not revocation")
	}
}

func TestConnectionScreenExplainsAnEmptyServer(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/admin/connections")
	if !strings.Contains(page, "No live connections.") {
		t.Errorf("an idle server's screen does not say so:\n%s", page)
	}
}

func TestEndingAConnectionFromTheBrowserClosesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	var ended, spared string
	target := liveOn(f, f.tenantA.ID, f.actorA.ID, core.ConnectionEvents, &ended)
	other := liveOn(f, f.tenantA.ID, f.actorA.ID, core.ConnectionSSH, &spared)

	b := f.as("alice")
	resp := b.post("/admin/connections/end", url.Values{"id": {target.ID()}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	if ended == "" {
		t.Error("the named connection was not closed")
	}
	if spared != "" {
		t.Errorf("another connection was closed with %q", spared)
	}
	if _, ok := f.live.Lookup(f.tenantA.ID, other.ID()); !ok {
		t.Error("a connection nobody named is gone")
	}
	if page := b.page("/admin/connections"); strings.Contains(page, target.ID()) {
		t.Errorf("the ended connection is still on the screen:\n%s", page)
	}
}

// Another tenant's connections never reach the screen, and naming one is
// refused as not found rather than as forbidden.
func TestTheConnectionScreenIsScopedToTheSessionTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	var theirs string
	other := liveOn(f, f.tenantB.ID, f.actorB.ID, core.ConnectionSSH, &theirs)

	b := f.as("alice")
	page := b.page("/admin/connections")
	for _, secret := range []string{other.ID(), f.actorB.ID, "bob"} {
		if strings.Contains(page, secret) {
			t.Errorf("the screen discloses %q:\n%s", secret, page)
		}
	}
	if !strings.Contains(page, "holding 1 in total") {
		t.Errorf("the process total does not cover the other tenant's connection:\n%s", page)
	}

	resp := b.post("/admin/connections/end", url.Values{"id": {other.ID()}})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("ending another tenant's connection = %d, want 404", resp.StatusCode)
	}
	if theirs != "" {
		t.Error("another tenant's connection was closed from the browser")
	}
}
