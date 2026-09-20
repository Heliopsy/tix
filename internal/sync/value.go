package sync

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// timeLayouts are the shapes external systems write timestamps in.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000-0700",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// Lookup returns the value at a dotted path, indexing a list by a numeric
// segment. The second result reports whether the path resolved.
func Lookup(fields map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	if v, ok := fields[path]; ok {
		return v, true
	}
	var cur any = fields
	for _, seg := range strings.Split(path, ".") {
		next, ok := step(cur, seg)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func step(cur any, seg string) (any, bool) {
	switch t := cur.(type) {
	case map[string]any:
		v, ok := t[seg]
		return v, ok
	case []any:
		i, err := strconv.Atoi(seg)
		if err != nil || i < 0 || i >= len(t) {
			return nil, false
		}
		return t[i], true
	default:
		return nil, false
	}
}

// Paths returns every addressable leaf path in a record, deepest first, so an
// importer can tell which of a source's fields a mapping left untouched.
func Paths(fields map[string]any) []string {
	var out []string
	walk("", fields, &out)
	return out
}

func walk(prefix string, v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 && prefix != "" {
			*out = append(*out, prefix)
			return
		}
		for k, child := range t {
			walk(join(prefix, k), child, out)
		}
	case []any:
		if prefix != "" {
			*out = append(*out, prefix)
		}
	default:
		if prefix != "" {
			*out = append(*out, prefix)
		}
	}
}

func join(prefix, seg string) string {
	if prefix == "" {
		return seg
	}
	return prefix + "." + seg
}

// Text renders a value as the string an external system meant by it.
func Text(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(raw)
	}
}

// Strings renders a value as a list, splitting a comma-separated scalar.
func Strings(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := strings.TrimSpace(Text(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	default:
		var out []string
		for _, part := range strings.Split(Text(v), ",") {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
}

// ParseTime reads a timestamp written in any layout an external system uses.
func ParseTime(v any) (time.Time, bool) {
	if t, ok := v.(time.Time); ok {
		return t.UTC(), true
	}
	raw := strings.TrimSpace(Text(v))
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// Coerce converts an external value to the Go shape a tix field type stores.
func Coerce(t core.FieldType, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch t {
	case core.FieldInt:
		n, err := strconv.ParseInt(strings.TrimSpace(Text(v)), 10, 64)
		if err != nil {
			return nil, core.Invalid("value %q is not an integer", Text(v))
		}
		return n, nil
	case core.FieldFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(Text(v)), 64)
		if err != nil {
			return nil, core.Invalid("value %q is not a number", Text(v))
		}
		return f, nil
	case core.FieldBool:
		b, err := strconv.ParseBool(strings.TrimSpace(Text(v)))
		if err != nil {
			return nil, core.Invalid("value %q is not a boolean", Text(v))
		}
		return b, nil
	case core.FieldDate, core.FieldDateTime:
		parsed, ok := ParseTime(v)
		if !ok {
			return nil, core.Invalid("value %q is not a timestamp", Text(v))
		}
		return parsed.Format(time.RFC3339), nil
	case core.FieldJSON:
		return v, nil
	default:
		return Text(v), nil
	}
}
