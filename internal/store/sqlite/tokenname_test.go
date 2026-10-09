// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/store"
)

// insertToken writes one token row straight through, which is how a database
// that predates the unique index can be made to hold a duplicate at all.
func insertToken(t *testing.T, s *Store, f fixture, id, name string) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.writer.ExecContext(ctx,
		`INSERT INTO api_tokens (id, tenant_id, actor_id, name, token_hash, scopes, created_at)
		 VALUES (?, ?, ?, ?, ?, '["task:read"]', '2026-01-01T00:00:00Z')`,
		id, f.tenant.ID, f.actor.ID, name, "hash-"+id,
	); err != nil {
		t.Fatalf("inserting token %q: %v", id, err)
	}
}

// tokenNames returns every token name of the tenant that is not revoked, by
// identifier, read straight from the table so the assertion does not depend on
// the listing it is meant to be fixing.
func tokenNames(t *testing.T, s *Store, f fixture) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := s.writer.QueryContext(ctx,
		`SELECT id, name FROM api_tokens WHERE tenant_id = ? AND revoked_at IS NULL`, f.tenant.ID)
	if err != nil {
		t.Fatalf("reading token names: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scanning token name: %v", err)
		}
		out[id] = name
	}
	return out
}

// TestUpgradeKeepsDuplicateTokensWorkingAndTellsThemApart is the upgrade path
// for a database that already holds two live tokens of the same name, which is
// the very defect the index closes, so it cannot be assumed away.
//
// Renamed rather than revoked: revoking would stop an agent that is working
// right now in order to tidy a listing. The oldest identifier keeps the name
// people refer to it by.
func TestUpgradeKeepsDuplicateTokensWorkingAndTellsThemApart(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	for _, stmt := range []string{
		"DROP INDEX idx_tokens_name_live",
		"DELETE FROM schema_migrations WHERE version > 9",
	} {
		if _, err := s.writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("rewinding below the index (%s): %v", stmt, err)
		}
	}
	insertToken(t, s, f, "01AAAAAAAAAAAAAAAAAAAAAAAA", "ci")
	insertToken(t, s, f, "01BBBBBBBBBBBBBBBBBBBBBBBB", "ci")
	insertToken(t, s, f, "01CCCCCCCCCCCCCCCCCCCCCCCC", "release")

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating a database holding duplicate token names: %v", err)
	}

	names := tokenNames(t, s, f)
	if len(names) != 3 {
		t.Fatalf("%d live tokens after the upgrade, want 3: %v", len(names), names)
	}
	if got := names["01AAAAAAAAAAAAAAAAAAAAAAAA"]; got != "ci" {
		t.Errorf("the oldest duplicate is now named %q, want it to keep ci", got)
	}
	renamed := names["01BBBBBBBBBBBBBBBBBBBBBBBB"]
	if renamed == "ci" {
		t.Errorf("the second token is still named ci, so the index cannot exist")
	}
	if !strings.Contains(renamed, "01BBBBBBBBBBBBBBBBBBBBBBBB") {
		t.Errorf("the renamed token is called %q, which does not say which row it is", renamed)
	}
	if got := names["01CCCCCCCCCCCCCCCCCCCCCCCC"]; got != "release" {
		t.Errorf("a token that was never duplicated is now named %q", got)
	}
}

// TestTokenNameInUseIsScopedToTheTenant is the read the service's rule runs
// on, so it must not see another tenant's names.
func TestTokenNameInUseIsScopedToTheTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	mine := seed(t, s, clk, "acme")
	theirs := seed(t, s, clk, "beta")
	insertToken(t, s, theirs, "01DDDDDDDDDDDDDDDDDDDDDDDD", "ci")

	var taken bool
	if err := s.View(ctx, mine.scope, func(tx store.Tx) error {
		var err error
		taken, err = tx.TokenNameInUse(ctx, "ci")
		return err
	}); err != nil {
		t.Fatalf("TokenNameInUse: %v", err)
	}
	if taken {
		t.Error("another tenant's token name reads as taken here")
	}

	insertToken(t, s, mine, "01EEEEEEEEEEEEEEEEEEEEEEEE", "ci")
	if err := s.View(ctx, mine.scope, func(tx store.Tx) error {
		var err error
		taken, err = tx.TokenNameInUse(ctx, "ci")
		return err
	}); err != nil {
		t.Fatalf("TokenNameInUse: %v", err)
	}
	if !taken {
		t.Error("this tenant's own token name does not read as taken")
	}
}

// TestRevokedTokenNamesDoNotBlockTheIndex pins the window the index covers, so
// the database agrees with the service about what rotation means.
func TestRevokedTokenNamesDoNotBlockTheIndex(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	insertToken(t, s, f, "01FFFFFFFFFFFFFFFFFFFFFFFF", "ci")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.RevokeToken(ctx, "01FFFFFFFFFFFFFFFFFFFFFFFF", clk.Now())
	}); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	insertToken(t, s, f, "01GGGGGGGGGGGGGGGGGGGGGGGG", "ci")

	var taken bool
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		taken, err = tx.TokenNameInUse(ctx, "ci")
		return err
	}); err != nil {
		t.Fatalf("TokenNameInUse: %v", err)
	}
	if !taken {
		t.Error("the reissued token's name does not read as taken")
	}
	if _, err := s.writer.ExecContext(ctx,
		`INSERT INTO api_tokens (id, tenant_id, actor_id, name, token_hash, scopes, created_at)
		 VALUES ('01HHHHHHHHHHHHHHHHHHHHHHHH', ?, ?, 'ci', 'hash-h', '[]', '2026-01-01T00:00:00Z')`,
		f.tenant.ID, f.actor.ID,
	); err == nil {
		t.Error("the database accepted a third live token named ci")
	}
}
