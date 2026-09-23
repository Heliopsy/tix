// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// drain waits for the archive housekeeping a rotation started, so a test can
// assert on the files without racing the goroutine that prunes them.
func (w *File) drain() { w.wg.Wait() }

// tiny is a rotation size small enough that a handful of records crosses it.
// It is a whole megabyte because the setting is megabytes; the tests that need
// many rotations write a megabyte at a time rather than pretending otherwise.
const tiny = 1

func mib(n int) []byte {
	line := make([]byte, n*bytesPerMB)
	for i := range line {
		line[i] = 'x'
	}
	line[len(line)-1] = '\n'
	return line
}

func TestNewFileRejectsUnusableLimits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name string
		path string
		opts FileOptions
		want string
	}{
		{"no path", "", FileOptions{MaxSizeMB: 1}, "needs a path"},
		{"zero size", filepath.Join(dir, "a.log"), FileOptions{}, "max size must be positive"},
		{"negative size", filepath.Join(dir, "b.log"), FileOptions{MaxSizeMB: -1}, "max size must be positive"},
		{"negative backups", filepath.Join(dir, "c.log"), FileOptions{MaxSizeMB: 1, MaxBackups: -1}, "max backups must not be negative"},
		{"negative age", filepath.Join(dir, "d.log"), FileOptions{MaxSizeMB: 1, MaxAge: core.Duration(-time.Hour)}, "max age must not be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, err := NewFile(tc.path, tc.opts, clock.NewFakeAt())
			if err == nil {
				_ = f.Close()
				t.Fatalf("NewFile(%q, %+v) succeeded, want an error about %q", tc.path, tc.opts, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("NewFile error = %q, want it to mention %q", err, tc.want)
			}
			if core.KindOf(err) != core.KindInvalid {
				t.Fatalf("NewFile error kind = %v, want invalid", core.KindOf(err))
			}
		})
	}
}

func TestWriteRotatesPastTheSize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tix.log")
	clk := clock.NewFakeAt()
	f, err := NewFile(path, FileOptions{MaxSizeMB: tiny, MaxBackups: 5}, clk)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	defer func() { _ = f.Close() }()

	for i := range 3 {
		if _, err := f.Write(mib(1)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		clk.Advance(time.Second)
	}
	f.drain()

	got := archivesIn(t, dir)
	if len(got) != 2 {
		t.Fatalf("after 3 writes of the whole budget there are %d archives (%v), want 2", len(got), got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() != int64(bytesPerMB) {
		t.Fatalf("the live file holds %d bytes, want the one record of %d", info.Size(), bytesPerMB)
	}
}

func TestPruningHonoursTheLimits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		maxBackups int
		maxAge     time.Duration
		advance    time.Duration
		writes     int
		want       int
	}{
		{"count bounds the archives", 2, 0, time.Second, 6, 2},
		{"zero count keeps every archive", 0, 0, time.Second, 4, 3},
		{"age evicts the old ones", 0, 5 * time.Second, 2 * time.Second, 6, 3},
		{"count and age both apply", 2, 5 * time.Second, 2 * time.Second, 6, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "tix.log")
			clk := clock.NewFakeAt()
			f, err := NewFile(path,
				FileOptions{MaxSizeMB: tiny, MaxBackups: tc.maxBackups, MaxAge: core.Duration(tc.maxAge)}, clk)
			if err != nil {
				t.Fatalf("NewFile: %v", err)
			}
			defer func() { _ = f.Close() }()

			for range tc.writes {
				if _, err := f.Write(mib(1)); err != nil {
					t.Fatalf("write: %v", err)
				}
				f.drain()
				clk.Advance(tc.advance)
			}
			f.drain()

			got := archivesIn(t, dir)
			if len(got) != tc.want {
				t.Fatalf("kept %d archives (%v), want %d with max_backups=%d max_age=%s",
					len(got), got, tc.want, tc.maxBackups, tc.maxAge)
			}
		})
	}
}

func TestCompressReplacesTheArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tix.log")
	clk := clock.NewFakeAt()
	f, err := NewFile(path, FileOptions{MaxSizeMB: tiny, MaxBackups: 3, Compress: true}, clk)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	defer func() { _ = f.Close() }()

	first := mib(1)
	if _, err := f.Write(first); err != nil {
		t.Fatalf("write: %v", err)
	}
	clk.Advance(time.Second)
	if _, err := f.Write(mib(1)); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.drain()

	got := archivesIn(t, dir)
	if len(got) != 1 {
		t.Fatalf("there are %d archives (%v), want 1", len(got), got)
	}
	if !strings.HasSuffix(got[0], gzipExt) {
		t.Fatalf("archive %q is not compressed, want a %s suffix", got[0], gzipExt)
	}
	if body := gunzip(t, filepath.Join(dir, got[0])); len(body) != len(first) {
		t.Fatalf("the compressed archive holds %d bytes, want the %d that were written", len(body), len(first))
	}
}

// TestConcurrentWritersLoseNoLinesAcrossRotation is the test that pays for
// hand-rolling the rotation instead of taking a dependency: many writers, many
// rotations, and afterwards every line written is present exactly once, whole,
// in exactly one of the files.
func TestConcurrentWritersLoseNoLinesAcrossRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tix.log")
	clk := clock.NewFakeAt()
	f, err := NewFile(path, FileOptions{MaxSizeMB: tiny}, clk)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}

	const writers, perWriter = 8, 1200
	// A record big enough that the run crosses the megabyte budget many times
	// rather than once.
	const padding = 512

	want := make(map[string]bool, writers*perWriter)
	for w := range writers {
		for i := range perWriter {
			want[line(w, i, padding)] = false
		}
	}

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWriter {
				if _, err := f.Write([]byte(line(w, i, padding) + "\n")); err != nil {
					t.Errorf("writer %d record %d: %v", w, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files := append([]string{"tix.log"}, archivesIn(t, dir)...)
	if len(files) < 3 {
		t.Fatalf("the run produced %d files (%v); it did not rotate enough to prove anything", len(files), files)
	}
	seen := 0
	for _, name := range files {
		for _, got := range lines(t, filepath.Join(dir, name)) {
			written, known := want[got]
			if !known {
				t.Fatalf("file %s holds %q, which no writer wrote; a record was torn across a rotation", name, elide(got))
			}
			if written {
				t.Fatalf("file %s repeats %q; a record was written twice", name, elide(got))
			}
			want[got] = true
			seen++
		}
	}
	if seen != writers*perWriter {
		var missing []string
		for record, written := range want {
			if !written {
				missing = append(missing, elide(record))
			}
		}
		sort.Strings(missing)
		t.Fatalf("%d of %d records survived rotation; %d were lost, first few: %v",
			seen, writers*perWriter, len(missing), missing[:min(3, len(missing))])
	}
}

func TestWriteAfterCloseIsRefusedRatherThanSilent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tix.log")
	f, err := NewFile(path, FileOptions{MaxSizeMB: tiny}, clock.NewFakeAt())
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := f.Write([]byte("after\n")); err == nil {
		t.Fatal("writing to a closed log file succeeded, want an error rather than a silently dropped record")
	}
}

func TestArchiveNamesDoNotCollideInsideOneTick(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tix.log")
	// The clock never advances, so every rotation asks for the same stamp.
	f, err := NewFile(path, FileOptions{MaxSizeMB: tiny}, clock.NewFakeAt())
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	defer func() { _ = f.Close() }()
	for range 4 {
		if _, err := f.Write(mib(1)); err != nil {
			t.Fatalf("write: %v", err)
		}
		f.drain()
	}
	f.drain()
	if got := archivesIn(t, dir); len(got) != 3 {
		t.Fatalf("three rotations inside one clock tick left %d archives (%v), want 3", len(got), got)
	}
}

func line(writer, record, padding int) string {
	return fmt.Sprintf("w=%02d r=%04d %s", writer, record, strings.Repeat("p", padding))
}

func elide(s string) string {
	if len(s) <= 24 {
		return s
	}
	return s[:24] + "..."
}

// archivesIn lists the rotated files in dir, sorted, excluding the live one.
func archivesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.Name() == "tix.log" {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 -- a path this test created
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	var out []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return out
}

func gunzip(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 -- a path this test created
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("reading %s as gzip: %v", path, err)
	}
	defer func() { _ = zr.Close() }()
	body := make([]byte, 0, bytesPerMB)
	buf := make([]byte, 32*1024)
	for {
		n, err := zr.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	return body
}

func TestNewFileRefusesAPathItCannotOpen(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", blocker, err)
	}
	if _, err := NewFile(filepath.Join(blocker, "tix.log"), FileOptions{MaxSizeMB: 1}, clock.NewFakeAt()); err == nil {
		t.Fatal("a destination under a regular file was opened, want a refusal")
	}
}

func TestReopeningAppendsRatherThanTruncates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tix.log")
	first, err := NewFile(path, FileOptions{MaxSizeMB: tiny}, clock.NewFakeAt())
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if _, err := first.Write([]byte("before-the-restart\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := NewFile(path, FileOptions{MaxSizeMB: tiny}, clock.NewFakeAt())
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if _, err := second.Write([]byte("after-the-restart\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := lines(t, path)
	want := []string{"before-the-restart", "after-the-restart"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("after a restart the file holds %v, want %v; a restart must not lose the log", got, want)
	}
}

// TestPruningLeavesForeignFilesAlone keeps the writer to the files it named.
// A rotator that globs a directory and deletes what it finds is one symlink or
// one shared directory away from removing somebody else's data.
func TestPruningLeavesForeignFilesAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tix.log")
	strangers := []string{
		filepath.Join(dir, "tix-not-a-timestamp.log"),
		filepath.Join(dir, "tix-20260101T000000.000.txt"),
		filepath.Join(dir, "other.log"),
	}
	for _, name := range strangers {
		if err := os.WriteFile(name, []byte("not ours\n"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	clk := clock.NewFakeAt()
	f, err := NewFile(path, FileOptions{MaxSizeMB: tiny, MaxBackups: 1}, clk)
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	defer func() { _ = f.Close() }()
	for range 4 {
		if _, err := f.Write(mib(1)); err != nil {
			t.Fatalf("write: %v", err)
		}
		f.drain()
		clk.Advance(time.Second)
	}
	f.drain()

	for _, name := range strangers {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("pruning removed %s, which this writer did not create: %v", name, err)
		}
	}
}
