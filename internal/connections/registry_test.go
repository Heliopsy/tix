// SPDX-License-Identifier: AGPL-3.0-or-later

package connections_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
)

// newRegistry builds a registry on a stopped clock, so a connection's start is
// whatever the test says it is.
func newRegistry(t *testing.T) *connections.Registry {
	t.Helper()
	return connections.New(connections.WithClock(clock.NewFakeAt()), connections.WithServerID("srv-test"))
}

// register adds one connection that records the reason it was closed with.
func register(r *connections.Registry, tenant, actor string, surface core.ConnectionSurface, closed *string) *connections.Handle {
	var mu sync.Mutex
	return r.Register(connections.Entry{
		Surface: surface, TenantID: tenant, ActorID: actor, ActorHandle: actor, Remote: "203.0.113.7",
	}, func(reason string) error {
		mu.Lock()
		defer mu.Unlock()
		if closed != nil {
			*closed = reason
		}
		return nil
	})
}

func TestRegisteredConnectionIsListedAndCounted(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	h := register(r, "t1", "a1", core.ConnectionEvents, nil)

	got := r.List("t1")
	if len(got) != 1 {
		t.Fatalf("List = %d connections, want 1", len(got))
	}
	if got[0].ID != h.ID() || got[0].Surface != core.ConnectionEvents || got[0].ActorID != "a1" {
		t.Errorf("listed connection = %+v", got[0])
	}
	if got[0].Since.IsZero() {
		t.Error("a listed connection does not say when it opened")
	}
	counts := r.Counts("t1")
	if counts != (core.ConnectionCounts{Events: 1, Tenant: 1, Process: 1}) {
		t.Errorf("Counts = %+v", counts)
	}
}

func TestUnregisteredConnectionLeavesWithoutBeingRemovedByHand(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	h := register(r, "t1", "a1", core.ConnectionEvents, nil)
	h.Unregister()
	h.Unregister()

	if got := r.List("t1"); len(got) != 0 {
		t.Errorf("a closed connection is still listed: %+v", got)
	}
	if r.Len() != 0 {
		t.Errorf("Len = %d after the only connection left", r.Len())
	}
	if _, ok := r.Lookup("t1", h.ID()); ok {
		t.Error("a closed connection is still addressable")
	}
}

func TestNothingLiveListsEmptyRatherThanFailing(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	if got := r.List("t1"); got == nil || len(got) != 0 {
		t.Errorf("List with nothing live = %#v, want an empty list", got)
	}
	if counts := r.Counts("t1"); counts != (core.ConnectionCounts{}) {
		t.Errorf("Counts with nothing live = %+v", counts)
	}
}

func TestEndClosesOneConnectionAndOnlyThatOne(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	var first, second string
	target := register(r, "t1", "a1", core.ConnectionEvents, &first)
	other := register(r, "t1", "a2", core.ConnectionSSH, &second)

	if err := r.End("t1", target.ID(), "ended"); err != nil {
		t.Fatalf("End: %v", err)
	}
	if first != "ended" {
		t.Errorf("the ended connection was closed with %q", first)
	}
	if second != "" {
		t.Errorf("another connection was closed with %q", second)
	}
	if _, ok := r.Lookup("t1", other.ID()); !ok {
		t.Error("the connection that was not named is gone")
	}
	if r.Len() != 1 {
		t.Errorf("Len = %d after ending one of two", r.Len())
	}
}

func TestEndSurfacesACloseThatFailed(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	h := r.Register(connections.Entry{Surface: core.ConnectionSSH, TenantID: "t1", ActorID: "a1"},
		func(string) error { return fmt.Errorf("socket already gone") })

	if err := r.End("t1", h.ID(), "ended"); err == nil || !strings.Contains(err.Error(), "socket already gone") {
		t.Errorf("End = %v, want the closer's failure", err)
	}
	if r.Len() != 0 {
		t.Error("a connection whose close failed is still registered")
	}
}

func TestEndIgnoresAConnectionThatHasAlreadyGone(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	if err := r.End("t1", "nothing-here", "ended"); err != nil {
		t.Errorf("ending an absent connection = %v, want no error", err)
	}
}

// Another tenant's connection is absent, never refused: a refusal would
// confirm that the identifier names something.
func TestAnotherTenantsConnectionIsInvisibleAndUnendable(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	var closed string
	theirs := register(r, "t2", "a9", core.ConnectionEvents, &closed)
	register(r, "t1", "a1", core.ConnectionEvents, nil)

	for _, got := range r.List("t1") {
		if got.TenantID != "t1" {
			t.Errorf("another tenant's connection is listed: %+v", got)
		}
	}
	if _, ok := r.Lookup("t1", theirs.ID()); ok {
		t.Error("another tenant's identifier resolved")
	}
	if err := r.End("t1", theirs.ID(), "ended"); err != nil {
		t.Errorf("ending another tenant's connection = %v", err)
	}
	if closed != "" {
		t.Error("another tenant's connection was closed")
	}
	if _, ok := r.Lookup("t2", theirs.ID()); !ok {
		t.Error("another tenant's connection was removed")
	}
}

// The process total counts every connection and breaks nothing down.
func TestProcessTotalCoversEveryTenantAndTheTenantCountsDoNot(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	register(r, "t1", "a1", core.ConnectionEvents, nil)
	register(r, "t2", "a2", core.ConnectionEvents, nil)
	register(r, "t2", "a3", core.ConnectionSSH, nil)

	counts := r.Counts("t1")
	if counts.Process != 3 {
		t.Errorf("process total = %d, want 3", counts.Process)
	}
	if counts.Tenant != 1 || counts.Events != 1 || counts.SSH != 0 {
		t.Errorf("tenant counts = %+v, want only this tenant's one event stream", counts)
	}
}

func TestAnEmptyTenantSeesNothingButTheProcessTotal(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	register(r, "t2", "a2", core.ConnectionEvents, nil)
	if got := r.List(""); len(got) != 0 {
		t.Errorf("an empty tenant listed %d connections", len(got))
	}
	if counts := r.Counts(""); counts.Tenant != 0 || counts.Process != 1 {
		t.Errorf("Counts(\"\") = %+v", counts)
	}
	if _, ok := r.Lookup("", "anything"); ok {
		t.Error("an empty tenant resolved an identifier")
	}
}

func TestServerIDNamesTheProcess(t *testing.T) {
	t.Parallel()
	if got := newRegistry(t).ServerID(); got != "srv-test" {
		t.Errorf("ServerID = %q", got)
	}
	if got := connections.New().ServerID(); strings.TrimSpace(got) == "" {
		t.Error("a registry with no configured name answers with nothing")
	}
}

// Registering, listing and ending happen on different goroutines, so the whole
// surface is exercised at once under -race.
func TestRegistryIsSafeUnderConcurrentUse(t *testing.T) {
	t.Parallel()
	r := newRegistry(t)
	const workers = 16

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tenant := fmt.Sprintf("t%d", n%3)
			h := register(r, tenant, fmt.Sprintf("a%d", n), core.ConnectionEvents, nil)
			r.List(tenant)
			r.Counts(tenant)
			r.Lookup(tenant, h.ID())
			if n%2 == 0 {
				_ = r.End(tenant, h.ID(), "ended")
				return
			}
			h.Unregister()
		}(i)
	}
	wg.Wait()

	if r.Len() != 0 {
		t.Errorf("Len = %d once every worker finished", r.Len())
	}
}

// A nil handle is what a feeder gets for a connection that speaks for nobody,
// and it must be safe to hold and release like any other.
func TestNilHandleIsInert(t *testing.T) {
	t.Parallel()
	var h *connections.Handle
	h.Unregister()
	if h.ID() != "" {
		t.Error("a nil handle names an identifier")
	}
}
