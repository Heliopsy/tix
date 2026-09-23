// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// gopkg.in/natefinch/lumberjack is the usual answer here and is not used, for
// the same reason there is no router and no ORM: the whole of what it does for
// us is a rename under the lock a single-process writer already needs. It also
// brings behaviour we would have to work around rather than inherit, namely a
// MaxAge measured in whole days and a rotated-file clock that cannot be faked,
// which the retention windows beside it express as durations and test with
// clock.Fake. The one thing genuinely fiddly about rotation, a writer landing
// bytes in a file that is being renamed out from under it, cannot happen here:
// the rename runs inside the same mutex as every Write, so no record is ever
// split across two files and none is written to a descriptor already archived.
// This is not a claim, it is what TestConcurrentWritersLoseNoLinesAcrossRotation
// asserts. The cost of the choice is that it is ours to fix.

// fileMode and dirMode keep a log readable by its owner and nobody else: a
// record carries tenant keys and request paths.
const (
	fileMode os.FileMode = 0o600
	dirMode  os.FileMode = 0o700
)

// stampLayout dates a rotated file. It sorts lexically in the order it sorts
// chronologically, which is what lets pruning order by name.
const stampLayout = "20060102T150405.000"

// bytesPerMB converts the megabytes an operator configures into the bytes the
// writer counts.
const bytesPerMB = 1024 * 1024

// File is an io.WriteCloser that rotates its destination once it grows past a
// size, keeping a bounded number of dated archives. It is safe for concurrent
// writers.
type File struct {
	path       string
	maxBytes   int64
	maxAge     time.Duration
	maxBackups int
	compress   bool
	clk        clock.Clock

	mu   sync.Mutex
	f    *os.File
	size int64

	// maint serializes the archive housekeeping, which runs off the write
	// path so compressing a large archive never blocks a logging server.
	maint sync.Mutex
	wg    sync.WaitGroup
}

// NewFile opens path for appending, creating its directory if needed.
func NewFile(path string, o FileOptions, clk clock.Clock) (*File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, core.Invalid("a log file destination needs a path")
	}
	if o.MaxSizeMB <= 0 {
		return nil, core.Invalid("log file max size must be positive, got %d", o.MaxSizeMB)
	}
	if o.MaxBackups < 0 {
		return nil, core.Invalid("log file max backups must not be negative, got %d", o.MaxBackups)
	}
	if o.MaxAge < 0 {
		return nil, core.Invalid("log file max age must not be negative, got %s", o.MaxAge)
	}
	if clk == nil {
		clk = clock.New()
	}
	f := &File{
		path:       path,
		maxBytes:   int64(o.MaxSizeMB) * bytesPerMB,
		maxAge:     time.Duration(o.MaxAge),
		maxBackups: o.MaxBackups,
		compress:   o.Compress,
		clk:        clk,
	}
	if err := f.open(); err != nil {
		return nil, err
	}
	return f, nil
}

// Write appends p, rotating first when p would carry the file past its size.
func (w *File) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return 0, core.Internal("writing to a closed log file %q", w.path)
	}
	// A record larger than the whole budget still goes to one file rather
	// than being split, so a reader never has to stitch one back together.
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// Close flushes the file and waits for any archive housekeeping to finish.
func (w *File) Close() error {
	w.mu.Lock()
	f := w.f
	w.f = nil
	w.mu.Unlock()
	w.wg.Wait()
	if f == nil {
		return nil
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing log file %q: %w", w.path, err)
	}
	return nil
}

func (w *File) open() error {
	if err := os.MkdirAll(filepath.Dir(w.path), dirMode); err != nil {
		return fmt.Errorf("creating log directory for %q: %w", w.path, err)
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, fileMode) // #nosec G304 -- the path is the operator's log destination
	if err != nil {
		return fmt.Errorf("opening log file %q: %w", w.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("sizing log file %q: %w", w.path, err)
	}
	w.f = f
	w.size = info.Size()
	return nil
}

// rotate archives the open file and opens a fresh one. The caller holds mu.
func (w *File) rotate() error {
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("closing log file %q: %w", w.path, err)
	}
	w.f = nil
	archive := w.archiveName()
	if err := os.Rename(w.path, archive); err != nil {
		return fmt.Errorf("rotating log file %q: %w", w.path, err)
	}
	if err := w.open(); err != nil {
		return err
	}
	w.wg.Add(1)
	go w.housekeep(archive)
	return nil
}

// archiveName dates the archive, and disambiguates a stamp already taken so
// two rotations inside one clock tick cannot overwrite each other.
func (w *File) archiveName() string {
	ext := filepath.Ext(w.path)
	stem := strings.TrimSuffix(w.path, ext)
	stamp := w.clk.Now().UTC().Format(stampLayout)
	for n := 0; ; n++ {
		candidate := stem + "-" + stamp + ext
		if n > 0 {
			candidate = fmt.Sprintf("%s-%s-%02d%s", stem, stamp, n, ext)
		}
		if !taken(candidate) {
			return candidate
		}
	}
}

func taken(path string) bool {
	if _, err := os.Lstat(path); err == nil {
		return true
	}
	_, err := os.Lstat(path + gzipExt)
	return err == nil
}

const gzipExt = ".gz"

// housekeep compresses the archive and prunes the ones past the limits. It is
// serialized so a burst of rotations cannot have two runs disagree about which
// archives exist.
func (w *File) housekeep(archive string) {
	defer w.wg.Done()
	w.maint.Lock()
	defer w.maint.Unlock()
	if w.compress {
		_ = compress(archive)
	}
	_ = w.prune()
}

// prune removes the archives past MaxBackups or past MaxAge. Zero means no
// limit of that kind, so a deployment shipping logs elsewhere can keep them.
func (w *File) prune() error {
	archives, err := w.archives()
	if err != nil {
		return err
	}
	cutoff := w.clk.Now().UTC().Add(-w.maxAge)
	for i, a := range archives {
		stale := w.maxBackups > 0 && i >= w.maxBackups
		expired := w.maxAge > 0 && a.stamp.Before(cutoff)
		if stale || expired {
			_ = os.Remove(a.path)
		}
	}
	return nil
}

type archive struct {
	path  string
	stamp time.Time
}

// archives lists this file's archives, newest first.
func (w *File) archives() ([]archive, error) {
	ext := filepath.Ext(w.path)
	stem := strings.TrimSuffix(w.path, ext)
	matches, err := filepath.Glob(stem + "-*" + ext + "*")
	if err != nil {
		return nil, fmt.Errorf("listing log archives for %q: %w", w.path, err)
	}
	out := make([]archive, 0, len(matches))
	for _, path := range matches {
		stamp, ok := stampOf(path, stem, ext)
		if !ok {
			continue
		}
		out = append(out, archive{path: path, stamp: stamp})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].stamp.Equal(out[j].stamp) {
			return out[i].path > out[j].path
		}
		return out[i].stamp.After(out[j].stamp)
	})
	return out, nil
}

// stampOf reads the rotation time back out of an archive name, which is what
// keeps a file this writer did not create out of the pruning set.
func stampOf(path, stem, ext string) (time.Time, bool) {
	rest, ok := strings.CutPrefix(path, stem+"-")
	if !ok {
		return time.Time{}, false
	}
	rest = strings.TrimSuffix(rest, gzipExt)
	rest, ok = strings.CutSuffix(rest, ext)
	if !ok {
		return time.Time{}, false
	}
	if len(rest) > len(stampLayout) {
		rest = rest[:len(stampLayout)]
	}
	stamp, err := time.Parse(stampLayout, rest)
	if err != nil {
		return time.Time{}, false
	}
	return stamp.UTC(), true
}

// compress replaces an archive with its gzip, leaving the original in place if
// anything fails: a log half written is worse than a log not compressed.
func compress(path string) error {
	src, err := os.Open(path) // #nosec G304 -- an archive this writer named
	if err != nil {
		return fmt.Errorf("opening log archive %q: %w", path, err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(path+gzipExt, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode) // #nosec G304 -- an archive this writer named
	if err != nil {
		return fmt.Errorf("creating log archive %q: %w", path+gzipExt, err)
	}
	zw := gzip.NewWriter(dst)
	if _, err := io.Copy(zw, src); err != nil {
		_ = zw.Close()
		_ = dst.Close()
		_ = os.Remove(path + gzipExt)
		return fmt.Errorf("compressing log archive %q: %w", path, err)
	}
	if err := zw.Close(); err != nil {
		_ = dst.Close()
		_ = os.Remove(path + gzipExt)
		return fmt.Errorf("compressing log archive %q: %w", path, err)
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(path + gzipExt)
		return fmt.Errorf("compressing log archive %q: %w", path, err)
	}
	return os.Remove(path)
}
