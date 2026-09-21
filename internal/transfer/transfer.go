// Package transfer encodes and decodes streamed tenant snapshots.
package transfer

import (
	"github.com/heliopsy/tix/internal/core"
)

// Kinds are the snapshot record kinds in the order a snapshot writes them, so
// a referenced row always arrives before the row that references it.
var Kinds = []core.RecordKind{
	core.RecordHeader,
	core.RecordWorkflow,
	core.RecordProject,
	core.RecordFieldDef,
	core.RecordLabel,
	core.RecordTask,
	core.RecordDependency,
	core.RecordComment,
	core.RecordArtifact,
}

// Order returns a record kind's position in the snapshot order.
func Order(k core.RecordKind) (int, bool) {
	for i, known := range Kinds {
		if known == k {
			return i, true
		}
	}
	return 0, false
}

// HasPayload reports whether the record carries the payload its kind names.
func HasPayload(rec core.SnapshotRecord) bool {
	switch rec.Kind {
	case core.RecordHeader:
		return rec.Header != nil
	case core.RecordWorkflow:
		return rec.Workflow != nil
	case core.RecordProject:
		return rec.Project != nil
	case core.RecordFieldDef:
		return rec.FieldDef != nil
	case core.RecordLabel:
		return rec.Tag != nil
	case core.RecordTask:
		return rec.Task != nil
	case core.RecordDependency:
		return rec.Dependency != nil
	case core.RecordComment:
		return rec.Comment != nil
	case core.RecordArtifact:
		return rec.Artifact != nil
	default:
		return false
	}
}
