// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"strings"
	"testing"
	"time"
)

func TestNewTimeStyleRefusesUnknownFormatOrZone(t *testing.T) {
	if _, err := NewTimeStyle("nonsense", "utc"); err == nil {
		t.Fatal("expected an error for an unknown format")
	}
	if _, err := NewTimeStyle("iso", "Nowhere/Fake"); err == nil {
		t.Fatal("expected an error for an unknown timezone")
	}
}

func TestNewTimeStyleDefaultsFormatToISO(t *testing.T) {
	style, err := NewTimeStyle("", "utc")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	if got := style.Format(time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)); got != "2026-03-04 05:06" {
		t.Errorf("Format = %q, want the iso layout", got)
	}
}

func TestNewTimeStyleAcceptsLocalAndUTC(t *testing.T) {
	if _, err := NewTimeStyle("iso", "local"); err != nil {
		t.Errorf("local should be accepted: %v", err)
	}
	if _, err := NewTimeStyle("iso", ""); err != nil {
		t.Errorf("empty timezone should default to local: %v", err)
	}
	if _, err := NewTimeStyle("iso", "utc"); err != nil {
		t.Errorf("utc should be accepted: %v", err)
	}
}

// instant is a fixed point in time used across the named-format examples, so
// each format's rendering is documented and pinned by a single value.
var instant = time.Date(2026, 9, 21, 14, 5, 9, 0, time.UTC)

func TestNamedFormatsRenderTheDocumentedExamples(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		{TimeISO, "2026-09-21 14:05"},
		{TimeRFC3339, "2026-09-21T14:05:09Z"},
		{TimeShort, "21 Sep 14:05"},
		{TimeUS, "09/21/2026 2:05 PM"},
	}
	for _, c := range cases {
		style, err := NewTimeStyle(c.format, "utc")
		if err != nil {
			t.Fatalf("%s: NewTimeStyle: %v", c.format, err)
		}
		if got := style.Format(instant); got != c.want {
			t.Errorf("%s: Format = %q, want %q", c.format, got, c.want)
		}
	}
}

func TestFormatFollowsTheConfiguredZoneAcrossHours(t *testing.T) {
	style, err := NewTimeStyle(TimeISO, "Asia/Tokyo")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	// UTC 14:05 on the 21st is 23:05 the same day in Tokyo (+9): a zone bug
	// that silently kept UTC would show 14:05, not 23:05.
	if got := style.Format(instant); got != "2026-09-21 23:05" {
		t.Errorf("Format in Asia/Tokyo = %q, want 2026-09-21 23:05", got)
	}
}

func TestFormatZeroTimeIsEmpty(t *testing.T) {
	style, err := NewTimeStyle(TimeISO, "utc")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	if got := style.Format(time.Time{}); got != "" {
		t.Errorf("Format(zero) = %q, want empty: a zero time means never happened, not 1970", got)
	}
	if got := style.FormatPtr(nil); got != "" {
		t.Errorf("FormatPtr(nil) = %q, want empty", got)
	}
	if got := style.Absolute(time.Time{}); got != "" {
		t.Errorf("Absolute(zero) = %q, want empty", got)
	}
	if got := style.Clock(time.Time{}); got != "" {
		t.Errorf("Clock(zero) = %q, want empty", got)
	}
	if got := style.Relative(time.Time{}); got != "" {
		t.Errorf("Relative(zero) = %q, want empty", got)
	}
}

// A tooltip is the reader's own layout, except for "relative", where a second
// relative string would say nothing the visible one does not already say.
func TestAbsoluteKeepsTheChosenLayoutAndEscapesOnlyRelative(t *testing.T) {
	cases := map[string]string{
		TimeISO:      "2026-09-21 14:05",
		TimeRFC3339:  "2026-09-21T14:05:09Z",
		TimeShort:    "21 Sep 14:05",
		TimeUS:       "09/21/2026 2:05 PM",
		TimeRelative: "2026-09-21 14:05",
	}
	for format, want := range cases {
		style, err := NewTimeStyle(format, "utc")
		if err != nil {
			t.Fatalf("%s: NewTimeStyle: %v", format, err)
		}
		if got := style.Absolute(instant); got != want {
			t.Errorf("%s: Absolute = %q, want %q", format, got, want)
		}
	}
}

func TestClockFollowsTheConfiguredZone(t *testing.T) {
	style, err := NewTimeStyle(TimeISO, "Asia/Tokyo")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	if got := style.Clock(instant); got != "23:05:09" {
		t.Errorf("Clock in Asia/Tokyo = %q, want 23:05:09", got)
	}
}

func TestRelativeRendersPastAndFutureFromAnInjectedClock(t *testing.T) {
	style, err := NewTimeStyle(TimeRelative, "utc")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	style.now = func() time.Time { return now }

	past := now.Add(-18 * time.Minute)
	if got := style.Relative(past); got != "18 minutes ago" {
		t.Errorf("Relative(past) = %q, want 18 minutes ago", got)
	}
	if got := style.Format(past); got != "18 minutes ago" {
		t.Errorf("Format with the relative named format = %q, want 18 minutes ago", got)
	}
	future := now.Add(2 * time.Hour)
	if got := style.Relative(future); got != "in 2 hours" {
		t.Errorf("Relative(future) = %q, want in 2 hours", got)
	}
}

func TestRelativeIsIndependentOfTheConfiguredNamedFormat(t *testing.T) {
	style, err := NewTimeStyle(TimeISO, "utc")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	style.now = func() time.Time { return now }
	if got := style.Relative(now.Add(-90 * time.Second)); got != "1 minute ago" {
		t.Errorf("Relative = %q, want 1 minute ago even though the named format is iso", got)
	}
}

func TestTimeFormatsListsOnlyImplementedNames(t *testing.T) {
	for _, f := range TimeFormats {
		if _, err := NewTimeStyle(f, "utc"); err != nil {
			t.Errorf("%s: listed in TimeFormats but refused: %v", f, err)
		}
	}
	if !strings.Contains(strings.Join(TimeFormats, ","), TimeRelative) {
		t.Error("TimeFormats should list relative")
	}
}

// TestRelativeNeverSaysZero pins the gap between the just-now cutoff and a
// whole minute, which truncation used to render as "0 minutes ago".
func TestRelativeNeverSaysZero(t *testing.T) {
	base := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := TimeStyle{format: TimeRelative, loc: time.UTC, now: func() time.Time { return base }}
	for _, d := range []time.Duration{
		46 * time.Second, 59 * time.Second,
		61 * time.Minute, 25 * time.Hour,
		31 * 24 * time.Hour, 366 * 24 * time.Hour,
	} {
		for _, at := range []time.Time{base.Add(-d), base.Add(d)} {
			if got := s.Format(at); strings.Contains(got, "0 ") {
				t.Fatalf("delta %s rendered %q, which says zero", d, got)
			}
		}
	}
}
