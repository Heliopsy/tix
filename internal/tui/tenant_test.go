// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// typeText plays a string into an open input one key at a time.
func typeText(m Model, text string) Model {
	for _, r := range text {
		m, _ = m.reduce(pressKey(string(r)))
	}
	return m
}

// runCmd runs a command the model returned and folds its message back in.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command was returned")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		t.Fatalf("expected one message, got a batch of %d", len(batch))
	}
	next, _ := m.reduce(msg)
	return next
}

// dialerFor returns a dialer that always opens the given service, recording
// how many times it was asked and whether the connection was closed.
func dialerFor(svc core.Service, closed *int, keys *[]string) TenantDialer {
	return func(ctx context.Context, key string) (TenantConn, error) {
		*keys = append(*keys, key)
		return TenantConn{Service: svc, Context: ctx, Close: func() error {
			*closed++
			return nil
		}}, nil
	}
}

func TestTenantSwitchPlanRefusesWhatCannotBeSwitchedTo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, current, typed, want string
	}{
		{"empty", "default", "   ", "a tenant key is required"},
		{"the current tenant", "acme", "acme", `already on tenant "acme"`},
		{"the current tenant in another case", "acme", "ACME", "already on tenant"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key, err := TenantSwitchPlan(tc.current, tc.typed)
			if err == nil {
				t.Fatalf("TenantSwitchPlan(%q, %q) = %q, want a refusal", tc.current, tc.typed, key)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal = %v, want it to mention %q", err, tc.want)
			}
			if core.KindOf(err) != core.KindInvalid {
				t.Errorf("refusal kind = %v, want invalid", core.KindOf(err))
			}
		})
	}
	key, err := TenantSwitchPlan("default", "  acme  ")
	if err != nil || key != "acme" {
		t.Fatalf("TenantSwitchPlan = %q, %v, want the trimmed key", key, err)
	}
}

func TestTenantSwitchNoticeSaysWhatHappensToLeases(t *testing.T) {
	t.Parallel()
	if got := TenantSwitchNotice("acme", 0); got != "switched to tenant acme" {
		t.Errorf("notice with no leases = %q", got)
	}
	one := TenantSwitchNotice("acme", 1)
	if !strings.Contains(one, "1 lease") || !strings.Contains(one, "expire") {
		t.Errorf("notice with one lease = %q, want it to say the lease is dropped unreleased", one)
	}
	if got := TenantSwitchNotice("acme", 3); !strings.Contains(got, "3 leases") {
		t.Errorf("notice with three leases = %q", got)
	}
}

// The view cannot offer a list of tenants, so it has to say why rather than
// leaving a reader hunting for one.
func TestTenantViewSaysThereIsNoListToChooseFrom(t *testing.T) {
	t.Parallel()
	body := strings.Join(TenantViewLines("default", "ada", true), "\n")
	for _, want := range []string{"default", "ada", "no list", "one tenant"} {
		if !strings.Contains(body, want) {
			t.Errorf("tenant view does not mention %q:\n%s", want, body)
		}
	}
	readOnly := strings.Join(TenantViewLines("default", "ada", false), "\n")
	if !strings.Contains(readOnly, "cannot switch tenants") {
		t.Errorf("a session with no dialer did not say so:\n%s", readOnly)
	}
	if !strings.Contains(readOnly, "tix tenant use") {
		t.Errorf("a session with no dialer did not name the command that can:\n%s", readOnly)
	}
	unnamed := strings.Join(TenantViewLines("", "", true), "\n")
	if strings.Contains(unnamed, "current: \n") {
		t.Errorf("an unnamed tenant rendered blank:\n%s", unnamed)
	}
}

func TestTenantViewOpensAndPopsBack(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(pressKey("T"))
	if m.view != viewTenant {
		t.Fatalf("view = %v, want viewTenant", m.view)
	}
	if !strings.Contains(m.View(), "tenant") {
		t.Fatal("the tenant view does not name itself")
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view != viewBoard {
		t.Fatalf("esc from the tenant view landed on %v", m.view)
	}
}

func TestTenantViewWithoutADialerRefusesRatherThanPrompting(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(pressKey("T"))
	m, _ = m.reduce(pressKey("enter"))
	if m.prompt != promptNone {
		t.Fatal("a session that cannot switch opened an input anyway")
	}
	if !strings.Contains(m.err, "tix tenant use") {
		t.Fatalf("err = %q, want it to name the command that can switch", m.err)
	}
}

func TestSwitchingTenantReplacesTheWholeSession(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m.tenantKey = "default"
	m.leases = map[string]string{"t1": "token"}
	m, _ = m.reduce(eventMsg{event: activityEvent(1, "alice")})

	next := newFakeService()
	next.whoAmI = &core.Actor{ID: "u2", Handle: "bob", TenantID: "acme"}
	closed, keys := 0, []string(nil)
	m.dialTenant = dialerFor(next, &closed, &keys)

	m, _ = m.reduce(pressKey("T"))
	m, _ = m.reduce(pressKey("enter"))
	if m.prompt != promptTenant {
		t.Fatalf("prompt = %v, want the tenant input", m.prompt)
	}
	m = typeText(m, "acme")
	m, cmd := m.reduce(pressKey("enter"))
	m = runCmd(t, m, cmd)

	if keys == nil || keys[0] != "acme" {
		t.Fatalf("dialled %v, want the typed key", keys)
	}
	if m.tenantKey != "acme" || m.svc != core.Service(next) {
		t.Fatalf("session is on tenant %q with service %T", m.tenantKey, m.svc)
	}
	if m.actor == nil || m.actor.Handle != "bob" {
		t.Fatalf("actor = %+v, want the one the target tenant named", m.actor)
	}
	if m.project.Key != "" || len(m.columns) != 0 || len(m.activity) != 0 || len(m.leases) != 0 {
		t.Fatal("state from the previous tenant survived the switch")
	}
	if m.view != viewProjects {
		t.Fatalf("view after a switch = %v, want the project list", m.view)
	}
	if !strings.Contains(m.status, "acme") || !strings.Contains(m.status, "lease") {
		t.Fatalf("status = %q, want the tenant and the dropped lease named", m.status)
	}
	if !strings.Contains(m.View(), "@acme") {
		t.Fatal("the title bar does not name the tenant in force")
	}
}

func TestATenantThatCannotBeReachedKeepsTheSessionWhereItIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		dial TenantDialer
	}{
		{"the dial fails", func(context.Context, string) (TenantConn, error) {
			return TenantConn{}, errors.New("no such database")
		}},
		{"the target refuses the actor", func(ctx context.Context, _ string) (TenantConn, error) {
			svc := newFakeService()
			svc.whoAmIErr = core.Forbidden("this actor belongs to another tenant")
			return TenantConn{Service: svc, Context: ctx, Close: func() error { return nil }}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := boardModel(t)
			before := newFakeService()
			m.svc, m.tenantKey, m.dialTenant = before, "default", tc.dial
			m, _ = m.reduce(pressKey("T"))
			m, _ = m.reduce(pressKey("enter"))
			m = typeText(m, "ghost")
			m, cmd := m.reduce(pressKey("enter"))
			m = runCmd(t, m, cmd)

			if m.tenantKey != "default" || m.svc != core.Service(before) {
				t.Fatalf("a failed switch moved the session to %q", m.tenantKey)
			}
			if !strings.Contains(m.err, "ghost") {
				t.Fatalf("err = %q, want the key that could not be reached", m.err)
			}
		})
	}
}

func TestSwitchingAgainClosesTheConnectionThisSessionOpened(t *testing.T) {
	m := boardModel(t)
	m.svc, m.tenantKey = newFakeService(), "default"
	closed, keys := 0, []string(nil)
	m.dialTenant = dialerFor(newFakeService(), &closed, &keys)

	m, _ = m.reduce(pressKey("T"))
	m, _ = m.reduce(pressKey("enter"))
	m = typeText(m, "acme")
	m, cmd := m.reduce(pressKey("enter"))
	m = runCmd(t, m, cmd)
	if closed != 0 {
		t.Fatalf("the caller's own connection was closed %d times", closed)
	}

	m, _ = m.reduce(pressKey("T"))
	m, _ = m.reduce(pressKey("enter"))
	m = typeText(m, "beta")
	m, cmd = m.reduce(pressKey("enter"))
	if m = runCmd(t, m, cmd); m.tenantKey != "beta" {
		t.Fatalf("the second switch landed on %q", m.tenantKey)
	}
	if closed != 1 {
		t.Fatalf("connections closed = %d, want the one this session had opened", closed)
	}
}

// A stream belongs to the connection it was opened on. After a switch the
// previous tenant's events must not be drawn under the new tenant's name.
func TestEventsFromThePreviousTenantAreDroppedAfterASwitch(t *testing.T) {
	m := boardModel(t)
	m.svc, m.tenantKey = newFakeService(), "default"
	closed, keys := 0, []string(nil)
	m.dialTenant = dialerFor(newFakeService(), &closed, &keys)
	m, _ = m.reduce(pressKey("T"))
	m, _ = m.reduce(pressKey("enter"))
	m = typeText(m, "acme")
	m, cmd := m.reduce(pressKey("enter"))
	m = runCmd(t, m, cmd)

	stale := m.gen - 1
	m, _ = m.reduce(eventMsg{event: activityEvent(9, "alice"), gen: stale})
	if len(m.activity) != 0 {
		t.Fatalf("activity = %v, want the previous tenant's event dropped", m.activity)
	}
	m, _ = m.reduce(streamMsg{connected: true, events: make(chan core.Event), gen: stale})
	if m.connected {
		t.Fatal("a stale subscription was adopted")
	}
	m, _ = m.reduce(eventMsg{event: activityEvent(9, "alice"), gen: m.gen})
	if len(m.activity) != 1 {
		t.Fatalf("activity = %v, want the current tenant's event recorded", m.activity)
	}
}

func TestTenantSwitchIsReachableFromEveryScheme(t *testing.T) {
	t.Parallel()
	for _, scheme := range Schemes() {
		keys, err := KeyMapFrom(scheme, nil)
		if err != nil {
			t.Fatalf("KeyMapFrom(%q): %v", scheme, err)
		}
		if b, ok := keys.Binding("Tenant"); !ok || len(b.Keys()) == 0 {
			t.Errorf("scheme %q leaves the tenant view unbound", scheme)
		}
	}
}
