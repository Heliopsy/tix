// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/heliopsy/tix/internal/core"
)

// dateLayouts are the timestamp forms a flag value may take.
var dateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"}

// parsePriority accepts a name or a number between 1 and 5.
func parsePriority(value string) (core.Priority, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return 0, nil
	case "highest":
		return core.PriorityHighest, nil
	case "high":
		return core.PriorityHigh, nil
	case "normal", "medium":
		return core.PriorityNormal, nil
	case "low":
		return core.PriorityLow, nil
	case "lowest":
		return core.PriorityLowest, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, core.Invalid("priority %q must be a name or a number from 1 to 5", value)
	}
	p := core.Priority(n)
	if !p.Valid() {
		return 0, core.Invalid("priority %d is out of range", n)
	}
	return p, nil
}

// parseTime accepts a date or a timestamp.
func parseTime(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, core.Invalid("time %q must be RFC3339 or YYYY-MM-DD", value)
}

// parseDuration accepts a Go duration such as "15m".
func parseDuration(value string) (core.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, core.Invalid("duration %q must be a Go duration such as 15m", value)
	}
	if d < 0 {
		return 0, core.Invalid("duration %q must not be negative", value)
	}
	return core.Duration(d), nil
}

// parseFields turns repeated key=value flags into custom field values, decoding
// each value as JSON when it parses and leaving it a string otherwise.
func parseFields(pairs []string) (map[string]any, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(pairs))
	for _, pair := range pairs {
		key, raw, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, core.Invalid("field %q must be given as key=value", pair)
		}
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			out[strings.TrimSpace(key)] = decoded
			continue
		}
		out[strings.TrimSpace(key)] = raw
	}
	return out, nil
}

// parseRef parses a single task reference.
func parseRef(value string) (core.TaskRef, error) { return core.ParseTaskRef(value) }

// decodeFile unmarshals a YAML or JSON document into v, since JSON is valid YAML.
func decodeFile(data []byte, v any) error {
	if err := yaml.Unmarshal(data, v); err != nil {
		return core.Invalid("parsing document: %v", err)
	}
	return nil
}
