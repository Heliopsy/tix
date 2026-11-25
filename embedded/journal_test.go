// SPDX-License-Identifier: AGPL-3.0-or-later

package embedded_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/heliopsy/tix/embedded"
	"github.com/heliopsy/tix/internal/core"

	_ "modernc.org/sqlite"
)

// copyMainFile copies the database file alone, without its -wal or journal,
// the way a sync tool that caught it between two files would.
func copyMainFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "copy.db")
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

func tenantKeysOf(t *testing.T, path string) []string {
	t.Helper()
	in := start(t, path, false)
	tenants, err := in.Tenants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return keys(tenants)
}

func TestDeleteModeKeepsEveryCommitInTheMainFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tix.db")
	ctx := context.Background()
	in, err := embedded.Start(ctx, embedded.Options{DBPath: path, InitIfMissing: true, JournalMode: "delete"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Stop(ctx) })
	if _, err := in.CreateTenant(ctx, "kept", "Kept"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + "-wal"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("delete mode keeps a -wal file beside the database: %v", err)
	}
	copied := copyMainFile(t, path)
	if got := tenantKeysOf(t, copied); !slices.Equal(got, []string{"default", "kept"}) {
		t.Fatalf("a copy of the running database's main file holds %v", got)
	}
}

// holdOpen keeps an idle connection to the database, as another reader of the
// file would, so closing the instance is not the last close and SQLite's own
// checkpoint on last close does not run.
func holdOpen(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM sqlite_master").Scan(&n); err != nil {
		t.Fatal(err)
	}
}

func TestStopFoldsTheWriteAheadLogIntoTheMainFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tix.db")
	ctx := context.Background()
	in, err := embedded.Start(ctx, embedded.Options{DBPath: path, InitIfMissing: true})
	if err != nil {
		t.Fatal(err)
	}
	holdOpen(t, path)
	if _, err := in.CreateTenant(ctx, "kept", "Kept"); err != nil {
		t.Fatal(err)
	}
	if err := in.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() > 0 {
		t.Fatalf("the -wal file holds %d bytes after stop", info.Size())
	}
	copied := copyMainFile(t, path)
	if got := tenantKeysOf(t, copied); !slices.Equal(got, []string{"default", "kept"}) {
		t.Fatalf("the main file alone holds %v after stop", got)
	}
}

func TestAnUnknownJournalModeIsRefusedBeforeOpening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tix.db")
	_, err := embedded.Start(context.Background(), embedded.Options{DBPath: path, InitIfMissing: true, JournalMode: "memory"})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("journal mode memory: got %v, want invalid", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused journal mode created the database: %v", err)
	}
}
