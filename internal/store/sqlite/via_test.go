// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func appendVia(t *testing.T, s *Store, f fixture, action string, v core.Via) {
	t.Helper()
	ctx := context.Background()
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.AppendAudit(ctx, &core.AuditEntry{
			ActorID: f.actor.ID, Action: action, SubjectType: "task", SubjectID: "t1",
			Source: core.SourceAPI, Via: v,
		}); err != nil {
			return err
		}
		return tx.AppendEvent(ctx, &core.Event{
			Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1", Via: &v,
		})
	}); err != nil {
		t.Fatalf("appending %s: %v", action, err)
	}
}

func listVia(t *testing.T, s *Store, f fixture, filter core.AuditFilter) []core.AuditEntry {
	t.Helper()
	var out []core.AuditEntry
	if err := s.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		out, err = tx.ListAudit(context.Background(), filter)
		return err
	}); err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	return out
}

func TestTheAccessChannelRoundTripsAndFilters(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	ssh := core.Via{Kind: core.ViaSSH, KeyFingerprint: "SHA256:abc", ClientAddr: "192.0.2.5"}
	ci := core.Via{Kind: core.ViaAPI, TokenID: "tok1", TokenName: "ci-agent", ClientAddr: "192.0.2.6", UserAgent: "curl"}
	appendVia(t, s, f, "by.ssh", ssh)
	appendVia(t, s, f, "by.ci", ci)
	appendVia(t, s, f, "by.mcp", core.Via{Kind: core.ViaMCP, TokenName: "mcp-agent"})

	all := listVia(t, s, f, core.AuditFilter{Actions: []string{"by.ssh", "by.ci"}})
	got := map[string]core.Via{}
	for _, e := range all {
		got[e.Action] = e.Via
	}
	if got["by.ssh"] != ssh || got["by.ci"] != ci {
		t.Errorf("read back %+v, want %+v and %+v", got, ssh, ci)
	}

	for _, tc := range []struct {
		name   string
		filter core.AuditFilter
		want   []string
	}{
		{"one channel", core.AuditFilter{Vias: []core.ViaKind{core.ViaSSH}}, []string{"by.ssh"}},
		{"two channels", core.AuditFilter{Vias: []core.ViaKind{core.ViaSSH, core.ViaMCP}}, []string{"by.ssh", "by.mcp"}},
		{"a token", core.AuditFilter{Tokens: []string{"ci-agent"}}, []string{"by.ci"}},
		{"a channel and a token", core.AuditFilter{Vias: []core.ViaKind{core.ViaMCP}, Tokens: []string{"ci-agent"}}, nil},
		{"text finds the token", core.AuditFilter{Text: "mcp-agent"}, []string{"by.mcp"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actions []string
			for _, e := range listVia(t, s, f, tc.filter) {
				actions = append(actions, e.Action)
			}
			if !sameSet(actions, tc.want) {
				t.Errorf("selected %v, want %v", actions, tc.want)
			}
		})
	}

	var events []core.Event
	if err := s.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		events, err = tx.ReadEvents(context.Background(), 0, 10)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Via == nil || events[0].Via.Kind != core.ViaSSH {
		t.Fatalf("events = %+v, want three with the ssh channel first", events)
	}
	if events[0].Via.KeyFingerprint != "" || events[1].Via.ClientAddr != "" || events[1].Via.UserAgent != "" {
		t.Errorf("an event kept network detail: %+v, %+v", *events[0].Via, *events[1].Via)
	}
	if events[1].Via.TokenName != "ci-agent" {
		t.Errorf("an event lost its token name: %+v", *events[1].Via)
	}
}

// A row written before the channel existed has nothing in its columns but
// the migration's default, and reads, and filters, as unknown.
func TestAnEntryFromBeforeTheChannelReadsAsUnknown(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO audit_entries (tenant_id, action, subject_type, subject_id, source, occurred_at)
		 VALUES ('` + f.tenant.ID + `', 'old.entry', 'task', 't0', 'cli', '2025-01-01T00:00:00.000000000Z')`,
		`INSERT INTO events (id, tenant_id, type, subject_type, subject_id, occurred_at)
		 VALUES ('old-event', '` + f.tenant.ID + `', 'task.created', 'task', 't0', '2025-01-01T00:00:00.000000000Z')`,
	} {
		if _, err := s.writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("writing an old row: %v", err)
		}
	}
	old := listVia(t, s, f, core.AuditFilter{Vias: []core.ViaKind{core.ViaUnknown}})
	if len(old) != 1 || old[0].Action != "old.entry" || old[0].Via != (core.Via{Kind: core.ViaUnknown}) {
		t.Fatalf("unknown channel selected %+v, want the old entry alone", old)
	}
	var events []core.Event
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		events, err = tx.ReadEvents(ctx, 0, 10)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Via == nil || events[0].Via.Kind != core.ViaUnknown {
		t.Errorf("old event = %+v, want the unknown channel", events)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}
