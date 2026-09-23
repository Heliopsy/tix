// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestEventVerb(t *testing.T) {
	tests := []struct {
		in   core.EventType
		want string
	}{
		{core.EventTaskCreated, "created"},
		{core.EventTaskClaimed, "claimed"},
		{core.EventTaskLeaseExpired, "lease expired"},
		{core.EventType("no_dot"), "no dot"},
	}
	for _, tc := range tests {
		if got := EventVerb(tc.in); got != tc.want {
			t.Errorf("EventVerb(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEventRefFallsBackToSubjectID(t *testing.T) {
	withRef := core.Event{SubjectID: "tsk_1", Payload: map[string]any{"ref": "infra-3"}}
	if got := EventRef(withRef); got != "infra-3" {
		t.Fatalf("EventRef = %q, want infra-3", got)
	}
	withoutRef := core.Event{SubjectID: "tsk_1", Payload: map[string]any{"claimed_by": "alice"}}
	if got := EventRef(withoutRef); got != "tsk_1" {
		t.Fatalf("EventRef = %q, want the raw subject id", got)
	}
}

func TestEventDetail(t *testing.T) {
	transitioned := core.Event{
		Type:    core.EventTaskTransitioned,
		Payload: map[string]any{"from": "todo", "to": "doing"},
	}
	if got := EventDetail(transitioned); got != "todo → doing" {
		t.Fatalf("EventDetail = %q", got)
	}
	created := core.Event{Type: core.EventTaskCreated, Payload: map[string]any{"title": "buy milk"}}
	if got := EventDetail(created); got != "buy milk" {
		t.Fatalf("EventDetail = %q, want the title", got)
	}
	bare := core.Event{Type: core.EventTaskClaimed, Payload: map[string]any{"lease_expires_at": "..."}}
	if got := EventDetail(bare); got != "" {
		t.Fatalf("EventDetail = %q, want empty for a payload with nothing to say", got)
	}
}

func TestEventDetailTaskUpdated(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{"plain field edit carries nothing today", map[string]any{"ref": "homelab-3", "version": 2}, ""},
		{"fields key, once the service ships one", map[string]any{"fields": []string{"title", "priority"}}, "the title and priority"},
		{"fields key as []any, JSON's own decoded shape", map[string]any{"fields": []any{"title"}}, "the title"},
		{"restore", map[string]any{"restored": true}, "restored"},
		{"comment delete", map[string]any{"deleted": true}, "comment deleted"},
		{"lease renew", map[string]any{"lease_expires_at": "2026-01-01T00:00:00Z"}, "lease renewed"},
		{"tag removed", map[string]any{"removed": true, "tag": "urgent"}, "tag removed: urgent"},
		{"dependency removed", map[string]any{"removed": true, "depends_on": "infra-9"}, "dependency removed: infra-9"},
		{"sync import", map[string]any{"system": "jira", "title": "buy milk"}, "synced from jira"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EventDetail(core.Event{Type: core.EventTaskUpdated, Payload: tc.payload})
			if got != tc.want {
				t.Fatalf("EventDetail = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFieldLabel(t *testing.T) {
	if got := FieldLabel("title"); got != "title" {
		t.Fatalf("FieldLabel(title) = %q", got)
	}
	if got := FieldLabel("some_custom_key"); got != "some custom key" {
		t.Fatalf("FieldLabel(some_custom_key) = %q, want the underscore fallback", got)
	}
}

func TestSummariseFields(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"title"}, "the title"},
		{[]string{"title", "priority"}, "the title and priority"},
		{[]string{"title", "priority", "due_at"}, "the title, priority and due date"},
	}
	for _, tc := range tests {
		if got := SummariseFields(tc.in); got != tc.want {
			t.Fatalf("SummariseFields(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEventActor(t *testing.T) {
	if got := EventActor(core.Event{ActorID: "alice"}); got != "alice" {
		t.Fatalf("EventActor = %q", got)
	}
	if got := EventActor(core.Event{}); got != "-" {
		t.Fatalf("EventActor = %q, want a placeholder for no actor", got)
	}
}

func TestFormatEventLineReadsAtAGlanceAndNeverColoursWithoutAPainter(t *testing.T) {
	e := core.Event{
		Type:       core.EventTaskClaimed,
		ActorID:    "alice",
		SubjectID:  "tsk_1",
		OccurredAt: time.Date(2026, 1, 1, 10, 15, 3, 0, time.Local),
		Payload:    map[string]any{"ref": "homelab-3"},
	}
	plain := FormatEventLine(NewPainter(ModeNever, nil), e)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain line carries an escape code: %q", plain)
	}
	for _, want := range []string{"10:15:03", "alice", "claimed", "homelab-3"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("line = %q, missing %q", plain, want)
		}
	}
	coloured := FormatEventLine(NewPainter(ModeAlways, new(strings.Builder)), e)
	if !strings.Contains(coloured, "\x1b[") {
		t.Fatalf("coloured line carries no escape code: %q", coloured)
	}
}

// A watcher's whole resume story rests on being able to read its position off
// the stream. The default line is the one output a person and a shell pipeline
// both see, so the sequence number has to be on it.
func TestFormatEventLineLeadsWithTheResumeCursor(t *testing.T) {
	line := FormatEventLine(NewPainter(ModeNever, nil), core.Event{
		Seq: 42, Type: core.EventTaskCreated, ActorID: "alice",
		OccurredAt: time.Date(2026, 1, 1, 10, 15, 3, 0, time.Local),
		Payload:    map[string]any{"ref": "homelab-3", "title": "buy milk"},
	})
	if !strings.HasPrefix(line, "000042") {
		t.Fatalf("line = %q, want it to lead with the sequence number a watcher resumes from", line)
	}
}

// What a person watching a queue needs is what changed, not a re-listing of
// the task. Each of these event types used to print its ref and nothing else.
func TestEventDetailNamesWhatActuallyChanged(t *testing.T) {
	occurred := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		event core.Event
		want  string
	}{
		{"a transition names both states", core.Event{
			Type: core.EventTaskTransitioned, Payload: map[string]any{"from": "todo", "to": "doing"},
		}, "todo → doing"},
		{"an edit names the fields it touched", core.Event{
			Type: core.EventTaskUpdated, Payload: map[string]any{"fields": []any{"title", "priority"}},
		}, "the title and priority"},
		{"a self claim names the lease length", core.Event{
			Type: core.EventTaskClaimed, ActorID: "alice", OccurredAt: occurred,
			Payload: map[string]any{
				"actor_handle": "alice", "claimed_by": "alice",
				"lease_expires_at": occurred.Add(30 * time.Minute),
			},
		}, "lease 30m"},
		{"a claim for somebody else names the holder", core.Event{
			Type: core.EventTaskClaimed, ActorID: "a1", OccurredAt: occurred,
			Payload: map[string]any{
				"actor_handle": "alice", "claimed_by": "a2", "claimed_by_handle": "pax",
				"lease_expires_at": occurred.Add(time.Hour),
			},
		}, "for pax, lease 1h"},
		{"a lease expiry names the holder it was taken from", core.Event{
			Type: core.EventTaskLeaseExpired,
			Payload: map[string]any{
				"previous_holder": "a2", "previous_holder_handle": "pax", "reverted_to": "todo",
			},
		}, "held by pax, reverted to todo"},
		{"a lease expiry with no revert still names the holder", core.Event{
			Type:    core.EventTaskLeaseExpired,
			Payload: map[string]any{"previous_holder": "a2", "previous_holder_handle": "pax"},
		}, "held by pax"},
		{"a holder with no handle falls back to its identifier", core.Event{
			Type:    core.EventTaskLeaseExpired,
			Payload: map[string]any{"previous_holder": "a2"},
		}, "held by a2"},
		{"a release names the status it left the task in", core.Event{
			Type: core.EventTaskReleased, Payload: map[string]any{"final_status": "done", "status": "done"},
		}, "→ done"},
		{"a timestamp that survived a JSON round trip still reads", core.Event{
			Type: core.EventTaskClaimed, ActorID: "alice", OccurredAt: occurred,
			Payload: map[string]any{
				"actor_handle": "alice", "claimed_by": "alice",
				"lease_expires_at": occurred.Add(90 * time.Minute).Format(time.RFC3339Nano),
			},
		}, "lease 1h30m"},
		{"an already expired lease says nothing rather than a negative", core.Event{
			Type: core.EventTaskClaimed, ActorID: "alice", OccurredAt: occurred,
			Payload: map[string]any{
				"actor_handle": "alice", "claimed_by": "alice",
				"lease_expires_at": occurred.Add(-time.Minute),
			},
		}, ""},
		{"a creation still names the title", core.Event{
			Type: core.EventTaskCreated, Payload: map[string]any{"title": "buy milk"},
		}, "buy milk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EventDetail(tt.event); got != tt.want {
				t.Errorf("EventDetail = %q, want %q", got, tt.want)
			}
		})
	}
}

// The default line is for a person scanning a queue, so the richness lives in
// the structured formats. A detail that grew to a paragraph would make the
// tail unreadable, which is the failure the original one-line format avoided
// by saying nothing at all.
func TestEventDetailStaysShortEnoughToScan(t *testing.T) {
	occurred := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []core.Event{
		{Type: core.EventTaskClaimed, ActorID: "a1", OccurredAt: occurred, Payload: map[string]any{
			"claimed_by": "a2", "claimed_by_handle": "pax", "lease_expires_at": occurred.Add(time.Hour)}},
		{Type: core.EventTaskLeaseExpired, Payload: map[string]any{
			"previous_holder_handle": "pax", "reverted_to": "todo"}},
		{Type: core.EventTaskUpdated, Payload: map[string]any{
			"fields": []any{"title", "body", "priority", "assignee_actor_id", "due_at"}}},
	}
	for _, e := range events {
		if got := EventDetail(e); len(got) > 80 {
			t.Errorf("detail for %s is %d characters, too long for one scannable line: %q", e.Type, len(got), got)
		}
	}
}
