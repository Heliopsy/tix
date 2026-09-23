// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	c := Cursor{SortValue: "2026-09-20T12:00:00Z", ID: "01JBXR8GTM4K", Sort: SortCreatedAt, Direction: Ascending}

	got, err := DecodeCursor(c.Encode())
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if got != c {
		t.Errorf("round trip = %+v, want %+v", got, c)
	}
}

// TestCursorRoundTripCompound checks the additive second sort value used by
// a compound ordering.
func TestCursorRoundTripCompound(t *testing.T) {
	c := Cursor{SortValue: "2", SortValue2: "2030-01-01T00:00:00Z", ID: "01JBXR8GTM4K", Sort: SortUrgency, Direction: Ascending}

	got, err := DecodeCursor(c.Encode())
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if got != c {
		t.Errorf("round trip = %+v, want %+v", got, c)
	}
}

// TestOldCursorDecodesWithoutTheSecondSortValue confirms a cursor encoded
// before SortValue2 existed still decodes, with the new field zero.
func TestOldCursorDecodesWithoutTheSecondSortValue(t *testing.T) {
	old := Cursor{SortValue: "x", ID: "1", Sort: SortCreatedAt, Direction: Ascending}
	token := old.Encode()

	got, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if got.SortValue2 != "" {
		t.Errorf("SortValue2 = %q, want empty for a single-key cursor", got.SortValue2)
	}
	if got != old {
		t.Errorf("decoded = %+v, want %+v", got, old)
	}
}

func TestCursorZero(t *testing.T) {
	if !(Cursor{}).Zero() {
		t.Error("empty cursor should be zero")
	}
	if (Cursor{}).Encode() != "" {
		t.Error("zero cursor encodes to the empty token")
	}
	got, err := DecodeCursor("")
	if err != nil || !got.Zero() {
		t.Errorf("DecodeCursor(\"\") = %+v, %v; want the zero cursor", got, err)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, in := range []string{
		"not-base64!!",
		"////",
		"YWJjZA", // valid base64, not JSON
		"e30",    // "{}" decodes to a zero cursor
		"bnVsbA", // "null"
		"../../etc/passwd",
	} {
		if got, err := DecodeCursor(in); err == nil {
			t.Errorf("DecodeCursor(%q) = %+v, want an error", in, got)
		}
	}
}

func TestCursorCheckOrdering(t *testing.T) {
	c := Cursor{SortValue: "x", ID: "1", Sort: SortCreatedAt, Direction: Ascending}

	if err := c.CheckOrdering(SortCreatedAt, Ascending); err != nil {
		t.Errorf("matching ordering should be accepted, got %v", err)
	}
	if err := c.CheckOrdering(SortPriority, Ascending); err == nil {
		t.Error("a cursor from a different sort field must be rejected")
	}
	if err := c.CheckOrdering(SortCreatedAt, Descending); err == nil {
		t.Error("a cursor from a different direction must be rejected")
	}
	if err := (Cursor{}).CheckOrdering(SortPriority, Descending); err != nil {
		t.Errorf("the zero cursor matches any ordering, got %v", err)
	}
}

func TestPageNormalize(t *testing.T) {
	tests := []struct {
		name      string
		in        Page
		wantLimit int
		wantDir   SortDirection
		wantErr   bool
	}{
		{"defaults", Page{}, DefaultPageLimit, Ascending, false},
		{"explicit limit", Page{Limit: 10}, 10, Ascending, false},
		{"over-large limit is clamped", Page{Limit: 100000}, MaxPageLimit, Ascending, false},
		{"exactly max", Page{Limit: MaxPageLimit}, MaxPageLimit, Ascending, false},
		{"descending kept", Page{Direction: Descending}, DefaultPageLimit, Descending, false},
		{"negative limit rejected", Page{Limit: -1}, 0, "", true},
		{"unknown direction rejected", Page{Direction: "sideways"}, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Normalize()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got.Limit != tt.wantLimit || got.Direction != tt.wantDir {
				t.Errorf("Normalize() = {%d, %q}, want {%d, %q}",
					got.Limit, got.Direction, tt.wantLimit, tt.wantDir)
			}
		})
	}
}

func TestTaskFilterValidateDefaults(t *testing.T) {
	got, err := TaskFilter{}.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got.Page.Sort != SortUrgency {
		t.Errorf("default sort = %q, want %q", got.Page.Sort, SortUrgency)
	}
	if got.Page.Limit != DefaultPageLimit {
		t.Errorf("default limit = %d, want %d", got.Page.Limit, DefaultPageLimit)
	}
}

func TestTaskFilterValidateRejects(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   TaskFilter
	}{
		{"parent and parent-is-null", TaskFilter{ParentID: "t1", ParentIsNull: true}},
		{"priority out of range", TaskFilter{Priorities: []Priority{99}}},
		{"priority zero", TaskFilter{Priorities: []Priority{0}}},
		{"unsortable field", TaskFilter{Page: Page{Sort: "custom_fields"}}},
		{"injection in sort field", TaskFilter{Page: Page{Sort: "created_at; DROP TABLE tasks"}}},
		{"inverted due window", TaskFilter{DueBefore: &past, DueAfter: &future}},
		{"bad cursor", TaskFilter{Page: Page{Cursor: "garbage!!"}}},
		{"negative limit", TaskFilter{Page: Page{Limit: -5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.in.Validate(); err == nil {
				t.Error("expected an error")
			} else if !IsKind(err, KindInvalid) {
				t.Errorf("error kind = %q, want invalid", KindOf(err))
			}
		})
	}
}

// TestTaskFilterRejectsCursorFromBeforeTheDefaultChanged confirms a cursor
// minted under the old created_at default is refused, rather than silently
// misread, once no explicit sort is given and the filter defaults to urgency.
func TestTaskFilterRejectsCursorFromBeforeTheDefaultChanged(t *testing.T) {
	old := Cursor{SortValue: "2026-01-01T00:00:00Z", ID: "1", Sort: SortCreatedAt, Direction: Ascending}
	f := TaskFilter{Page: Page{Cursor: old.Encode()}}

	if _, err := f.Validate(); err == nil {
		t.Error("a cursor produced under the previous default sort must be rejected, not reinterpreted")
	} else if !IsKind(err, KindInvalid) {
		t.Errorf("error kind = %q, want invalid", KindOf(err))
	}
}

func TestTaskFilterRejectsMismatchedCursor(t *testing.T) {
	c := Cursor{SortValue: "x", ID: "1", Sort: SortCreatedAt, Direction: Ascending}
	f := TaskFilter{Page: Page{Sort: SortPriority, Cursor: c.Encode()}}

	if _, err := f.Validate(); err == nil {
		t.Error("a cursor from a different ordering must be rejected")
	}
}

func TestTaskFilterAcceptsMatchingCursor(t *testing.T) {
	c := Cursor{SortValue: "x", ID: "1", Sort: SortPriority, Direction: Descending}
	f := TaskFilter{Page: Page{Sort: SortPriority, Direction: Descending, Cursor: c.Encode()}}

	if _, err := f.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestEveryTaskSortFieldValidates(t *testing.T) {
	for _, field := range TaskSortFields {
		f := TaskFilter{Page: Page{Sort: field}}
		if _, err := f.Validate(); err != nil {
			t.Errorf("sort by %q should be accepted, got %v", field, err)
		}
	}
}

func TestTriState(t *testing.T) {
	tests := []struct {
		state    TriState
		in, want bool
	}{
		{Either, true, true},
		{Either, false, true},
		{Yes, true, true},
		{Yes, false, false},
		{No, true, false},
		{No, false, true},
	}
	for _, tt := range tests {
		if got := tt.state.Match(tt.in); got != tt.want {
			t.Errorf("TriState(%d).Match(%v) = %v, want %v", tt.state, tt.in, got, tt.want)
		}
	}
	if Either != 0 {
		t.Error("Either must be the zero value")
	}
}

func TestEventFilterMatches(t *testing.T) {
	ev := Event{Type: EventTaskClaimed, ProjectID: "p1"}

	tests := []struct {
		name   string
		filter EventFilter
		want   bool
	}{
		{"empty filter matches everything", EventFilter{}, true},
		{"exact type", EventFilter{Types: []EventType{EventTaskClaimed}}, true},
		{"other type", EventFilter{Types: []EventType{EventTaskDeleted}}, false},
		{"prefix wildcard", EventFilter{Types: []EventType{"task.*"}}, true},
		{"non-matching prefix", EventFilter{Types: []EventType{"project.*"}}, false},
		{"bare wildcard", EventFilter{Types: []EventType{"*"}}, true},
		{"one of several types", EventFilter{Types: []EventType{EventTaskDeleted, EventTaskClaimed}}, true},
		{"matching project", EventFilter{ProjectIDs: []string{"p1"}}, true},
		{"other project", EventFilter{ProjectIDs: []string{"p2"}}, false},
		{"project and type both match", EventFilter{ProjectIDs: []string{"p1"}, Types: []EventType{"task.*"}}, true},
		{"project matches but type does not", EventFilter{ProjectIDs: []string{"p1"}, Types: []EventType{"project.*"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Matches(ev); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortDirectionValid(t *testing.T) {
	if !Ascending.Valid() || !Descending.Valid() {
		t.Error("asc and desc should be valid")
	}
	if SortDirection("random").Valid() || SortDirection("").Valid() {
		t.Error("unknown directions must be invalid")
	}
}
