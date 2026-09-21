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

// NewWithMode returns a Formatter that colours according to mode. Only the
// table format ever colours: the machine-readable formats are piped into other
// programs, where an escape code is a bug.
func NewWithMode(format string, mode Mode) Formatter {
	switch format {
	case FormatJSON:
		return &jsonFormatter{}
	case FormatYAML:
		return &yamlFormatter{}
	case FormatNDJSON:
		return &ndjsonFormatter{}
	default:
		return &tableFormatter{mode: mode}
	}
}
