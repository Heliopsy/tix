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
