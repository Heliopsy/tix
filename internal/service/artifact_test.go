package service

import (
	"bytes"
	"math"
	"testing"

	"github.com/heliopsy/tix/internal/core"
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
