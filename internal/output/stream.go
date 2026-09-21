package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"gopkg.in/yaml.v3"
)

// FormatNDJSON writes one JSON object per line.
const FormatNDJSON = "ndjson"

// Stream writes records one at a time so a listing or an export never has to
// be held in memory. Close finishes the format; a stream is not valid output
// until it is closed.
type Stream interface {
	Write(record any) error
	Close() error
}

// NewStream returns a Stream writing to w in the named format. An unrecognised
// format falls back to table, matching New.
func NewStream(format string, w io.Writer) Stream { return NewStreamWithMode(format, w, ModeAuto) }

// NewStreamWithMode returns a Stream that colours table output according to mode.
func NewStreamWithMode(format string, w io.Writer, mode Mode) Stream {
	switch format {
	case FormatNDJSON:
		return &ndjsonStream{enc: json.NewEncoder(w)}
	case FormatJSON:
		return &jsonArrayStream{w: w, enc: json.NewEncoder(w)}
	case FormatYAML:
		return &yamlStream{w: w}
	default:
		return &bufferedStream{w: w, format: format, mode: mode}
	}
}

// ndjsonStream is the streaming format: constant memory, line-oriented, and
// readable by jq without buffering the whole response.
type ndjsonStream struct {
	enc *json.Encoder
	err error
}

func (s *ndjsonStream) Write(record any) error {
	if s.err != nil {
		return s.err
	}
	if err := s.enc.Encode(record); err != nil {
		s.err = fmt.Errorf("writing ndjson record: %w", err)
	}
	return s.err
}

func (s *ndjsonStream) Close() error { return s.err }

// jsonArrayStream emits a well-formed JSON array incrementally.
type jsonArrayStream struct {
	w       io.Writer
	enc     *json.Encoder
	started bool
	closed  bool
	err     error
}

func (s *jsonArrayStream) Write(record any) error {
	if s.err != nil {
		return s.err
	}
	sep := ",\n  "
	if !s.started {
		sep = "[\n  "
		s.started = true
	}
	if _, err := io.WriteString(s.w, sep); err != nil {
		s.err = err
		return s.err
	}
	b, err := json.Marshal(record)
	if err != nil {
		s.err = fmt.Errorf("writing json record: %w", err)
		return s.err
	}
	_, s.err = s.w.Write(b)
	return s.err
}

func (s *jsonArrayStream) Close() error {
	if s.err != nil || s.closed {
		return s.err
	}
	s.closed = true
	tail := "[]\n"
	if s.started {
		tail = "\n]\n"
	}
	_, s.err = io.WriteString(s.w, tail)
	return s.err
}

// yamlStream writes each record as its own YAML document.
type yamlStream struct {
	w   io.Writer
	err error
}

func (s *yamlStream) Write(record any) error {
	if s.err != nil {
		return s.err
	}
	b, err := yaml.Marshal(record)
	if err != nil {
		s.err = fmt.Errorf("writing yaml record: %w", err)
		return s.err
	}
	if _, err := io.WriteString(s.w, "---\n"); err != nil {
		s.err = err
		return s.err
	}
	_, s.err = s.w.Write(b)
	return s.err
}

func (s *yamlStream) Close() error { return s.err }

// bufferedStream collects records and renders them once on Close. The table
// format needs every row before it can size its columns, so it cannot stream;
// it is a human format and always paginated, which bounds what it holds.
type bufferedStream struct {
	w       io.Writer
	format  string
	mode    Mode
	records []any
	err     error
}

func (s *bufferedStream) Write(record any) error {
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, record)
	return nil
}

func (s *bufferedStream) Close() error {
	if s.err != nil {
		return s.err
	}
	s.err = NewWithMode(s.format, s.mode).Format(s.w, s.records)
	return s.err
}

// ndjsonFormatter renders a whole value as ndjson, emitting one line per
// element when given a slice so the non-streaming path matches the streaming one.
type ndjsonFormatter struct{}

func (ndjsonFormatter) Format(w io.Writer, data any) error {
	s := NewStream(FormatNDJSON, w)
	for _, rec := range explode(data) {
		if err := s.Write(rec); err != nil {
			return err
		}
	}
	return s.Close()
}

// explode turns a slice into its elements so ndjson emits one line each, and
// leaves any other value as a single record.
func explode(data any) []any {
	if data == nil {
		return nil
	}
	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return []any{data}
	}
	out := make([]any, 0, v.Len())
	for i := range v.Len() {
		out = append(out, v.Index(i).Interface())
	}
	return out
}
