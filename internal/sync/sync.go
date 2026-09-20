// Package sync imports records from external systems into tix.
package sync

import (
	"context"
	"time"
)

// Record is one external entity in adapter-neutral form. Fields holds the
// source payload as the adapter read it, addressed later by dotted path.
type Record struct {
	Fields    map[string]any `json:"fields" yaml:"fields"`
	UpdatedAt time.Time      `json:"updated_at" yaml:"updated_at"`
}

// Options selects what one Fetch call returns.
type Options struct {
	// Cursor is the watermark of the last successful run, ignored when Full.
	Cursor string
	// Page continues a run within the same set of results.
	Page string
	// Full reconsiders every source record regardless of the cursor.
	Full bool
	// Size is the requested page size; zero means the adapter's default.
	Size int
}

// Batch is one page of external records.
type Batch struct {
	Records []Record `json:"records" yaml:"records"`
	// Page continues the run, or is empty when the source is exhausted.
	Page string `json:"page,omitempty" yaml:"page,omitempty"`
	// Cursor is the watermark to store once this page has been committed.
	Cursor string `json:"cursor,omitempty" yaml:"cursor,omitempty"`
}

// Importer pulls records from an external system one page at a time.
//
// Fetch is the whole of the v1 contract; a two-way adapter adds a Push method
// to its own implementation without changing anything a caller depends on.
type Importer interface {
	System() string
	Fetch(ctx context.Context, opt Options) (Batch, error)
}

// DefaultPageSize is the page size an adapter uses when none is configured.
const DefaultPageSize = 100

// PageSize clamps a requested page size into something a source will accept.
func PageSize(requested int) int {
	switch {
	case requested <= 0:
		return DefaultPageSize
	case requested > 1000:
		return 1000
	default:
		return requested
	}
}
