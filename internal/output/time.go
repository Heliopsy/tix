package output

import "time"

// CompactTimeLayout is the timestamp layout used by table output.
const CompactTimeLayout = "2006-01-02 15:04"

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
