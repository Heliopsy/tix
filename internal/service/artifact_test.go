// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/lease"
)

func TestArtifactsAreStoredAndListed(t *testing.T) {
	l, ctx, scope, actor, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "worked on"})
	ref := core.TaskRef{ID: task.ID}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	blob := []byte("raw output")
	got, err := l.PutArtifact(ctx, ref, core.ArtifactInput{
		Kind:        core.ArtifactResult,
		Name:        "summary",
		Payload:     map[string]any{"passed": true, "count": float64(3)},
		ContentType: "application/json",
		Blob:        blob,
	})
	if err != nil {
		t.Fatalf("PutArtifact: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("writing an artifact wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
	if got.ActorID != actor.ID || got.TaskID != task.ID {
		t.Errorf("artifact = %+v", got)
	}

	if _, err := l.PutArtifact(ctx, ref, core.ArtifactInput{
		Kind: core.ArtifactResult, Name: "second", Payload: map[string]any{"passed": false},
	}); err != nil {
		t.Fatalf("second artifact: %v", err)
	}

	artifacts, err := l.ListArtifacts(ctx, ref)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %d, want both of the same kind", len(artifacts))
	}
	var stored *core.Artifact
	for i := range artifacts {
		if artifacts[i].ID == got.ID {
			stored = &artifacts[i]
		}
	}
	if stored == nil {
		t.Fatal("the first artifact is missing from the listing")
	}
	if !bytes.Equal(stored.Blob, blob) {
		t.Errorf("blob = %q, want it returned unchanged", stored.Blob)
	}
	if stored.Payload["passed"] != true || stored.Payload["count"] != float64(3) {
		t.Errorf("payload = %v, want it returned unchanged", stored.Payload)
	}
}

func TestArtifactInputIsValidated(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}

	if _, err := l.PutArtifact(ctx, ref, core.ArtifactInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("artifact with no kind = %v, want invalid", err)
	}
	if _, err := l.PutArtifact(ctx, ref, core.ArtifactInput{
		Kind: core.ArtifactFile, Blob: make([]byte, core.MaxBlobSize+1),
	}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("oversized blob = %v, want invalid", err)
	}
	if _, err := l.PutArtifact(ctx, ref, core.ArtifactInput{
		Kind: core.ArtifactMetric, Payload: map[string]any{"nan": math.NaN()},
	}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("payload that is not valid JSON = %v, want invalid", err)
	}
	if _, err := l.PutArtifact(ctx, core.MustParseTaskRef("infra-99"), core.ArtifactInput{Kind: core.ArtifactLog}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("artifact on a missing task = %v, want not found", err)
	}
	if _, err := l.PutArtifact(t.Context(), ref, core.ArtifactInput{Kind: core.ArtifactLog}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("PutArtifact unauthenticated = %v", err)
	}
	if _, err := l.ListArtifacts(t.Context(), ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("ListArtifacts unauthenticated = %v", err)
	}
}

// TestArtifactWriteIsLeaseBound covers the matrix requireLeaseToken decides,
// through the artifact write rather than through a transition. The case that
// matters is the live lease with no token: a zombie runs `artifact put` with no
// token at all, so a guard that only refused an offered token would catch
// nothing.
func TestArtifactWriteIsLeaseBound(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lapse   bool
		token   func(held string) string
		wantErr bool
	}{
		{name: "the holder writes under its token", token: func(held string) string { return held }},
		{name: "a live lease refuses a tokenless write", token: func(string) string { return "" }, wantErr: true},
		{name: "a live lease refuses another token", token: func(string) string { return "not-the-held-one" }, wantErr: true},
		{name: "a lapsed lease accepts no token", lapse: true, token: func(string) string { return "" }},
		{name: "a lapsed lease refuses an offered token", lapse: true, token: func(held string) string { return held }, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClaimFixture(t, 0)
			task := f.seedTask(t, "under a lease", "doing", core.PriorityNormal)
			claim := mustClaim(t, f, task)
			if tc.lapse {
				f.clock.Advance(lease.DefaultTTL + time.Second)
			}

			_, err := f.local.PutArtifact(f.ctx, ref(task), core.ArtifactInput{
				Kind: core.ArtifactResult, Name: "summary", LeaseToken: tc.token(claim.LeaseToken),
			})
			if tc.wantErr {
				if !core.IsKind(err, core.KindLeaseExpired) {
					t.Fatalf("PutArtifact = %v, want lease expired", err)
				}
				if got := f.artifacts(t, task); len(got) != 0 {
					t.Errorf("a refused write left %d artifacts, want the list untouched", len(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("PutArtifact: %v", err)
			}
			if got := f.artifacts(t, task); len(got) != 1 {
				t.Fatalf("artifacts = %d, want the one just written", len(got))
			}
		})
	}
}

// TestZombieCannotWriteOverTheNewHolder is the reported defect: the predecessor
// writes a result that every later reader takes for the current holder's.
func TestZombieCannotWriteOverTheNewHolder(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "handed on", "doing", core.PriorityNormal)
	zombie := mustClaim(t, f, task)
	f.clock.Advance(lease.DefaultTTL + time.Second)

	live, err := f.local.ClaimTask(f.asOther(), ref(task), core.ClaimInput{})
	if err != nil {
		t.Fatalf("re-claiming as the new holder: %v", err)
	}
	if _, err := f.local.PutArtifact(f.asOther(), ref(task), core.ArtifactInput{
		Kind: core.ArtifactResult, Name: "summary",
		Payload: map[string]any{"by": "live"}, LeaseToken: live.LeaseToken,
	}); err != nil {
		t.Fatalf("the new holder writing its result: %v", err)
	}

	for _, token := range []string{zombie.LeaseToken, ""} {
		if _, err := f.local.PutArtifact(f.ctx, ref(task), core.ArtifactInput{
			Kind: core.ArtifactResult, Name: "summary",
			Payload: map[string]any{"by": "zombie"}, LeaseToken: token,
		}); !core.IsKind(err, core.KindLeaseExpired) {
			t.Errorf("the zombie's write with token %q = %v, want lease expired", token, err)
		}
	}

	got := f.artifacts(t, task)
	if len(got) != 1 {
		t.Fatalf("artifacts = %d, want only the live holder's", len(got))
	}
	if got[0].Payload["by"] != "live" || got[0].ActorID != f.other.ID {
		t.Errorf("surviving artifact = %+v, want the live holder's", got[0])
	}
}

// TestCommentingAndEditingStayOpenOnAClaimedTask is the other half of the
// design: tix is one queue shared by humans and agents, so the writes by which
// somebody else participates in held work take no token.
func TestCommentingAndEditingStayOpenOnAClaimedTask(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "held while discussed", "doing", core.PriorityNormal)
	mustClaim(t, f, task)

	if _, err := f.local.AddComment(f.asOther(), ref(task), "is this the race in TestRetry?"); err != nil {
		t.Errorf("commenting on a task somebody holds: %v", err)
	}
	high := core.PriorityHigh
	if _, err := f.local.UpdateTask(f.asOther(), ref(task), core.UpdateTaskInput{Priority: &high}); err != nil {
		t.Errorf("re-prioritising a task somebody holds: %v", err)
	}
	if got := f.reload(t, task); got.Priority != core.PriorityHigh || got.ClaimedByActorID != f.actor.ID {
		t.Errorf("task = %+v, want it re-prioritised and still held", got)
	}
}
