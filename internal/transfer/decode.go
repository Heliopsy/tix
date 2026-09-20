package transfer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/thereisnotime/tix/internal/core"
)

// Decoder reads a snapshot one record at a time, never holding more than the
// line it is parsing.
type Decoder struct {
	r      *bufio.Reader
	line   int
	last   int
	header bool
}

// NewDecoder returns a decoder reading records from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r), last: -1}
}

// Line reports the line number of the record most recently read.
func (d *Decoder) Line() int { return d.line }

// Next returns the next record, or io.EOF once the stream is exhausted.
func (d *Decoder) Next() (*core.SnapshotRecord, error) {
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

// Header reads the opening record and refuses a version this build cannot read.
func (d *Decoder) Header() (*core.SnapshotHeader, error) {
	rec, err := d.Next()
	if errors.Is(err, io.EOF) {
		return nil, core.Invalid("snapshot is empty; it must open with a header record")
	}
	if err != nil {
		return nil, err
	}
	if rec.Kind != core.RecordHeader {
		return nil, core.Invalid("snapshot opens with a %q record at line %d, not a header", rec.Kind, d.line)
	}
	switch rec.Header.Version {
	case 0:
		return nil, core.Invalid("snapshot declares no format version; version %d is required", core.SnapshotVersion)
	case core.SnapshotVersion:
		return rec.Header, nil
	default:
		return nil, core.Invalid("snapshot format version %d is not supported; this build reads version %d",
			rec.Header.Version, core.SnapshotVersion)
	}
}

// parse turns one line into a record, refusing anything the format forbids.
func (d *Decoder) parse(raw []byte) (*core.SnapshotRecord, error) {
	var rec core.SnapshotRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, core.Invalid("snapshot line %d is not a valid record: %v", d.line, err)
	}
	order, ok := Order(rec.Kind)
	if !ok {
		return nil, core.Invalid("snapshot line %d has unknown record kind %q", d.line, rec.Kind)
	}
	if !HasPayload(rec) {
		return nil, core.Invalid("snapshot line %d carries no %s payload", d.line, rec.Kind)
	}
	if rec.Kind == core.RecordHeader && d.header {
		return nil, core.Invalid("snapshot line %d repeats the header record", d.line)
	}
	if order < d.last {
		return nil, core.Invalid("snapshot line %d has a %q record after a %q record, which breaks the snapshot order",
			d.line, rec.Kind, Kinds[d.last])
	}
	d.header = d.header || rec.Kind == core.RecordHeader
	d.last = order
	return &rec, nil
}

// readLine returns the next line, treating a final line without a newline as a
// line of its own so a truncated stream fails to parse rather than vanishing.
func (d *Decoder) readLine() ([]byte, error) {
	raw, err := d.r.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, core.Internal("reading snapshot: %v", err).Wrap(err)
	}
	if len(raw) == 0 {
		return nil, io.EOF
	}
	d.line++
	return raw, nil
}
