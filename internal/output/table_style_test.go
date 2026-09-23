// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TestTableFollowsConfiguredTimeStyle exercises the table renderer directly
// with several format/timezone combinations against the same instant, so a
// change to how a column is rendered is caught here rather than only in a
// hand check.
func TestTableFollowsConfiguredTimeStyle(t *testing.T) {
	task := sampleTask()
	task.DueAt = nil
	task.UpdatedAt = time.Date(2026, 9, 21, 14, 5, 9, 0, time.UTC)

	cases := []struct {
		name   string
		format string
		zone   string
		want   string
	}{
		{"iso utc", TimeISO, "utc", "2026-09-21 14:05"},
		{"iso tokyo", TimeISO, "Asia/Tokyo", "2026-09-21 23:05"},
		{"us utc", TimeUS, "utc", "09/21/2026 2:05 PM"},
		{"rfc3339 tokyo", TimeRFC3339, "Asia/Tokyo", "2026-09-21T23:05:09+09:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			style, err := NewTimeStyle(c.format, c.zone)
			if err != nil {
				t.Fatalf("NewTimeStyle: %v", err)
			}
			var buf bytes.Buffer
			if err := NewWithStyle(FormatTable, ModeNever, style).Format(&buf, []core.Task{task}); err != nil {
				t.Fatalf("Format: %v", err)
			}
			if !strings.Contains(buf.String(), c.want) {
				t.Errorf("table = %s\nwant it to contain %q", buf.String(), c.want)
			}
		})
	}
}

// TestFormatCompactAndFormatClockAgreeOnZone guards the inconsistency the
// table renderer and the live stream used to have: FormatCompact rendered
// UTC while FormatClock rendered local, so the same instant printed two
// different clock times across the two surfaces. TimeStyle.Format and
// TimeStyle.Clock must always agree, because they read the same loc field.
func TestFormatCompactAndFormatClockAgreeOnZone(t *testing.T) {
	style, err := NewTimeStyle(TimeISO, "Asia/Tokyo")
	if err != nil {
		t.Fatalf("NewTimeStyle: %v", err)
	}
	full := style.Format(instant)
	clock := style.Clock(instant)
	if !strings.HasSuffix(full, clock[:5]) {
		t.Errorf("Format = %q and Clock = %q disagree on the hour:minute in Asia/Tokyo", full, clock)
	}
}

// TestMachineFormatsIgnoreTimeStyle is the invariant a helpful "improvement"
// is most likely to break later: JSON, YAML and NDJSON must render the exact
// same bytes no matter what TimeStyle a caller hands the formatter, because a
// consumer parsing a timestamp must never have to guess which zone or format
// a deployment configured.
func TestMachineFormatsIgnoreTimeStyle(t *testing.T) {
	task := sampleTask()
	styles := []TimeStyle{
		{},
		mustStyle(t, TimeISO, "utc"),
		mustStyle(t, TimeUS, "Asia/Tokyo"),
		mustStyle(t, TimeRelative, "America/New_York"),
		mustStyle(t, TimeRFC3339, "Europe/Sofia"),
	}
	for _, format := range []string{FormatJSON, FormatYAML, FormatNDJSON} {
		var baseline string
		for i, style := range styles {
			var buf bytes.Buffer
			if err := NewWithStyle(format, ModeNever, style).Format(&buf, []core.Task{task}); err != nil {
				t.Fatalf("%s: Format: %v", format, err)
			}
			got := buf.String()
			if i == 0 {
				baseline = got
				continue
			}
			if got != baseline {
				t.Errorf("%s: output changed with a different TimeStyle:\nbaseline:\n%s\ngot:\n%s", format, baseline, got)
			}
		}
	}
}

func mustStyle(t *testing.T, format, zone string) TimeStyle {
	t.Helper()
	style, err := NewTimeStyle(format, zone)
	if err != nil {
		t.Fatalf("NewTimeStyle(%q, %q): %v", format, zone, err)
	}
	return style
}
