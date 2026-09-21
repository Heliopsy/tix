package sql

import (
	stdsql "database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TimeLayout is fixed-width RFC3339 in UTC, so lexicographic order equals chronological order.
const TimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// TimeText renders t for storage.
func TimeText(t time.Time) string { return t.UTC().Format(TimeLayout) }

// NullTimeText renders an optional time for storage, yielding NULL when absent.
func NullTimeText(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return TimeText(*t)
}

// ParseTime reads a stored timestamp, accepting every layout the schema has carried.
func ParseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{TimeLayout, time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, core.Internal("parsing stored timestamp %q", s)
}

// ScanTime converts a possibly null timestamp column to a value.
func ScanTime(v stdsql.NullString) (time.Time, error) {
	if !v.Valid {
		return time.Time{}, nil
	}
	return ParseTime(v.String)
}

// ScanNullTime converts an optional timestamp column to a pointer.
func ScanNullTime(v stdsql.NullString) (*time.Time, error) {
	if !v.Valid || v.String == "" {
		return nil, nil
	}
	t, err := ParseTime(v.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Text returns the string held by an optional column, or the empty string.
func Text(v stdsql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// NullText renders an empty string as a NULL column value.
func NullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Bool converts a stored integer boolean.
func Bool(n int64) bool { return n != 0 }

// BoolInt renders a boolean for storage.
func BoolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// JSONText marshals a value for a JSON column, substituting fallback when empty.
func JSONText(v any, fallback string) (string, error) {
	if v == nil {
		return fallback, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encoding json column: %w", err)
	}
	if len(b) == 0 || string(b) == "null" {
		return fallback, nil
	}
	return string(b), nil
}

// ParseJSON unmarshals a JSON column into out, tolerating an empty column.
func ParseJSON(s string, out any) error {
	if s == "" || s == "null" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), out); err != nil {
		return fmt.Errorf("decoding json column: %w", err)
	}
	return nil
}
