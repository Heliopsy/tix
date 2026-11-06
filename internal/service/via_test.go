// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A sync run's changes are the sync's, and remember who started it: the
// source put before the run keeps the caller's own channel.
func TestASyncRunRecordsTheSyncChannelAndWhoStartedIt(t *testing.T) {
	f := newSyncFixture(t, "ops")
	started := core.Via{Kind: core.ViaWeb, Session: "s-0123456789abcdef", ClientAddr: "192.0.2.9"}
	f.ctx = core.WithVia(f.ctx, started)
	f.run(t, core.RunSyncInput{})

	entries, _, err := f.local.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: 200}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]core.Via{}
	for _, e := range entries {
		seen[e.Action] = e.Via
	}
	for _, action := range []string{auditSyncTaskCreate, auditSyncRun} {
		v, ok := seen[action]
		if !ok {
			t.Fatalf("no %s entry among %v", action, seen)
		}
		if v.Kind != core.ViaSync || v.Origin != core.ViaWeb || v.Session != started.Session {
			t.Errorf("%s recorded %+v, want sync started from the web session", action, v)
		}
	}
	if v := seen[auditSyncSourcePut]; v.Kind != core.ViaUnknown {
		t.Errorf("the source registered before the channel was set recorded %+v", v)
	}
}

// The filter is checked by the service, so an API caller asking for a channel
// that does not exist is told so rather than shown an empty log.
func TestAnUnknownChannelFilterIsRefused(t *testing.T) {
	l, _, _, actor := newLocal(t)
	_, _, err := l.ListAudit(core.WithActor(context.Background(), actor), core.AuditFilter{Vias: []core.ViaKind{"telepathy"}})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("a made-up channel = %v, want invalid", err)
	}
}
