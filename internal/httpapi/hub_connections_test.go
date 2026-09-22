package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
)

// hubWithRegistry gives a hub its own registry, so one test never sees
// another's connections.
func hubWithRegistry(t *testing.T) (*Hub, *connections.Registry) {
	t.Helper()
	reg := connections.New(connections.WithClock(clock.NewFakeAt()), connections.WithServerID("srv-test"))
	h := NewHub()
	h.SetRegistry(reg)
	return h, reg
}

func TestHubRegistersAndUnregistersInTheConnectionRegistry(t *testing.T) {
	h, reg := hubWithRegistry(t)
	c := newConn(testActor("t1"), 4)
	c.remote = "203.0.113.5"

	h.Register(c)
	got := reg.List("t1")
	if len(got) != 1 {
		t.Fatalf("the hub registered %d connections, want 1", len(got))
	}
	if got[0].Surface != core.ConnectionEvents {
		t.Errorf("surface = %q, want %q", got[0].Surface, core.ConnectionEvents)
	}
	if got[0].ActorID != c.actor.ID || got[0].ActorHandle != c.actor.Handle {
		t.Errorf("registered connection does not name its actor: %+v", got[0])
	}
	if got[0].Remote != "203.0.113.5" {
		t.Errorf("remote = %q, want the address the upgrade came from", got[0].Remote)
	}
	if got[0].Fingerprint != "" {
		t.Errorf("an event-stream connection reported a key fingerprint %q", got[0].Fingerprint)
	}

	h.Unregister(c)
	if n := reg.Len(); n != 0 {
		t.Errorf("a closed connection left %d entries in the registry", n)
	}
}

// Ending a registered connection closes it through the hub's own failure path,
// so the stream tells its client why before the socket goes.
func TestEndingARegisteredConnectionFailsTheStream(t *testing.T) {
	h, reg := hubWithRegistry(t)
	c := newConn(testActor("t1"), 4)
	h.Register(c)
	defer h.Unregister(c)

	id := reg.List("t1")[0].ID
	if err := reg.End("t1", id, "ended by an administrator"); err != nil {
		t.Fatalf("End: %v", err)
	}
	select {
	case <-c.closed:
	default:
		t.Fatal("the ended connection was not closed")
	}
	if got := c.failure(); got != "ended by an administrator" {
		t.Errorf("the stream was closed with %q", got)
	}
}

// A connection speaking for nobody is not something an administrator can be
// shown or asked to end, so it is not registered at all.
func TestAConnectionWithoutATenantIsNotRegistered(t *testing.T) {
	h, reg := hubWithRegistry(t)
	c := newConn(&core.Actor{ID: "nobody"}, 4)
	h.Register(c)
	if reg.Len() != 0 {
		t.Errorf("a connection with no tenant was registered: %+v", reg.List(""))
	}
	h.Unregister(c)
}

func TestRemoteOfPrefersTheResolvedClientAddress(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	r.RemoteAddr = "192.0.2.9:54321"
	if got := remoteOf(r); got != "192.0.2.9" {
		t.Errorf("remoteOf = %q, want the peer without its port", got)
	}

	r.RemoteAddr = "unix"
	if got := remoteOf(r); got != "unix" {
		t.Errorf("remoteOf of an address with no port = %q", got)
	}
}

func TestSetRegistryIgnoresNothing(t *testing.T) {
	h, reg := hubWithRegistry(t)
	h.SetRegistry(nil)
	c := newConn(testActor("t1"), 4)
	h.Register(c)
	defer h.Unregister(c)
	if reg.Len() != 1 {
		t.Error("setting a nil registry dropped the one the hub had")
	}
}
