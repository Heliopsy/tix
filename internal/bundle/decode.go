package bundle

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/heliopsy/tix/internal/core"
)

// Decoder reads a bundle one record at a time, never holding more than the
// line it is parsing.
type Decoder struct {
	r      *bufio.Reader
	line   int
	last   int
	header bool
	seen   map[string]bool
}

// NewDecoder returns a decoder reading records from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r), last: -1, seen: map[string]bool{}}
}

// Line reports the line number of the record most recently read.
func (d *Decoder) Line() int { return d.line }

// Header reads the opening record and refuses a schema version this build
// cannot read.
func (d *Decoder) Header() (*Header, error) {
	rec, err := d.Next()
	if errors.Is(err, io.EOF) {
		return nil, core.Invalid("bundle is empty; it must open with a header record")
	}
	if err != nil {
		return nil, err
	}
	if rec.Record != RecordHeader {
		return nil, core.Invalid("bundle opens with a %q record at line %d, not a header", rec.Record, d.line)
	}
	switch rec.Header.Version {
	case core.BundleVersion:
		return rec.Header, nil
	case 0:
		return nil, core.Invalid("bundle declares no schema version (0); this build reads bundle version %d",
			core.BundleVersion)
	default:
		return nil, core.Invalid("bundle schema version %d is not supported; this build reads bundle version %d",
			rec.Header.Version, core.BundleVersion)
	}
}

// Next returns the next record, or io.EOF once the bundle is exhausted.
func (d *Decoder) Next() (*Record, error) {
	for {
		raw, err := d.readLine()
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		return d.parse(raw)
	}
}

// Read decodes a whole bundle, validating every component before the caller
// writes anything.
func Read(r io.Reader) (*Header, []Component, error) {
	dec := NewDecoder(r)
	header, err := dec.Header()
	if err != nil {
		return nil, nil, err
	}
	var out []Component
	for {
		rec, err := dec.Next()
		if errors.Is(err, io.EOF) {
			return header, out, nil
		}
		if err != nil {
			return nil, nil, err
		}
		out = append(out, *rec.Component)
	}
}

// parse turns one line into a record, refusing anything the format forbids.
func (d *Decoder) parse(raw []byte) (*Record, error) {
	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, core.Invalid("bundle line %d is not a valid record: %v", d.line, err)
	}
	switch rec.Record {
	case RecordHeader:
		if d.header {
			return nil, core.Invalid("bundle line %d repeats the header record", d.line)
		}
		if rec.Header == nil {
			return nil, core.Invalid("bundle line %d is a header record with no header", d.line)
		}
		d.header = true
		return &rec, nil
	case RecordComponent:
		return d.parseComponent(rec)
	case "":
		return nil, core.Invalid("bundle line %d names no record type", d.line)
	default:
		return nil, core.Invalid("bundle line %d has unknown record type %q", d.line, rec.Record)
	}
}

// parseComponent checks one component record's payload, order and uniqueness.
func (d *Decoder) parseComponent(rec Record) (*Record, error) {
	if !d.header {
		return nil, core.Invalid("bundle line %d carries a component before the header record", d.line)
	}
	if rec.Component == nil {
		return nil, core.Invalid("bundle line %d is a component record with no component", d.line)
	}
	c := *rec.Component
	if err := c.Validate(); err != nil {
		return nil, core.Invalid("bundle line %d: %s", d.line, message(err))
	}
	order, _ := Order(c.Kind)
	if order < d.last {
		return nil, core.Invalid("bundle line %d has a %q component after a %q component, which breaks the bundle order",
			d.line, c.Kind, core.ComponentKinds[d.last])
	}
	id := string(c.Kind) + "\x00" + c.Key()
	if d.seen[id] {
		return nil, core.Invalid("bundle line %d repeats %s %q", d.line, c.Kind, c.Key())
	}
	d.seen[id] = true
	d.last = order
	return &rec, nil
}

// readLine returns the next line, treating a final line without a newline as a
// line of its own so a truncated bundle fails to parse rather than vanishing.
func (d *Decoder) readLine() ([]byte, error) {
	raw, err := d.r.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, core.Internal("reading bundle: %v", err).Wrap(err)
	}
	if len(raw) == 0 {
		return nil, io.EOF
	}
	d.line++
	return raw, nil
}
