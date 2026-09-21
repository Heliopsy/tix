package tui

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestActorIDsCollectsDistinctNonEmptyIdentifiers(t *testing.T) {
	task := core.Task{AssigneeActorID: "a1", CreatorActorID: "a2", ClaimedByActorID: "a1"}
	comments := []core.Comment{{AuthorActorID: "a3"}, {AuthorActorID: ""}, {AuthorActorID: "a2"}}
	got := actorIDs(task, comments)
	want := []string{"a1", "a2", "a3"}
	if len(got) != len(want) {
		t.Fatalf("actorIDs = %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("actorIDs[%d] = %q, want %q", i, got[i], id)
		}
	}
}

func TestActorIDsSkipsEmptyIdentifiers(t *testing.T) {
	if got := actorIDs(core.Task{}, nil); got != nil {
		t.Fatalf("actorIDs of an unassigned, unclaimed task = %v, want nil", got)
	}
}

func TestResolveActorsMapsHandlesAndSkipsFailedLookups(t *testing.T) {
	svc := newFakeService()
	svc.actors = map[string]*core.Actor{
		"a1": {ID: "a1", Handle: "alice"},
		"a2": {ID: "a2", Handle: ""}, // a real actor, but with no handle set
	}
	got := resolveActors(context.Background(), svc, []string{"a1", "a2", "gone"})
	if len(got) != 1 || got["a1"] != "alice" {
		t.Fatalf("resolveActors = %v, want only a1 -> alice", got)
	}
}

func TestDetailForResolvesEveryActorTheDetailViewNames(t *testing.T) {
	svc := newFakeService()
	svc.tasks = []core.Task{{
		ID: "t1", Ref: "infra-1", AssigneeActorID: "assignee", CreatorActorID: "creator", ClaimedByActorID: "holder",
	}}
	svc.actors = map[string]*core.Actor{
		"assignee": {Handle: "bob"},
		"creator":  {Handle: "root"},
		"holder":   {Handle: "carol"},
		"author":   {Handle: "dana"},
	}
	m := New(Config{Service: svc})
	cmd := m.detailFor(core.TaskRef{ID: "t1"})
	msg, ok := cmd().(detailMsg)
	if !ok {
		t.Fatalf("detailFor did not return a detailMsg")
	}
	for id, handle := range map[string]string{"assignee": "bob", "creator": "root", "holder": "carol"} {
		if msg.actors[id] != handle {
			t.Fatalf("actors[%q] = %q, want %q (actors=%v)", id, msg.actors[id], handle, msg.actors)
		}
	}
}
