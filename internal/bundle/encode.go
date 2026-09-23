// SPDX-License-Identifier: AGPL-3.0-or-later

package bundle

import (
	"encoding/json"
	"io"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/version"
)

// Encoder writes a bundle as one JSON record per line.
type Encoder struct {
	enc    *json.Encoder
	err    error
	header bool
	last   int
	seen   map[string]bool
}

// NewEncoder returns an encoder writing records to w as they are produced.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{enc: json.NewEncoder(w), last: -1, seen: map[string]bool{}}
}

// Header writes the opening record, naming the bundle and the versions that
// produced it.
func (e *Encoder) Header(name string, at time.Time) error {
	if e.err != nil {
		return e.err
	}
	if e.header {
		e.err = core.Invalid("a bundle carries exactly one header record")
		return e.err
	}
	e.header = true
	return e.write(Record{
		Record: RecordHeader,
		Header: &Header{
			Name:       name,
			Version:    core.BundleVersion,
			TixVersion: version.Version,
			ExportedAt: at.UTC(),
		},
	})
}

// Component writes one component, refusing anything that would make the bundle
// unreadable or its byte order unstable.
func (e *Encoder) Component(c Component) error {
	if e.err != nil {
		return e.err
	}
	if !e.header {
		e.err = core.Invalid("a bundle must open with its header record")
		return e.err
	}
	if err := c.Validate(); err != nil {
		e.err = err
		return e.err
	}
	order, _ := Order(c.Kind)
	if order < e.last {
		e.err = core.Invalid("component %s %q comes after a %q component, which breaks the bundle order",
			c.Kind, c.Key(), core.ComponentKinds[e.last])
		return e.err
	}
	id := string(c.Kind) + "\x00" + c.Key()
	if e.seen[id] {
		e.err = core.Invalid("bundle carries %s %q twice", c.Kind, c.Key())
		return e.err
	}
	e.seen[id] = true
	e.last = order
	return e.write(Record{Record: RecordComponent, Component: &c})
}

// Err returns the failure that stopped the encoder.
func (e *Encoder) Err() error { return e.err }

// write emits one record.
func (e *Encoder) write(rec Record) error {
	if err := e.enc.Encode(rec); err != nil {
		e.err = core.Internal("writing bundle record: %v", err).Wrap(err)
	}
	return e.err
}
