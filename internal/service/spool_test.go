// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// stalledWriter is a client that never reads: its first write blocks until
// the test lets it go, and entered says the export reached it.
type stalledWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newStalledWriter() *stalledWriter {
	return &stalledWriter{entered: make(chan struct{}), release: make(chan struct{})}
}

func (w *stalledWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}

// A bundle export wrote its header before opening the transaction, so it
// cannot fill the pool the same way; the two that stream inside it are the
// ones exercised.
//
// More exports than the store has reader connections, each stalled on a
// client that does not read, must not stop another tenant reading. Streaming
// inside the read transaction let them hold every reader connection.
func TestStalledExportsLeaveTheReaderPoolFree(t *testing.T) {
	l, _, _, _ := newLocal(t)
	stalled := max(4, runtime.NumCPU()) + 1
	exports := []func(context.Context, *stalledWriter) error{
		func(ctx context.Context, w *stalledWriter) error { return l.ExportTo(ctx, core.ExportInput{}, w) },
		func(ctx context.Context, w *stalledWriter) error {
			return l.ExportTasks(ctx, core.TaskExportInput{ProjectRef: "p"}, w)
		},
	}
	var writers []*stalledWriter
	var wg sync.WaitGroup
	for i := range stalled {
		ctx, _, _ := newTenant(t, l, fmt.Sprintf("t%d", i))
		seed(t, l, ctx, "p")
		w := newStalledWriter()
		writers = append(writers, w)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = exports[i%len(exports)](ctx, w)
		}()
	}
	defer func() {
		for _, w := range writers {
			close(w.release)
		}
		wg.Wait()
	}()
	for _, w := range writers {
		select {
		case <-w.entered:
		case <-time.After(10 * time.Second):
			t.Fatal("an export never reached its client: the stalled ones are holding every reader connection")
		}
	}

	other, _, _ := newTenant(t, l, "other")
	seed(t, l, other, "o")
	ctx, cancel := context.WithTimeout(other, 5*time.Second)
	defer cancel()
	if _, err := l.ListTasks(ctx, core.TaskFilter{}); err != nil {
		t.Fatalf("another tenant could not read while exports stalled: %v", err)
	}
}

func TestATenantRunsAtMostTwoExportsAtOnce(t *testing.T) {
	l, _, _, _ := newLocal(t)
	ctx, _, _ := newTenant(t, l, "busy")
	seed(t, l, ctx, "p")
	var writers []*stalledWriter
	var wg sync.WaitGroup
	for range MaxConcurrentExports {
		w := newStalledWriter()
		writers = append(writers, w)
		wg.Add(1)
		go func() { defer wg.Done(); _ = l.ExportTo(ctx, core.ExportInput{}, w) }()
		<-w.entered
	}
	err := l.ExportTo(ctx, core.ExportInput{}, io.Discard)
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a third concurrent export: %v, want a conflict", err)
	}
	other, _, _ := newTenant(t, l, "calm")
	seed(t, l, other, "q")
	if err := l.ExportTo(other, core.ExportInput{}, io.Discard); err != nil {
		t.Fatalf("another tenant's export was refused: %v", err)
	}
	for _, w := range writers {
		close(w.release)
	}
	wg.Wait()
	if err := l.ExportTo(ctx, core.ExportInput{}, io.Discard); err != nil {
		t.Fatalf("the slots were not given back: %v", err)
	}
}

func TestASpoolPastMemorySpillsAndReadsBackWhole(t *testing.T) {
	sp := &spool{}
	defer sp.Close()
	chunk := strings.Repeat("x", 1<<20)
	var want bytes.Buffer
	for i := 0; want.Len() <= maxSnapshotMemory+(1<<20); i++ {
		line := fmt.Sprintf("%d:%s\n", i, chunk)
		want.WriteString(line)
		if _, err := sp.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if sp.file == nil {
		t.Fatal("the spool never spilled to a file")
	}
	var got bytes.Buffer
	if err := sp.copyTo(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatalf("read back %d bytes, wrote %d", got.Len(), want.Len())
	}
}
