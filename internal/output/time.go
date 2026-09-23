// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import "time"

// CompactTimeLayout is the timestamp layout used by table output.
const CompactTimeLayout = "2006-01-02 15:04"

// ClockTimeLayout is the timestamp layout a live tail renders each line with.
// A running stream has no need for the date on every line, only the clock.
const ClockTimeLayout = "15:04:05"

// FormatClock renders t as a local time of day, for a line in a live stream.
func FormatClock(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(ClockTimeLayout)
}

// FormatTimestamp renders t as RFC3339, or empty for the zero time.
func FormatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// FormatTimestampPtr renders t as RFC3339, or empty for nil or the zero time.
func FormatTimestampPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return FormatTimestamp(*t)
}

// FormatCompact renders t in the compact table layout, or empty for the zero time.
func FormatCompact(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(CompactTimeLayout)
}

// FormatCompactPtr renders t in the compact table layout, or empty for nil or the zero time.
func FormatCompactPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return FormatCompact(*t)
}
