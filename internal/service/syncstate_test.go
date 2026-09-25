// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// TestAFullRefreshThatFailsLeavesTheWatermarkAlone pins the fix for a refresh
// that punished a network blip with a full re-import.
//
// A full refresh set the run's own cursor to "" so the source would be read
// from the beginning. That cursor is also what gets persisted, so a refresh
// that failed before committing a page wrote an empty watermark over the one
// every previous incremental run had earned. The source is told to ignore the
// watermark; the watermark itself is not the refresh's to destroy.
func TestAFullRefreshThatFailsLeavesTheWatermarkAlone(t *testing.T) {
	f := newSyncFixture(t, "ops")

	// An ordinary run first, so there is a watermark worth losing.
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return pagedImporter(-1), nil
	})
	f.run(t, core.RunSyncInput{})
	earned := f.reloadSource(t).Cursor
	if earned == "" {
		t.Fatal("the setup run stored no watermark")
	}

	// Now a full refresh whose very first fetch fails.
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return pagedImporter(0), nil
	})
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID, Full: true}); err == nil {
		t.Fatal("RunSync succeeded against a source that failed immediately")
	}

	after := f.reloadSource(t)
	if after.Cursor != earned {
		t.Errorf("watermark = %q after a failed full refresh, want the earned %q:\n"+
			"the next incremental run now re-reads the whole source", after.Cursor, earned)
	}
	if after.LastStatus != "failed" {
		t.Errorf("last status = %q, want the failure recorded", after.LastStatus)
	}
}

// A full refresh that succeeds must still move the watermark forward.
func TestAFullRefreshThatSucceedsStoresTheNewWatermark(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return pagedImporter(-1), nil
	})
	f.run(t, core.RunSyncInput{Full: true})
	if got := f.reloadSource(t); got.Cursor != "2026-01-03T00:00:00Z" {
		t.Errorf("watermark = %q, want the last page's", got.Cursor)
	}
}

// TestTheAuditEntryCarriesTheWatermarkTheRunReached pins the record an
// operator actually reads later.
//
// The result's cursor was assigned after the run finished, which is after the
// audit entry that embeds the result has already been written. Every entry in
// the log therefore claimed the run reached nothing, and the only place the
// real value appeared was the command's own output, which nobody keeps.
func TestTheAuditEntryCarriesTheWatermarkTheRunReached(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return pagedImporter(-1), nil
	})
	res := f.run(t, core.RunSyncInput{})
	if res.Cursor == "" {
		t.Fatal("the returned result carries no watermark")
	}

	var entries []core.AuditEntry
	if err := f.local.read(f.ctx, f.actor, func(tx store.Tx) error {
		var err error
		entries, err = tx.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: 200}})
		return err
	}); err != nil {
		t.Fatalf("reading audit: %v", err)
	}

	var found bool
	for _, e := range entries {
		if e.Action != auditSyncRun {
			continue
		}
		found = true
		raw, err := json.Marshal(e.After)
		if err != nil {
			t.Fatalf("marshalling the recorded result: %v", err)
		}
		var got core.SyncResult
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("the recorded result is not a sync result: %v\n%s", err, raw)
		}
		if got.Cursor != res.Cursor {
			t.Errorf("the audit entry recorded watermark %q, want %q", got.Cursor, res.Cursor)
		}
	}
	if !found {
		t.Fatal("the run wrote no audit entry")
	}
}

// TestASourceThatCannotBeReadIsUpstreamNotInternal pins the classification an
// operator acts on.
//
// Every import failure was core.Internal, so an unreachable Jira and a bug in
// tix were indistinguishable: both printed "internal error", both exited 1, and
// the HTTP surface replaced the message with "internal error" on the way out,
// which is exactly the information the operator needed.
func TestASourceThatCannotBeReadIsUpstreamNotInternal(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return pagedImporter(0), nil
	})

	_, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID})
	if err == nil {
		t.Fatal("RunSync succeeded against an unreachable source")
	}
	if got := core.KindOf(err); got != core.KindUpstream {
		t.Errorf("error kind = %q, want %q", got, core.KindUpstream)
	}

	var domain *core.Error
	if !errors.As(err, &domain) {
		t.Fatalf("error is not a domain error: %v", err)
	}
	if !strings.Contains(domain.Message, f.source.Name) {
		t.Errorf("message %q does not name the source", domain.Message)
	}
	if got := core.KindUpstream.ExitCode(); got == core.ExitError {
		t.Errorf("an upstream failure exits %d, the same as any other error: a "+
			"scheduled sync cannot tell an outage from a bug", got)
	}
}

// The other half of the same split: a failure in tix's own write path keeps
// its own kind rather than being reported as someone else's outage.
func TestAFailureApplyingAPageIsNotReportedAsUpstream(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return &fakeImporter{failAt: -1, pages: []extsync.Batch{{
			// A record with no identity cannot be applied; the mapping refuses
			// it, and that refusal is tix's own, not the source's.
			Records: []extsync.Record{{Fields: map[string]any{"unmapped": "x"}}},
			Cursor:  "2026-01-01T00:00:00Z",
		}}}, nil
	})
	if _, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID}); err != nil {
		if core.KindOf(err) == core.KindUpstream {
			t.Errorf("a local failure was reported as an upstream one: %v", err)
		}
	}
}
