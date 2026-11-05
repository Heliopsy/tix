// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUpdateTaskInputDueAtIsTriState(t *testing.T) {
	due := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		body      string
		wantTouch bool
		wantDue   *time.Time
		wantTitle string
	}{
		{"absent leaves it", `{"title":"x"}`, false, nil, "x"},
		{"null clears it", `{"title":"x","due_at":null}`, true, nil, "x"},
		{"null with spacing clears it", `{"due_at" :  null }`, true, nil, ""},
		{"a value sets it", `{"due_at":"2026-12-01T09:00:00Z"}`, true, &due, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var in UpdateTaskInput
			if err := json.Unmarshal([]byte(tc.body), &in); err != nil {
				t.Fatalf("decoding: %v", err)
			}
			if (in.DueAt != nil) != tc.wantTouch {
				t.Fatalf("DueAt touched = %v, want %v", in.DueAt != nil, tc.wantTouch)
			}
			if in.Title != nil && *in.Title != tc.wantTitle || in.Title == nil && tc.wantTitle != "" {
				t.Fatalf("Title = %v, want %q", in.Title, tc.wantTitle)
			}
			if !tc.wantTouch {
				return
			}
			got := *in.DueAt
			if (got == nil) != (tc.wantDue == nil) || got != nil && !got.Equal(*tc.wantDue) {
				t.Fatalf("DueAt = %v, want %v", got, tc.wantDue)
			}
		})
	}
}

func TestUpdateTaskInputNullDueAtRoundTrips(t *testing.T) {
	var none *time.Time
	raw, err := json.Marshal(UpdateTaskInput{DueAt: &none})
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if string(raw) != `{"due_at":null}` {
		t.Fatalf("encoded %s, want a null due_at", raw)
	}
	var back UpdateTaskInput
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if back.DueAt == nil || *back.DueAt != nil {
		t.Fatalf("decoded DueAt = %v, want a clear", back.DueAt)
	}
}

func TestUpdateTaskInputRejectsMalformedJSON(t *testing.T) {
	for _, body := range []string{`{"due_at":"tomorrow"}`, `{"title":1}`, `[`} {
		var in UpdateTaskInput
		if err := json.Unmarshal([]byte(body), &in); err == nil {
			t.Errorf("decoding %s succeeded, want an error", body)
		}
	}
}
