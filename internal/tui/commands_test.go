// SPDX-License-Identifier: AGPL-3.0-or-later

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
	m := New(Config{Access: fullAccess(), Service: svc})
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

// A title edit names the field it touched, not the generic "edited".
func TestUpdateSentenceNamesASingleField(t *testing.T) {
	title := "a better title"
	got := updateSentence("infra-3", core.UpdateTaskInput{Title: &title})
	if got != "edited the title of infra-3" {
		t.Fatalf("updateSentence = %q", got)
	}
}

// A priority edit is the one single-field case that names the value it was
// set to, since "P2" is exactly as short as the field name it would
// otherwise repeat.
func TestUpdateSentenceNamesThePriorityValue(t *testing.T) {
	p := core.PriorityHigh
	got := updateSentence("infra-3", core.UpdateTaskInput{Priority: &p})
	if got != "set the priority of infra-3 to P2" {
		t.Fatalf("updateSentence = %q", got)
	}
}

// Several fields changed at once summarise rather than listing everything
// changed to.
func TestUpdateSentenceSummarisesSeveralFields(t *testing.T) {
	title, p := "a better title", core.PriorityHigh
	got := updateSentence("infra-3", core.UpdateTaskInput{Title: &title, Priority: &p})
	if got != "edited the title and priority of infra-3" {
		t.Fatalf("updateSentence = %q", got)
	}
}

// An update that somehow set nothing still says something, rather than an
// empty status bar.
func TestUpdateSentenceFallsBackWhenNothingChanged(t *testing.T) {
	if got := updateSentence("infra-3", core.UpdateTaskInput{}); got != "edited infra-3" {
		t.Fatalf("updateSentence = %q", got)
	}
}

// updateTask's returned command carries the sentence the status bar shows,
// computed before the service call so a failed edit can still be explained
// by the generic path (actionFailure uses kind.Label(), not the sentence).
func TestUpdateTaskCommandCarriesTheSentence(t *testing.T) {
	svc := newFakeService()
	m := New(Config{Access: fullAccess(), Service: svc})
	title := "a better title"
	cmd := m.updateTask(core.Task{ID: "t1", Ref: "infra-3"}, core.UpdateTaskInput{Title: &title})
	msg, ok := cmd().(actionMsg)
	if !ok {
		t.Fatalf("updateTask did not return an actionMsg")
	}
	if msg.sentence != "edited the title of infra-3" {
		t.Fatalf("sentence = %q", msg.sentence)
	}
	if got := msg.statusText(); got != "edited the title of infra-3" {
		t.Fatalf("statusText = %q", got)
	}
}

// transition's returned command names the states it moved between, using
// the task's own ref rather than its raw identifier. boardModel's first
// task, "a", starts selected and sits in "todo".
func TestTransitionCommandNamesTheStates(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	cmd := m.transition("doing")
	msg, ok := cmd().(actionMsg)
	if !ok {
		t.Fatalf("transition did not return an actionMsg")
	}
	if msg.sentence != "moved infra-a from todo to doing" {
		t.Fatalf("sentence = %q", msg.sentence)
	}
}
