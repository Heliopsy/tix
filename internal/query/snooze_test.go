// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestParseSnoozeTerms(t *testing.T) {
	tests := []struct {
		expr    string
		snoozed core.TriState
		include bool
	}{
		{"", core.Either, false},
		{"is:snoozed", core.Yes, false},
		{"snoozed:true", core.Yes, false},
		{"-is:snoozed", core.No, false},
		{"is:awake", core.No, false},
		{"-is:awake", core.Yes, false},
		{"snoozed:false", core.No, false},
		{"-snoozed:true", core.No, false},
		{"snoozed:any", core.Either, true},
		{"snoozed:ANY", core.Either, true},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			f, err := Parse(tc.expr)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.expr, err)
			}
			if f.Snoozed != tc.snoozed || f.IncludeSnoozed != tc.include {
				t.Fatalf("Parse(%q) = snoozed %v include %v, want %v %v",
					tc.expr, f.Snoozed, f.IncludeSnoozed, tc.snoozed, tc.include)
			}
		})
	}
}

func TestParseRefusesASnoozeTermItCannotAnswer(t *testing.T) {
	for _, expr := range []string{"snoozed:maybe", "-snoozed:any", "is:dozing"} {
		if _, err := Parse(expr); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("Parse(%q) = %v, want invalid", expr, err)
		}
	}
}

// The terminal filters a loaded page with Matches, so the default that hides
// a snoozed task has to hold there as it does in Service.ListTasks.
func TestMatchesAnswersTheSnoozeTermsAndTheDefault(t *testing.T) {
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	later, earlier := now.Add(time.Hour), now.Add(-time.Hour)
	tasks := map[string]core.Task{
		"asleep": {Title: "asleep", SnoozedUntil: &later},
		"woke":   {Title: "woke", SnoozedUntil: &earlier},
		"never":  {Title: "never"},
	}
	tests := []struct {
		expr string
		want []string
	}{
		{"", []string{"never", "woke"}},
		{"is:snoozed", []string{"asleep"}},
		{"-is:snoozed", []string{"never", "woke"}},
		{"snoozed:any", []string{"asleep", "never", "woke"}},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			f, err := Parse(tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for name, task := range tasks {
				if Matches(f, task, "infra", now) {
					got = append(got, name)
				}
			}
			sort.Strings(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("%q matches %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestParseUntil(t *testing.T) {
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Skip("no zone database")
	}
	// A Wednesday afternoon in Sofia.
	now := time.Date(2026, 11, 4, 14, 20, 0, 0, sofia)
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, sofia) }
	tests := []struct {
		in   string
		want time.Time
	}{
		{"later", at(2026, 11, 4, 18, 0)},
		{"later-today", at(2026, 11, 4, 18, 0)},
		{"tomorrow", at(2026, 11, 5, 9, 0)},
		{"next-week", at(2026, 11, 9, 9, 0)},
		{"NEXT-WEEK", at(2026, 11, 9, 9, 0)},
		{"90m", now.Add(90 * time.Minute)},
		{"2h", now.Add(2 * time.Hour)},
		{"3d", now.Add(72 * time.Hour)},
		{"1w", now.Add(7 * 24 * time.Hour)},
		{"2026-12-01", at(2026, 12, 1, 9, 0)},
		{"2026-12-01 14:30", at(2026, 12, 1, 14, 30)},
		{"2026-12-01T14:30", at(2026, 12, 1, 14, 30)},
		{"2026-12-01T14:30:00Z", time.Date(2026, 12, 1, 14, 30, 0, 0, time.UTC)},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseUntil(tc.in, now, sofia)
			if err != nil {
				t.Fatalf("ParseUntil(%q): %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("ParseUntil(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
	for _, bad := range []string{"", "someday", "0d", "-2h", "3x"} {
		if _, err := ParseUntil(bad, now, sofia); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("ParseUntil(%q) = %v, want invalid", bad, err)
		}
	}
}

func TestPresetsRoundLaterUpToTheHourAndSkipToNextMonday(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skip("no zone database")
	}
	monday := time.Date(2026, 11, 2, 8, 0, 0, 0, kolkata)
	if got, _ := Preset(UntilNextWeek, monday, kolkata); !got.Equal(time.Date(2026, 11, 9, 9, 0, 0, 0, kolkata)) {
		t.Errorf("next week from a Monday = %v, want the Monday after", got)
	}
	onTheHour := time.Date(2026, 11, 2, 8, 0, 0, 0, kolkata)
	if got, _ := Preset(UntilLater, onTheHour, kolkata); !got.Equal(time.Date(2026, 11, 2, 11, 0, 0, 0, kolkata)) {
		t.Errorf("later from 08:00 = %v, want 11:00 in a half-hour zone", got)
	}
	odd := time.Date(2026, 11, 2, 8, 10, 0, 0, kolkata)
	if got, _ := Preset(UntilLater, odd, kolkata); !got.Equal(time.Date(2026, 11, 2, 12, 0, 0, 0, kolkata)) {
		t.Errorf("later from 08:10 = %v, want 12:00", got)
	}
	if _, ok := Preset("someday", odd, kolkata); ok {
		t.Error("an unknown preset resolved")
	}
}
