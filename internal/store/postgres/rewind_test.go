// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/store/migrations/migrationtest"
)

// rewindTo takes a migrated store back to the given version, translating
// the portable statements as the migrations themselves are, so the next
// Migrate applies every later migration to whatever rows it now holds.
func rewindTo(t *testing.T, s *Store, version int) {
	t.Helper()
	stmts, err := migrationtest.Rewind(version)
	if err != nil {
		t.Fatalf("deriving the rewind to %d: %v", version, err)
	}
	for _, portable := range stmts {
		translated, err := translate(portable)
		if err != nil {
			t.Fatalf("translating %q: %v", portable, err)
		}
		for _, stmt := range translated {
			if _, err := s.db.ExecContext(context.Background(), stmt); err != nil {
				t.Fatalf("rewinding to %d (%s): %v", version, stmt, err)
			}
		}
	}
}
