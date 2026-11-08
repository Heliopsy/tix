// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"

	"github.com/heliopsy/tix/internal/core"
)

// MaxConcurrentExports is how many exports one tenant may run at once. An
// export walks the tenant inside a read transaction, and a tenant that started
// dozens at once would hold that many store connections between them.
const MaxConcurrentExports = 2

// exportSlots counts the exports running per tenant. The zero value is ready.
type exportSlots struct {
	mu      sync.Mutex
	running map[string]int
}

// take claims one export slot for tenantID, or refuses when the tenant has
// none left.
func (e *exportSlots) take(tenantID string) (func(), error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running == nil {
		e.running = map[string]int{}
	}
	if e.running[tenantID] >= MaxConcurrentExports {
		return nil, core.Conflict("this tenant already has %d exports running; try again when one ends",
			MaxConcurrentExports)
	}
	e.running[tenantID]++
	var once sync.Once
	return func() {
		once.Do(func() {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.running[tenantID]--; e.running[tenantID] <= 0 {
				delete(e.running, tenantID)
			}
		})
	}, nil
}

// spoolExport runs walk against a spool and copies the spool to w only once
// walk has returned, and with it the read transaction walk opened.
//
// Streaming straight to w held the transaction open for as long as the reader
// took. A client that read slowly or not at all, over SSH or HTTP, held a
// store connection with it, and a handful of them emptied the reader pool for
// every tenant on the server. The spool keeps the first maxSnapshotMemory
// bytes in memory and the rest in a temporary file, so memory stays bounded by
// that constant and the transaction by the speed of the store.
func (l *Local) spoolExport(ctx context.Context, tenantID string, w io.Writer, walk func(io.Writer) error) error {
	release, err := l.exports.take(tenantID)
	if err != nil {
		return err
	}
	defer release()
	sp := &spool{}
	defer sp.Close()
	if err := walk(sp); err != nil {
		return err
	}
	return sp.copyTo(ctx, w)
}

// spool is a write-once buffer that spills to a temporary file past
// maxSnapshotMemory.
type spool struct {
	mem  bytes.Buffer
	file *os.File
}

// Write appends p, spilling to a temporary file once memory is full.
func (s *spool) Write(p []byte) (int, error) {
	if s.file == nil && s.mem.Len()+len(p) <= maxSnapshotMemory {
		return s.mem.Write(p)
	}
	if s.file == nil {
		f, err := os.CreateTemp("", "tix-export-*")
		if err != nil {
			return 0, core.Internal("spooling an export: %v", err)
		}
		s.file = f
	}
	return s.file.Write(p)
}

// copyTo writes what was spooled to w: the memory first, then the file.
func (s *spool) copyTo(ctx context.Context, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := w.Write(s.mem.Bytes()); err != nil {
		return err
	}
	if s.file == nil {
		return nil
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return core.Internal("reading a spooled export: %v", err)
	}
	_, err := io.Copy(w, s.file)
	return err
}

// Close removes the temporary file, if there is one.
func (s *spool) Close() {
	if s.file == nil {
		return
	}
	name := s.file.Name()
	_ = s.file.Close()
	_ = os.Remove(name)
}
