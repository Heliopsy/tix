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
