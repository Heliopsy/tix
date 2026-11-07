// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sqlitetest hands tests a migrated SQLite database for the price of a
// file copy.
//
// Applying every migration to a fresh file takes seconds under the race
// detector, because the pure Go driver is instrumented like any other code,
// and a package that did it once per test spent most of its time there. The
// schema is built once per test binary from the embedded migrations, so it is
// always the current one, and each test gets its own copy. The migration path
// itself stays covered by internal/store/sqlite and internal/store/migrations,
// which still start from an empty file.
package sqlitetest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/store/sqlite"
)

var (
	once     sync.Once
	template []byte
	buildErr error
)

// Path returns a fresh, fully migrated database file under t.TempDir().
func Path(t testing.TB) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tix.db")
	Seed(t, path)
	return path
}

// Seed writes a fully migrated database at path, creating its directory.
func Seed(t testing.TB, path string) {
	t.Helper()
	once.Do(func() { template, buildErr = build() })
	if buildErr != nil {
		t.Fatalf("building the migrated template: %v", buildErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("creating %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, template, 0o600); err != nil {
		t.Fatalf("writing %q: %v", path, err)
	}
}

func build() ([]byte, error) {
	dir, err := os.MkdirTemp("", "tix-sqlitetest-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	src := filepath.Join(dir, "src.db")
	st, err := sqlite.Open(src, nil)
	if err != nil {
		return nil, err
	}
	if err := st.Migrate(context.Background()); err != nil {
		_ = st.Close()
		return nil, err
	}
	if err := st.Close(); err != nil {
		return nil, err
	}

	// Closing the last connection checkpoints the write-ahead log into the
	// main file and removes it. A log left behind holds schema a copy of the
	// main file alone would lose, so that is refused rather than copied.
	if info, err := os.Stat(src + "-wal"); err == nil && info.Size() > 0 {
		return nil, fmt.Errorf("%s-wal survived closing the store with %d bytes", src, info.Size())
	}
	return os.ReadFile(src)
}
