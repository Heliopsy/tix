// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestMigrationDropsTheIndexedColumnAndKeepsTheField rewinds to the schema
// before 0029, with a field definition marked indexed, and migrates again.
func TestMigrationDropsTheIndexedColumnAndKeepsTheField(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.PutFieldDef(ctx, &core.FieldDef{ProjectID: f.project.ID, Key: "severity",
			Label: "Severity", Type: core.FieldString, Required: true, Position: 3})
	}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"ALTER TABLE field_defs ADD COLUMN indexed INTEGER NOT NULL DEFAULT 1",
		"ALTER TABLE tenants DROP COLUMN time_format",
		"ALTER TABLE tenants DROP COLUMN timezone",
		"ALTER TABLE users DROP COLUMN time_format",
		"ALTER TABLE users DROP COLUMN timezone",
		"DELETE FROM schema_migrations WHERE version > 28",
	} {
		if _, err := s.writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("rewinding to 0028 (%s): %v", stmt, err)
		}
	}

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	var columns int
	if err := s.writer.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('field_defs') WHERE name = 'indexed'").Scan(&columns); err != nil {
		t.Fatalf("reading the columns: %v", err)
	}
	if columns != 0 {
		t.Error("field_defs.indexed survived the migration")
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		defs, err := tx.ListFieldDefs(ctx, f.project.ID)
		if err != nil {
			return err
		}
		if len(defs) != 1 || defs[0].Key != "severity" || !defs[0].Required || defs[0].Position != 3 {
			t.Fatalf("field definitions after the migration = %+v", defs)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
