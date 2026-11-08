// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

func openMode(t *testing.T, mode string) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tix.db")
	s, err := Open(path, clock.NewFakeAt(), WithJournalMode(mode))
	if err != nil {
		t.Fatalf("opening in %q mode: %v", mode, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return s, path
}

func pragma(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var v string
	if err := db.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&v); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return v
}

func TestJournalModeIsAppliedToBothPools(t *testing.T) {
	tests := []struct {
		mode        string
		journal     string
		synchronous string
	}{
		{mode: "", journal: "wal", synchronous: "1"},
		{mode: "wal", journal: "wal", synchronous: "1"},
		{mode: "WAL", journal: "wal", synchronous: "1"},
		{mode: "delete", journal: "delete", synchronous: "2"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			s, _ := openMode(t, tt.mode)
			if got := string(s.JournalMode()); got != tt.journal {
				t.Errorf("JournalMode() = %q, want %q", got, tt.journal)
			}
			for name, db := range map[string]*sql.DB{"writer": s.writer, "reader": s.reader} {
				if got := pragma(t, db, "journal_mode"); got != tt.journal {
					t.Errorf("%s journal_mode = %q, want %q", name, got, tt.journal)
				}
				if got := pragma(t, db, "synchronous"); got != tt.synchronous {
					t.Errorf("%s synchronous = %q, want %q", name, got, tt.synchronous)
				}
				if got := pragma(t, db, "foreign_keys"); got != "1" {
					t.Errorf("%s foreign_keys = %q, want 1", name, got)
				}
			}
		})
	}
}

func TestUnknownJournalModeIsRefusedBeforeOpening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tix.db")
	for _, mode := range []string{"memory", "off", "truncate", "persist"} {
		_, err := Open(path, clock.NewFakeAt(), WithJournalMode(mode))
		if core.KindOf(err) != core.KindInvalid {
			t.Errorf("Open(%q) error = %v, want invalid", mode, err)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a refused mode created %q: %v", path, err)
	}
}

func walSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path + "-wal")
	if errors.Is(err, fs.ErrNotExist) {
		return -1
	}
	if err != nil {
		t.Fatalf("reading the wal: %v", err)
	}
	return info.Size()
}

func TestCheckpointEmptiesTheWriteAheadLog(t *testing.T) {
	s, path := openMode(t, "wal")
	if walSize(t, path) <= 0 {
		t.Fatalf("migrating left no write-ahead log to checkpoint")
	}
	if err := s.Checkpoint(context.Background()); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if got := walSize(t, path); got > 0 {
		t.Errorf("wal holds %d bytes after the checkpoint, want 0", got)
	}
}

func TestDeleteModeKeepsNoWriteAheadLog(t *testing.T) {
	s, path := openMode(t, "delete")
	if err := s.Checkpoint(context.Background()); err != nil {
		t.Fatalf("Checkpoint in delete mode: %v", err)
	}
	if got := walSize(t, path); got != -1 {
		t.Errorf("delete mode left a %d byte -wal file", got)
	}
}
