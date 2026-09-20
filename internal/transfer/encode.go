package transfer

import (
	"encoding/json"
	"io"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// Encoder writes a snapshot as one JSON record per line.
type Encoder struct {
	enc *json.Encoder
	err error
}

// NewEncoder returns an encoder writing records to w as they are produced.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{enc: json.NewEncoder(w)}
}

// Header writes the opening header record.
func (e *Encoder) Header(tenantKey string, at time.Time) error {
	return e.Encode(core.SnapshotRecord{
		Kind: core.RecordHeader,
		Header: &core.SnapshotHeader{
			Version:    core.SnapshotVersion,
			TenantKey:  tenantKey,
			ExportedAt: at.UTC(),
		},
	})
}

// Encode writes one record.
func (e *Encoder) Encode(rec core.SnapshotRecord) error {
	if e.err != nil {
		return e.err
	}
	if _, ok := Order(rec.Kind); !ok {
		e.err = core.Invalid("snapshot record kind %q is not known", rec.Kind)
		return e.err
	}
	if !HasPayload(rec) {
		e.err = core.Invalid("snapshot record of kind %q carries no payload", rec.Kind)
		return e.err
	}
	if err := e.enc.Encode(rec); err != nil {
		e.err = core.Internal("writing snapshot record: %v", err).Wrap(err)
	}
	return e.err
}

// Err returns the failure that stopped the encoder.
func (e *Encoder) Err() error { return e.err }
