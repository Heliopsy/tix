// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// Comment is one comment as an export carries it.
type Comment struct {
	Author string    `json:"author"`
	At     time.Time `json:"created_at"`
	Body   string    `json:"body"`
}

// Change is one history entry as an export carries it.
type Change struct {
	Actor  string    `json:"actor"`
	At     time.Time `json:"at"`
	Action string    `json:"action"`
}

// WriteOptions shapes the cells a Writer writes.
type WriteOptions struct {
	// Delimiter separates CSV cells; zero is a comma.
	Delimiter rune
	// Date renders an instant; nil is RFC 3339 in UTC.
	Date func(time.Time) string
}

// Writer writes one task per call as it is handed them, so an export of any
// size streams in constant memory.
type Writer interface {
	// Write writes one task's cells, one per column in column order.
	Write(values []any) error
	// Close finishes the document and flushes it.
	Close() error
}

// NewWriter starts a document in the format with the columns as its header.
func NewWriter(w io.Writer, format core.TaskFormat, columns []string, opts WriteOptions) (Writer, error) {
	if opts.Date == nil {
		opts.Date = func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	}
	if opts.Delimiter == 0 {
		opts.Delimiter = ','
	}
	switch format {
	case core.TaskFormatCSV:
		return newCSVWriter(w, columns, opts)
	case core.TaskFormatJSON, core.TaskFormatNDJSON:
		return newJSONWriter(w, columns, opts, format == core.TaskFormatJSON)
	case core.TaskFormatMarkdown:
		return newMarkdownWriter(w, columns, opts)
	default:
		return nil, core.Invalid("export format %q is not supported", format)
	}
}

type csvWriter struct {
	w    *csv.Writer
	opts WriteOptions
}

func newCSVWriter(w io.Writer, columns []string, opts WriteOptions) (*csvWriter, error) {
	cw := csv.NewWriter(w)
	cw.Comma = opts.Delimiter
	if err := cw.Write(columns); err != nil {
		return nil, fmt.Errorf("writing csv header: %w", err)
	}
	return &csvWriter{w: cw, opts: opts}, nil
}

// Write guards every cell, because a CSV file is read by spreadsheets.
func (c *csvWriter) Write(values []any) error {
	row := make([]string, len(values))
	for i, v := range values {
		row[i] = output.GuardCell(Text(v, c.opts.Date))
	}
	if err := c.w.Write(row); err != nil {
		return fmt.Errorf("writing csv row: %w", err)
	}
	c.w.Flush()
	return c.w.Error()
}

func (c *csvWriter) Close() error {
	c.w.Flush()
	return c.w.Error()
}

type jsonWriter struct {
	w       *bufio.Writer
	columns []string
	opts    WriteOptions
	array   bool
	rows    int
}

func newJSONWriter(w io.Writer, columns []string, opts WriteOptions, array bool) (*jsonWriter, error) {
	jw := &jsonWriter{w: bufio.NewWriter(w), columns: columns, opts: opts, array: array}
	if array {
		if _, err := jw.w.WriteString("["); err != nil {
			return nil, err
		}
	}
	return jw, nil
}

func (j *jsonWriter) Write(values []any) error {
	obj := orderedObject{keys: j.columns, values: make([]any, len(values))}
	for i, v := range values {
		obj.values[i] = jsonValue(v, j.opts.Date)
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("encoding task: %w", err)
	}
	sep := "\n"
	if j.array && j.rows > 0 {
		sep = ",\n"
	}
	if !j.array {
		sep = ""
	}
	j.rows++
	if _, err := j.w.WriteString(sep); err != nil {
		return err
	}
	if _, err := j.w.Write(raw); err != nil {
		return err
	}
	if !j.array {
		if err := j.w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return j.w.Flush()
}

func (j *jsonWriter) Close() error {
	if j.array {
		end := "\n]\n"
		if j.rows == 0 {
			end = "]\n"
		}
		if _, err := j.w.WriteString(end); err != nil {
			return err
		}
	}
	return j.w.Flush()
}

// orderedObject is a JSON object whose keys keep the export's column order.
type orderedObject struct {
	keys   []string
	values []any
}

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(o.values[i])
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(val)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

func jsonValue(v any, date func(time.Time) string) any {
	switch t := v.(type) {
	case nil:
		return nil
	case time.Time:
		return date(t)
	case *time.Time:
		if t == nil {
			return nil
		}
		return date(*t)
	case []string:
		if t == nil {
			return []string{}
		}
		return t
	case []Comment:
		out := make([]map[string]string, len(t))
		for i, c := range t {
			out[i] = map[string]string{"author": c.Author, "created_at": date(c.At), "body": c.Body}
		}
		return out
	case []Change:
		out := make([]map[string]string, len(t))
		for i, c := range t {
			out[i] = map[string]string{"actor": c.Actor, "at": date(c.At), "action": c.Action}
		}
		return out
	default:
		return v
	}
}

type markdownWriter struct {
	w    *bufio.Writer
	opts WriteOptions
}

func newMarkdownWriter(w io.Writer, columns []string, opts WriteOptions) (*markdownWriter, error) {
	mw := &markdownWriter{w: bufio.NewWriter(w), opts: opts}
	head := make([]string, len(columns))
	rule := make([]string, len(columns))
	for i, c := range columns {
		head[i] = markdownCell(c)
		rule[i] = "---"
	}
	if _, err := fmt.Fprintf(mw.w, "| %s |\n| %s |\n", strings.Join(head, " | "), strings.Join(rule, " | ")); err != nil {
		return nil, err
	}
	return mw, nil
}

func (m *markdownWriter) Write(values []any) error {
	cells := make([]string, len(values))
	for i, v := range values {
		cells[i] = markdownCell(Text(v, m.opts.Date))
	}
	if _, err := fmt.Fprintf(m.w, "| %s |\n", strings.Join(cells, " | ")); err != nil {
		return err
	}
	return m.w.Flush()
}

func (m *markdownWriter) Close() error { return m.w.Flush() }

// markdownCell keeps a cell on its table row: a pipe would end it and a line
// break would end the row.
func markdownCell(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "<br>")
}

// ListSeparator joins a list cell, such as tags or dependencies, in a text
// format. An import splits on it.
const ListSeparator = ", "

// Text is a value as one text cell.
func Text(v any, date func(time.Time) string) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case time.Time:
		return date(t)
	case *time.Time:
		if t == nil {
			return ""
		}
		return date(*t)
	case []string:
		return strings.Join(t, ListSeparator)
	case []Comment:
		parts := make([]string, len(t))
		for i, c := range t {
			parts[i] = fmt.Sprintf("%s (%s): %s", c.Author, date(c.At), c.Body)
		}
		return strings.Join(parts, "\n\n")
	case []Change:
		parts := make([]string, len(t))
		for i, c := range t {
			parts[i] = fmt.Sprintf("%s %s %s", date(c.At), c.Actor, c.Action)
		}
		return strings.Join(parts, "\n")
	case bool, int, int64, float64, json.Number:
		return fmt.Sprint(t)
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(raw)
	}
}
