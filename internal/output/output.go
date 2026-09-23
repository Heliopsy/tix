// SPDX-License-Identifier: AGPL-3.0-or-later

// Package output renders domain values as tables, JSON or YAML for tix commands.
package output

import "io"

// Output formats.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

// Formats lists every supported output format.
var Formats = []string{FormatTable, FormatJSON, FormatYAML, FormatNDJSON}

// Formatter renders structured data to a writer.
type Formatter interface {
	Format(w io.Writer, data any) error
}

// New returns a Formatter for the given format, defaulting to table, with
// colour decided by the writer it is handed.
func New(format string) Formatter { return NewWithMode(format, ModeAuto) }

// NewWithMode returns a Formatter that colours according to mode, rendering
// any timestamp with the zero-value TimeStyle. Prefer NewWithStyle wherever a
// caller has resolved the configured time format and timezone.
func NewWithMode(format string, mode Mode) Formatter {
	return NewWithStyle(format, mode, TimeStyle{})
}

// NewWithStyle returns a Formatter that colours according to mode and renders
// every human-readable timestamp through style. Only the table format ever
// colours or consults style: JSON, YAML and NDJSON always carry RFC 3339 in
// UTC, so a consumer parsing one never has to guess which zone or format a
// deployment configured.
func NewWithStyle(format string, mode Mode, style TimeStyle) Formatter {
	switch format {
	case FormatJSON:
		return &jsonFormatter{}
	case FormatYAML:
		return &yamlFormatter{}
	case FormatNDJSON:
		return &ndjsonFormatter{}
	default:
		return &tableFormatter{mode: mode, style: style}
	}
}
