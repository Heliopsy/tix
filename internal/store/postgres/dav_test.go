// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// davToken stores an api_tokens row for an app password to hang off.
func davToken(t *testing.T, s *Store, f fixture, id, name, actorID string) {
	t.Helper()
	ctx := context.Background()
	tok := core.APIToken{ID: id, ActorID: actorID, Name: name, Scopes: []core.Scope{core.ScopeCaldav},
		CreatedAt: f.clock.Now()}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateToken(ctx, &tok, "hash-"+id) }); err != nil {
		t.Fatalf("creating token %s: %v", id, err)
	}
}

func TestDAVPasswordsKeepTheirSettingsPerTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	davToken(t, s, a, "01DAVTOKEN0000000000000001", "dav-worker-iphone", a.actor.ID)
	davToken(t, s, a, "01DAVTOKEN0000000000000002", "dav-worker-mac", a.actor.ID)
	created := clk.Now().Truncate(time.Second)
	for _, p := range []core.DAVPassword{
		{ID: "01DAVTOKEN0000000000000001", ActorID: a.actor.ID, Name: "iPhone",
			Collections: core.DAVCollectionsAssigned, CreatedAt: created},
		{ID: "01DAVTOKEN0000000000000002", ActorID: a.actor.ID, Name: "Mac",
			Collections: core.DAVCollectionsAll, ReadOnly: true, CreatedAt: created.Add(time.Second)},
	} {
		if err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.CreateDAVPassword(ctx, &p) }); err != nil {
			t.Fatalf("creating %s: %v", p.Name, err)
		}
	}
	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, err := tx.GetDAVPassword(ctx, "01DAVTOKEN0000000000000002")
		if err != nil {
			return err
		}
		if got.Name != "Mac" || got.Collections != core.DAVCollectionsAll || !got.ReadOnly ||
			got.ActorID != a.actor.ID || !got.CreatedAt.Equal(created.Add(time.Second)) {
			t.Errorf("read back %+v", got)
		}
		list, err := tx.ListDAVPasswords(ctx, a.actor.ID)
		if err != nil {
			return err
		}
		if len(list) != 2 || list[0].Name != "iPhone" || list[1].Name != "Mac" || list[0].ReadOnly {
			t.Errorf("listed %+v", list)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.View(ctx, b.scope, func(tx store.Tx) error {
		if _, err := tx.GetDAVPassword(ctx, "01DAVTOKEN0000000000000001"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("another tenant read the app password: %v", err)
		}
		list, err := tx.ListDAVPasswords(ctx, a.actor.ID)
		if err == nil && len(list) != 0 {
			t.Errorf("another tenant listed %+v", list)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDAVSyncStatesShareASetAndPrune(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	const tok = "01DAVTOKEN0000000000000003"
	davToken(t, s, a, tok, "dav-worker-phone", a.actor.ID)
	put := func(hash string, seq int64, at time.Time) {
		t.Helper()
		st := store.DAVSyncState{TokenID: tok, ActorID: a.actor.ID, Collection: "assigned", SetHash: hash,
			Members: []core.DAVMember{{Name: "x", ETag: `"1"`}}, EventSeq: seq, EventMark: 2, Config: "c",
			ValidUntil: at.Add(time.Hour), SeenAt: at}
		if err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.PutDAVSyncState(ctx, &st) }); err != nil {
			t.Fatalf("putting %s: %v", hash, err)
		}
	}
	base := clk.Now().Truncate(time.Second)
	put("h1", 1, base)
	put("h1", 7, base.Add(time.Minute))
	count := func(scope core.TenantScope, hash string) (found bool, seq int64) {
		t.Helper()
		_ = s.View(ctx, scope, func(tx store.Tx) error {
			st, err := tx.GetDAVSyncState(ctx, tok, "assigned", hash)
			if err == nil {
				found, seq = true, st.EventSeq
				if len(st.Members) != 1 || st.Members[0].ETag != `"1"` || st.EventMark != 2 || st.Config != "c" {
					t.Errorf("read back %+v", st)
				}
			} else if !core.IsKind(err, core.KindNotFound) {
				t.Errorf("reading %s: %v", hash, err)
			}
			return nil
		})
		return found, seq
	}
	if found, seq := count(a.scope, "h1"); !found || seq != 7 {
		t.Fatalf("one set written twice: found %v seq %d, want one row updated to 7", found, seq)
	}
	if found, _ := count(b.scope, "h1"); found {
		t.Error("another tenant read the sync state")
	}
	for i, h := range []string{"h2", "h3", "h4", "h5"} {
		put(h, int64(10+i), base.Add(time.Duration(2+i)*time.Minute))
	}
	var pruned int64
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		var err error
		pruned, err = tx.PruneDAVSyncStates(ctx, tok, "assigned", 3, base.Add(4*time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"h1": false, "h2": false, "h3": true, "h4": true, "h5": true}
	for h, keep := range want {
		if found, _ := count(a.scope, h); found != keep {
			t.Errorf("%s kept = %v, want %v", h, found, keep)
		}
	}
	if pruned != 2 {
		t.Errorf("pruned %d, want 2", pruned)
	}
}

// A write that took its event sequence number first and commits last leaves
// the latest sequence where the other write put it. The CalDAV short cut
// compares the count of events below that sequence too, and this is the case
// it exists for: the count moves when the slow write lands, so the next sync
// lists the collection again instead of answering "no changes".
func TestDAVEventMarkMovesWhenAWriteCommitsOutOfSequenceOrder(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	mark := func() (seq, count int64) {
		t.Helper()
		if err := s.View(ctx, a.scope, func(tx store.Tx) error {
			var err error
			if seq, err = tx.LatestEventSeq(ctx); err != nil {
				return err
			}
			count, err = tx.CountEventsAfter(ctx, seq-1024)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return seq, count
	}
	appended := make(chan struct{})
	release := make(chan struct{})
	slow := make(chan error, 1)
	go func() {
		slow <- s.Update(ctx, a.scope, func(tx store.Tx) error {
			if err := tx.AppendEvent(ctx, &core.Event{Type: core.EventTaskUpdated, SubjectType: "task", SubjectID: "slow"}); err != nil {
				return err
			}
			close(appended)
			<-release
			return nil
		})
	}()
	<-appended
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		return tx.AppendEvent(ctx, &core.Event{Type: core.EventTaskUpdated, SubjectType: "task", SubjectID: "fast"})
	}); err != nil {
		t.Fatal(err)
	}
	seqBefore, countBefore := mark()
	close(release)
	if err := <-slow; err != nil {
		t.Fatal(err)
	}
	seqAfter, countAfter := mark()
	if seqAfter != seqBefore {
		t.Fatalf("the latest sequence moved from %d to %d; the slow write did not commit out of order", seqBefore, seqAfter)
	}
	if countAfter != countBefore+1 {
		t.Errorf("the event count stayed %d after the slow write landed, want %d", countAfter, countBefore+1)
	}
}
