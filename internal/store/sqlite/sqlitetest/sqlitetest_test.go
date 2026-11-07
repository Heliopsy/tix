// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlitetest_test

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/store/migrations"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
)

// A copy has to be indistinguishable from a database migrated in place: at
// the latest version, with nothing left for Migrate to do, and its own file.
func TestPathIsAFullyMigratedCopy(t *testing.T) {
	ctx := context.Background()
	want, err := migrations.Latest()
	if err != nil {
		t.Fatalf("latest migration: %v", err)
	}
	first, second := sqlitetest.Path(t), sqlitetest.Path(t)
	if first == second {
		t.Fatalf("two calls returned the same file %q", first)
	}
	for _, path := range []string{first, second} {
		st, err := sqlite.Open(path, nil)
		if err != nil {
			t.Fatalf("opening %s: %v", path, err)
		}
		t.Cleanup(func() { _ = st.Close() })
		got, err := st.SchemaVersion(ctx)
		if err != nil {
			t.Fatalf("schema version: %v", err)
		}
		if got != want {
			t.Errorf("%s is at version %d, want %d", path, got, want)
		}
		if err := st.Migrate(ctx); err != nil {
			t.Errorf("migrating a copy: %v", err)
		}
	}
}
