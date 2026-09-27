// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// newProvisioner returns a provisioner over a real temporary database.
func newProvisioner(t *testing.T, opts ...func(*provisioner)) (*provisioner, *clock.Fake, store.Store) {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "demo.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	svc := service.New(st, service.WithClock(clk))
	t.Cleanup(func() { _ = svc.Close() })

	p := &provisioner{
		store:      st,
		service:    svc,
		clk:        clk,
		maxTenants: DefaultMaxTenants,
		leaseTTL:   DefaultLeaseTTL,
		tenantTTL:  DefaultTenantTTL,
	}
	for _, o := range opts {
		o(p)
	}
	return p, clk, st
}

// sessionContext is the context a session runs every call in: its own actor,
// and nothing else.
func sessionContext(actor *core.Actor) context.Context {
	return core.WithSource(core.WithActor(context.Background(), actor), core.SourceTUI)
}

func TestFirstConnectionSeedsASandbox(t *testing.T) {
	p, _, _ := newProvisioner(t)
	ctx := context.Background()

	actor, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}
	if actor.TenantID == "" || actor.Handle != visitorHandle {
		t.Fatalf("actor = %+v", actor)
	}
	if actor.Role != "" {
		t.Errorf("visitor role = %q, want an explicit scope grant and no role", actor.Role)
	}
	for _, denied := range []core.Scope{
		core.ScopeTenantAdmin, core.ScopeTokenAdmin, core.ScopeUserAdmin,
		core.ScopeWebhookAdmin, core.ScopeSyncAdmin, core.ScopeImport,
	} {
		if actor.HasScope(denied) {
			t.Errorf("visitor holds %q, which reaches outside the sandbox", denied)
		}
	}
	for _, granted := range []core.Scope{
		core.ScopeTaskWrite, core.ScopeTaskClaim, core.ScopeProjectWrite, core.ScopeWorkflowWrite,
	} {
		if !actor.HasScope(granted) {
			t.Errorf("visitor lacks %q, so the demo cannot show the product", granted)
		}
	}

	page, err := p.service.ListTasks(sessionContext(actor), core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) == 0 {
		t.Fatal("a fresh sandbox has no tasks, so the visitor lands on an empty board")
	}
}

func TestReturningFingerprintKeepsItsSandbox(t *testing.T) {
	p, clk, _ := newProvisioner(t)
	ctx := context.Background()

	first, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("first connection: %v", err)
	}
	before, err := p.service.ListTasks(sessionContext(first), core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if _, err := p.service.CreateTask(sessionContext(first), core.CreateTaskInput{
		ProjectRef: seedProjectKey, Title: "mine",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	clk.Advance(time.Hour)
	second, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("second connection: %v", err)
	}
	if second.TenantID != first.TenantID {
		t.Fatalf("returning key got tenant %q, want its own %q", second.TenantID, first.TenantID)
	}
	after, err := p.service.ListTasks(sessionContext(second), core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(after.Tasks) != len(before.Tasks)+1 {
		t.Fatalf("returning key sees %d tasks, want the %d it left plus one: a reseed, not a lookup",
			len(after.Tasks), len(before.Tasks)+1)
	}
}

func TestConnectionSlidesTheLastSeenTime(t *testing.T) {
	p, clk, st := newProvisioner(t)
	ctx := context.Background()

	actor, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("first connection: %v", err)
	}
	clk.Advance(5 * time.Hour)
	if _, err := p.ActorByFingerprint(ctx, "SHA256:aaa"); err != nil {
		t.Fatalf("second connection: %v", err)
	}

	// Four hours after a visit five hours in, the sandbox has been seen within
	// the six hour life, so counting from creation would wrongly reap it.
	clk.Advance(4 * time.Hour)
	n, err := reapOnce(ctx, st, clk.Now(), DefaultTenantTTL)
	if err != nil {
		t.Fatalf("reapOnce: %v", err)
	}
	if n != 0 {
		t.Fatalf("reaped %d sandboxes, want none: the visitor came back an hour ago", n)
	}
	if _, err := p.service.ListTasks(sessionContext(actor), core.TaskFilter{}); err != nil {
		t.Fatalf("the sandbox is gone: %v", err)
	}
}

func TestReaperDeletesAnUnvisitedSandboxAndEverythingUnderIt(t *testing.T) {
	p, clk, st := newProvisioner(t)
	ctx := context.Background()

	actor, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}
	tenantID := actor.TenantID

	clk.Advance(DefaultTenantTTL + time.Minute)
	n, err := reapOnce(ctx, st, clk.Now(), DefaultTenantTTL)
	if err != nil {
		t.Fatalf("reapOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("reaped %d sandboxes, want 1", n)
	}

	scope := core.TenantScope{TenantID: tenantID}
	for _, table := range []string{"tasks", "projects", "actors", "workflows"} {
		if got := countIn(t, st, scope, table); got != 0 {
			t.Errorf("%d %s rows survived the tenant, so the delete does not cascade", got, table)
		}
	}

	// A reaped fingerprint coming back is a new visitor, not an error.
	again, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("reconnecting after a reap: %v", err)
	}
	if again.TenantID == tenantID {
		t.Error("the reaped tenant came back, so it was not deleted")
	}
}

func TestReaperLeavesTenantsItDoesNotOwn(t *testing.T) {
	p, clk, st := newProvisioner(t)
	ctx := context.Background()

	real := &core.Tenant{Key: "acme", Name: "Acme"}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, real)
	}); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	if _, err := p.ActorByFingerprint(ctx, "SHA256:aaa"); err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}

	clk.Advance(DefaultTenantTTL + time.Hour)
	if _, err := reapOnce(ctx, st, clk.Now(), DefaultTenantTTL); err != nil {
		t.Fatalf("reapOnce: %v", err)
	}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		_, err := u.GetTenantByKey(ctx, "acme")
		return err
	}); err != nil {
		t.Fatalf("the reaper deleted a tenant it does not own: %v", err)
	}
}

func TestTenantCapRefusesANewFingerprintWithoutEvicting(t *testing.T) {
	p, _, _ := newProvisioner(t, func(p *provisioner) { p.maxTenants = 2 })
	ctx := context.Background()

	first, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := p.ActorByFingerprint(ctx, "SHA256:bbb"); err != nil {
		t.Fatalf("second: %v", err)
	}

	_, err = p.ActorByFingerprint(ctx, "SHA256:ccc")
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("a third fingerprint at the cap returned %v, want a precondition failure", err)
	}

	// The refusal must cost nobody their board.
	back, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("the first visitor cannot get back in: %v", err)
	}
	if back.TenantID != first.TenantID {
		t.Fatal("the first visitor's sandbox was evicted to admit a stranger")
	}
}

func TestTaskCapRefusesTheCreateThatWouldPassIt(t *testing.T) {
	p, _, _ := newProvisioner(t)
	ctx := context.Background()
	actor, err := p.ActorByFingerprint(ctx, "SHA256:aaa")
	if err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}
	sess := sessionContext(actor)
	page, err := p.service.ListTasks(sess, core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	capped := capped{Service: p.service, limit: len(page.Tasks) + 1}

	if _, err := capped.CreateTask(sess, core.CreateTaskInput{
		ProjectRef: seedProjectKey, Title: "the last one that fits",
	}); err != nil {
		t.Fatalf("CreateTask below the cap: %v", err)
	}
	_, err = capped.CreateTask(sess, core.CreateTaskInput{
		ProjectRef: seedProjectKey, Title: "one too many",
	})
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("CreateTask at the cap returned %v, want a precondition failure", err)
	}
}

// countIn counts the rows of one table inside a tenant, which is how a
// cascade is checked without trusting the thing being checked.
func countIn(t *testing.T, st store.Store, scope core.TenantScope, table string) int {
	t.Helper()
	ctx := context.Background()
	var n int
	if err := st.View(ctx, scope, func(tx store.Tx) error {
		switch table {
		case "tasks":
			rows, err := tx.ListTasks(ctx, core.TaskFilter{IncludeDeleted: true})
			n = len(rows)
			return err
		case "projects":
			rows, err := tx.ListProjects(ctx, core.ProjectFilter{})
			n = len(rows)
			return err
		case "actors":
			rows, err := tx.ListActors(ctx, core.Page{})
			n = len(rows)
			return err
		case "workflows":
			rows, err := tx.ListWorkflows(ctx)
			n = len(rows)
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}

func TestTenantKeyIsAStableFunctionOfTheFingerprint(t *testing.T) {
	first := tenantKeyFor("SHA256:aaa")
	if first != tenantKeyFor("SHA256:aaa") {
		t.Fatal("the same fingerprint produced two tenant keys, so a return visit reseeds")
	}
	if first == tenantKeyFor("SHA256:bbb") {
		t.Fatal("two fingerprints share a tenant key")
	}
	if !isSandbox(first) {
		t.Fatalf("tenant key %q is not recognised as a sandbox", first)
	}
	if isSandbox("acme") {
		t.Error("an ordinary tenant key was taken for a sandbox")
	}
	if err := core.ValidateProjectKey(first); err != nil {
		t.Fatalf("tenant key %q is not a valid key: %v", first, err)
	}
}

func TestShortRendersDurationsForASentence(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{6 * time.Hour, "6h"},
		{90 * time.Minute, "90m"},
		{2 * time.Minute, "2m"},
		{90 * time.Second, "1m30s"},
	}
	for _, tc := range tests {
		if got := short(tc.in); got != tc.want {
			t.Errorf("short(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMessageHidesAnInternalFailureFromAStranger(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"a domain refusal is explained", core.Precondition("this demo is full"), "this demo is full"},
		{"an internal failure is not", core.Internal("the disk is gone"),
			"tix could not open a session for you; try again shortly"},
		{"nor is an unclassified one", errors.New("raw"),
			"tix could not open a session for you; try again shortly"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := message(tc.err); got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
}

// numericOption is one of withDefaults' numbered limits, named once so the
// table below covers the whole list rather than the fields somebody happened
// to write down. Durations travel as their nanosecond count, which is what
// lets one table hold both.
type numericOption struct {
	name     string
	fallback int64
	// configured is a value nothing else in the table would produce, so a
	// default overwriting it is visible.
	configured int64
	get        func(Options) int64
	set        func(*Options, int64)
}

// numericOptions is every limit withDefaults fills. A field missing here is a
// field whose default nothing checks, which is the gap this table exists to
// close: add the option, add the row.
func numericOptions() []numericOption {
	return []numericOption{
		{
			"TenantTTL", int64(DefaultTenantTTL), int64(90 * time.Minute),
			func(o Options) int64 { return int64(o.TenantTTL) },
			func(o *Options, v int64) { o.TenantTTL = time.Duration(v) },
		},
		{
			"ReapInterval", int64(DefaultReapInterval), int64(7 * time.Minute),
			func(o Options) int64 { return int64(o.ReapInterval) },
			func(o *Options, v int64) { o.ReapInterval = time.Duration(v) },
		},
		{
			"MaxTenants", int64(DefaultMaxTenants), 11,
			func(o Options) int64 { return int64(o.MaxTenants) },
			func(o *Options, v int64) { o.MaxTenants = int(v) },
		},
		{
			"MaxTasks", int64(DefaultMaxTasks), 13,
			func(o Options) int64 { return int64(o.MaxTasks) },
			func(o *Options, v int64) { o.MaxTasks = int(v) },
		},
		{
			"LeaseTTL", int64(DefaultLeaseTTL), int64(17 * time.Second),
			func(o Options) int64 { return int64(o.LeaseTTL) },
			func(o *Options, v int64) { o.LeaseTTL = time.Duration(v) },
		},
		{
			"RatePerHour", int64(DefaultRatePerHour), 19,
			func(o Options) int64 { return int64(o.RatePerHour) },
			func(o *Options, v int64) { o.RatePerHour = int(v) },
		},
		{
			"RateBurst", int64(DefaultRateBurst), 23,
			func(o Options) int64 { return int64(o.RateBurst) },
			func(o *Options, v int64) { o.RateBurst = int(v) },
		},
		{
			"IdleTimeout", int64(DefaultIdleTimeout), int64(29 * time.Minute),
			func(o Options) int64 { return int64(o.IdleTimeout) },
			func(o *Options, v int64) { o.IdleTimeout = time.Duration(v) },
		},
		{
			"KeepaliveInterval", int64(DefaultKeepaliveInterval), int64(31 * time.Second),
			func(o Options) int64 { return int64(o.KeepaliveInterval) },
			func(o *Options, v int64) { o.KeepaliveInterval = time.Duration(v) },
		},
		{
			"KeepaliveMaxMissed", int64(DefaultKeepaliveMaxMissed), 7,
			func(o Options) int64 { return int64(o.KeepaliveMaxMissed) },
			func(o *Options, v int64) { o.KeepaliveMaxMissed = int(v) },
		},
		{
			"MaxSessions", int64(DefaultMaxSessions), 41,
			func(o Options) int64 { return int64(o.MaxSessions) },
			func(o *Options, v int64) { o.MaxSessions = int(v) },
		},
		{
			"MaxSessionsPerKey", int64(DefaultMaxSessionsPerKey), 9,
			func(o Options) int64 { return int64(o.MaxSessionsPerKey) },
			func(o *Options, v int64) { o.MaxSessionsPerKey = int(v) },
		},
	}
}

// TestEveryNumberedLimitDefaultsAndIsKeptWhenConfigured runs the whole list of
// limits through withDefaults three ways each: unset takes the default, a
// nonsense value takes the default, and a configured value is left alone.
//
// One table over every field rather than a test per field, because the failure
// this guards against is a field nobody added: a defaults test that covers
// some of the list and reads as though it covered the list.
func TestEveryNumberedLimitDefaultsAndIsKeptWhenConfigured(t *testing.T) {
	for _, tc := range numericOptions() {
		t.Run(tc.name+"/unset takes the default", func(t *testing.T) {
			if got := tc.get(Options{}.withDefaults()); got != tc.fallback {
				t.Fatalf("%s = %d when unset, want the default %d", tc.name, got, tc.fallback)
			}
		})
		t.Run(tc.name+"/a negative value takes the default", func(t *testing.T) {
			var o Options
			tc.set(&o, -1)
			if got := tc.get(o.withDefaults()); got != tc.fallback {
				t.Fatalf("%s = %d when set to -1, want the default %d", tc.name, got, tc.fallback)
			}
		})
		t.Run(tc.name+"/a configured value is kept", func(t *testing.T) {
			var o Options
			tc.set(&o, tc.configured)
			if got := tc.get(o.withDefaults()); got != tc.configured {
				t.Fatalf("%s = %d, want the configured %d: a default overwrote it",
					tc.name, got, tc.configured)
			}
		})
	}
}

// TestEveryUnsetDependencyIsFilled covers what withDefaults supplies that is
// not a number: the collaborators a listener cannot run without, and the
// address it binds.
func TestEveryUnsetDependencyIsFilled(t *testing.T) {
	o := Options{}.withDefaults()
	if o.Clock == nil {
		t.Error("Clock is nil, so every timer in the listener panics")
	}
	if o.Logger == nil {
		t.Error("Logger is nil")
	}
	if o.Connections == nil {
		t.Error("Connections is nil, so no session is visible to an administrator")
	}
	if o.Addr != DefaultAddr {
		t.Errorf("Addr = %q when unset, want %q", o.Addr, DefaultAddr)
	}
	configured := Options{Addr: "127.0.0.1:2323"}.withDefaults()
	if configured.Addr != "127.0.0.1:2323" {
		t.Errorf("Addr = %q, want the configured address", configured.Addr)
	}
}

// TestThePerKeyCapNeverExceedsTheListenerCap guards the one limit that is
// decided from another: a key may not be entitled to more sessions than the
// listener has.
func TestThePerKeyCapNeverExceedsTheListenerCap(t *testing.T) {
	tests := []struct {
		name       string
		perKey     int
		max        int
		wantPerKey int
	}{
		{"a per-key cap above the listener's is clamped to it", 9, 2, 2},
		{"an unset per-key cap is clamped too", 0, 2, 2},
		{"a per-key cap equal to the listener's stands", 4, 4, 4},
		{"a per-key cap below the listener's is untouched", 2, 4, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := Options{MaxSessionsPerKey: tc.perKey, MaxSessions: tc.max}.withDefaults()
			if o.MaxSessionsPerKey != tc.wantPerKey {
				t.Fatalf("MaxSessionsPerKey = %d, want %d", o.MaxSessionsPerKey, tc.wantPerKey)
			}
			if o.MaxSessions != tc.max {
				t.Fatalf("MaxSessions = %d, want the configured %d", o.MaxSessions, tc.max)
			}
		})
	}
}

func TestNewRejectsAnIncompleteConfiguration(t *testing.T) {
	p, _, st := newProvisioner(t)
	tests := []struct {
		name string
		opts Options
	}{
		{"no service", Options{Store: st}},
		{"no store", Options{Service: p.service}},
		{"no host key path", Options{Service: p.service, Store: st}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.opts); err == nil {
				t.Fatal("New accepted it")
			}
		})
	}
}

func TestSeededWorkflowCarriesTheShortLease(t *testing.T) {
	p, _, _ := newProvisioner(t, func(p *provisioner) { p.leaseTTL = 90 * time.Second })
	actor, err := p.ActorByFingerprint(context.Background(), "SHA256:aaa")
	if err != nil {
		t.Fatalf("ActorByFingerprint: %v", err)
	}
	flow, err := p.service.GetWorkflow(sessionContext(actor), builtinWorkflowKey)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got := time.Duration(flow.Definition.DefaultLease); got != 90*time.Second {
		t.Fatalf("seeded lease = %v, want the configured 90s", got)
	}
}
