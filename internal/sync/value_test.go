// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestLookupWalksNestedRecords(t *testing.T) {
	var fields map[string]any
	raw := `{"key":"OPS-1","fields":{"summary":"ship it","labels":["a","b"],
		"status":{"name":"In Progress"}}}`
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"key", "OPS-1", true},
		{"fields.summary", "ship it", true},
		{"fields.status.name", "In Progress", true},
		{"fields.labels.1", "b", true},
		{"fields.missing", "", false},
		{"fields.labels.9", "", false},
		{"", "", false},
		{"key.deeper", "", false},
	}
	for _, tc := range tests {
		got, ok := Lookup(fields, tc.path)
		if ok != tc.ok {
			t.Errorf("Lookup(%q) ok = %v, want %v", tc.path, ok, tc.ok)
			continue
		}
		if ok && Text(got) != tc.want {
			t.Errorf("Lookup(%q) = %q, want %q", tc.path, Text(got), tc.want)
		}
	}
}

func TestPathsEnumeratesLeaves(t *testing.T) {
	fields := map[string]any{
		"id":     "1",
		"nested": map[string]any{"a": "x", "b": map[string]any{}},
		"list":   []any{"one"},
	}
	want := map[string]bool{"id": true, "nested.a": true, "nested.b": true, "list": true}
	got := Paths(fields)
	if len(got) != len(want) {
		t.Fatalf("Paths() = %v, want %d entries", got, len(want))
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("Paths() produced unexpected %q", p)
		}
	}
}

func TestTextAndStrings(t *testing.T) {
	if got := Text(nil); got != "" {
		t.Errorf("Text(nil) = %q", got)
	}
	if got := Text(true); got != "true" {
		t.Errorf("Text(true) = %q", got)
	}
	if got := Text(3.5); got != "3.5" {
		t.Errorf("Text(3.5) = %q", got)
	}
	if got := Text(7); got != "7" {
		t.Errorf("Text(7) = %q", got)
	}
	if got := Text(int64(8)); got != "8" {
		t.Errorf("Text(int64) = %q", got)
	}
	if got := Text(json.Number("9")); got != "9" {
		t.Errorf("Text(json.Number) = %q", got)
	}
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := Text(when); got != "2026-01-02T03:04:05Z" {
		t.Errorf("Text(time) = %q", got)
	}
	if got := Text(map[string]any{"a": 1}); got != `{"a":1}` {
		t.Errorf("Text(map) = %q", got)
	}

	if got := Strings([]any{"a", " b ", ""}); len(got) != 2 || got[1] != "b" {
		t.Errorf("Strings(list) = %v", got)
	}
	if got := Strings("a, b"); len(got) != 2 || got[0] != "a" {
		t.Errorf("Strings(csv) = %v", got)
	}
	if got := Strings(nil); got != nil {
		t.Errorf("Strings(nil) = %v", got)
	}
	if got := Strings([]string{"x"}); len(got) != 1 {
		t.Errorf("Strings([]string) = %v", got)
	}
}

func TestParseTimeAcceptsCommonLayouts(t *testing.T) {
	for _, raw := range []string{
		"2026-01-02T03:04:05Z",
		"2026-01-02T03:04:05.000+0000",
		"2026-01-02 03:04:05",
		"2026-01-02",
	} {
		if _, ok := ParseTime(raw); !ok {
			t.Errorf("ParseTime(%q) failed", raw)
		}
	}
	if _, ok := ParseTime("not a date"); ok {
		t.Error("ParseTime accepted rubbish")
	}
	if _, ok := ParseTime(""); ok {
		t.Error("ParseTime accepted an empty value")
	}
	when := time.Now()
	if got, ok := ParseTime(when); !ok || got.Location() != time.UTC {
		t.Errorf("ParseTime(time.Time) = %v %v", got, ok)
	}
}

func TestCoerceConvertsToFieldTypes(t *testing.T) {
	tests := []struct {
		typ     core.FieldType
		in      any
		want    any
		wantErr bool
	}{
		{core.FieldInt, "42", int64(42), false},
		{core.FieldInt, "x", nil, true},
		{core.FieldFloat, "3.5", 3.5, false},
		{core.FieldFloat, "x", nil, true},
		{core.FieldBool, "true", true, false},
		{core.FieldBool, "maybe", nil, true},
		{core.FieldString, 12, "12", false},
		{core.FieldJSON, map[string]any{"a": 1}, map[string]any{"a": 1}, false},
	}
	for _, tc := range tests {
		got, err := Coerce(tc.typ, tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Coerce(%s, %v) accepted a bad value", tc.typ, tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Coerce(%s, %v): %v", tc.typ, tc.in, err)
			continue
		}
		if tc.typ != core.FieldJSON && got != tc.want {
			t.Errorf("Coerce(%s, %v) = %v, want %v", tc.typ, tc.in, got, tc.want)
		}
	}
	if got, err := Coerce(core.FieldDateTime, "2026-01-02"); err != nil || got != "2026-01-02T00:00:00Z" {
		t.Errorf("Coerce(datetime) = %v, %v", got, err)
	}
	if _, err := Coerce(core.FieldDate, "nope"); err == nil {
		t.Error("Coerce(date) accepted rubbish")
	}
	if got, err := Coerce(core.FieldString, nil); got != nil || err != nil {
		t.Errorf("Coerce(nil) = %v, %v", got, err)
	}
}
