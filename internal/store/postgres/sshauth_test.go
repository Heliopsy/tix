// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// rawSSHKeyRows runs the unscoped fingerprint lookup with no flag raised, which
// is what the isolation policy alone admits.
func rawSSHKeyRows(t *testing.T, tr *tx, fingerprint string) int {
	t.Helper()
	ctx := context.Background()
	q, args := sqlb.SSHKeysByFingerprintQuery(dialect, sshKeyColumns, fingerprint)
	rows, err := tr.ex.QueryContext(ctx, q, args...)
	if err != nil {
		t.Fatalf("reading ssh keys without the flag: %v", err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading ssh keys without the flag: %v", err)
	}
	return n
}

func seedSharedFingerprint(t *testing.T, s *Store, clk *clock.Fake) (fixture, fixture) {
	t.Helper()
	acme := seed(t, s, clk, "acme")
	globex := seed(t, s, clk, "globex")
	enrolKey(t, acme, "SHA256:shared", "acme")
	enrolKey(t, globex, "SHA256:shared", "globex")
	return acme, globex
}

// TestSSHAuthPolicyIsNarrow pins the five properties the cross-tenant lookup
// rests on. The policy widens one read on one table and nothing else.
func TestSSHAuthPolicyIsNarrow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	acme, _ := seedSharedFingerprint(t, s, clk)

	t.Run("closed by default", func(t *testing.T) {
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			if n := rawSSHKeyRows(t, u.(*tx), "SHA256:shared"); n != 0 {
				t.Fatalf("an unscoped read with no flag returned %d rows, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatalf("probing the closed default: %v", err)
		}
	})

	t.Run("one statement sees every tenant", func(t *testing.T) {
		var found []core.SSHKey
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			var err error
			found, err = u.FindSSHKeysByFingerprint(ctx, "SHA256:shared")
			return err
		}); err != nil {
			t.Fatalf("finding ssh keys by fingerprint: %v", err)
		}
		if len(found) != 2 {
			t.Fatalf("the lookup returned %d enrolments, want 2: %+v", len(found), found)
		}
	})

	t.Run("the flag does not outlive its transaction", func(t *testing.T) {
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			_, err := u.FindSSHKeysByFingerprint(ctx, "SHA256:shared")
			return err
		}); err != nil {
			t.Fatalf("raising the flag: %v", err)
		}
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			if n := rawSSHKeyRows(t, u.(*tx), "SHA256:shared"); n != 0 {
				t.Fatalf("a later transaction still read %d rows, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatalf("probing the next transaction: %v", err)
		}
	})

	t.Run("the flag admits no insert", func(t *testing.T) {
		err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			tr := u.(*tx)
			if err := tr.sshAuthLookup(ctx, true); err != nil {
				return err
			}
			_, err := tr.ex.ExecContext(ctx,
				`INSERT INTO ssh_keys (id, tenant_id, actor_id, fingerprint, public_key, label, created_at)
				 VALUES ($1, $2, $3, $4, $5, $6, now())`,
				"forged", acme.tenant.ID, acme.actor.ID, "SHA256:forged", "ssh-ed25519 AAAA", "")
			return err
		})
		if err == nil {
			t.Fatal("an insert under the flag succeeded, want it refused")
		}
	})

	t.Run("the flag admits no update", func(t *testing.T) {
		var affected int64
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			tr := u.(*tx)
			if err := tr.sshAuthLookup(ctx, true); err != nil {
				return err
			}
			res, err := tr.ex.ExecContext(ctx,
				`UPDATE ssh_keys SET label = $1 WHERE fingerprint = $2`, "seized", "SHA256:shared")
			if err != nil {
				return err
			}
			affected, err = res.RowsAffected()
			return err
		}); err != nil {
			t.Fatalf("attempting an update under the flag: %v", err)
		}
		if affected != 0 {
			t.Fatalf("an update under the flag reached %d rows, want 0", affected)
		}
	})

	t.Run("ordinary scoped reads are unaffected", func(t *testing.T) {
		if err := s.View(ctx, acme.scope, func(tx store.Tx) error {
			keys, err := tx.ListSSHKeys(ctx, acme.actor.ID)
			if err != nil {
				return err
			}
			if len(keys) != 1 || keys[0].TenantID != acme.tenant.ID {
				t.Fatalf("a scoped read returned %+v", keys)
			}
			return nil
		}); err != nil {
			t.Fatalf("reading in scope: %v", err)
		}
	})
}

// countingExecutor records how many statements a call issues.
type countingExecutor struct {
	inner executor
	n     atomic.Int64
}

func (c *countingExecutor) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	c.n.Add(1)
	return c.inner.ExecContext(ctx, q, args...)
}

func (c *countingExecutor) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)
	return c.inner.QueryContext(ctx, q, args...)
}

func (c *countingExecutor) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	c.n.Add(1)
	return c.inner.QueryRowContext(ctx, q, args...)
}

func lookupCost(t *testing.T, s *Store, fingerprint string) (int64, int) {
	t.Helper()
	ctx := context.Background()
	var (
		statements int64
		rows       int
	)
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		tr := u.(*tx)
		counter := &countingExecutor{inner: tr.ex}
		tr.ex = counter
		defer func() { tr.ex = counter.inner }()

		found, err := u.FindSSHKeysByFingerprint(ctx, fingerprint)
		if err != nil {
			return err
		}
		statements = counter.n.Load()
		rows = len(found)
		return nil
	}); err != nil {
		t.Fatalf("measuring the lookup: %v", err)
	}
	return statements, rows
}

// TestSSHKeyLookupCostDoesNotGrowWithTenants guards the pre-authentication path.
// Any connection presenting any key reaches this lookup before anything is known
// about the caller, so a cost that rises with the tenant count would let an
// unauthenticated connection amplify into that many round trips.
func TestSSHKeyLookupCostDoesNotGrowWithTenants(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	enrolKey(t, f, "SHA256:shared", "laptop")

	small, rows := lookupCost(t, s, "SHA256:shared")
	if rows != 1 {
		t.Fatalf("the lookup found %d enrolments in one tenant, want 1", rows)
	}

	for i := 0; i < 19; i++ {
		tenant := core.Tenant{Key: fmt.Sprintf("bystander-%02d", i), Name: "bystander"}
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			return u.CreateTenant(ctx, &tenant)
		}); err != nil {
			t.Fatalf("creating a bystanding tenant: %v", err)
		}
	}

	large, rows := lookupCost(t, s, "SHA256:shared")
	if rows != 1 {
		t.Fatalf("the lookup found %d enrolments with 20 tenants, want 1", rows)
	}
	if small != large {
		t.Fatalf("the lookup issued %d statements with 1 tenant and %d with 20; it must be constant",
			small, large)
	}
}

// TestSSHAuthSettingIsRaisedInOnePlace makes the safety argument enforceable.
// The flag is safe because exactly one function raises it and lowers it again;
// a second writer somewhere in the tree would make that claim untrue quietly.
func TestSSHAuthSettingIsRaisedInOnePlace(t *testing.T) {
	root := repoRoot(t)
	literals := map[string]int{}
	identifiers := map[string]int{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if n := strings.Count(string(body), `"tix.ssh_auth"`); n > 0 {
			literals[rel] = n
		}
		if n := strings.Count(string(body), "sshAuthSetting"); n > 0 {
			identifiers[rel] = n
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}

	const (
		schemaFile = "internal/store/postgres/schema.go"
		lookupFile = "internal/store/postgres/sshkey.go"
	)
	if len(literals) != 1 || literals[schemaFile] != 1 {
		t.Fatalf("the setting name should be spelled once, in %s; found %v", schemaFile, literals)
	}
	if len(identifiers) != 2 {
		t.Fatalf("only %s and %s may name the setting; found %v", schemaFile, lookupFile, identifiers)
	}
	if identifiers[lookupFile] != 1 {
		t.Fatalf("%s raises the flag in %d places, want exactly 1", lookupFile, identifiers[lookupFile])
	}
	if identifiers[schemaFile] == 0 {
		t.Fatalf("%s no longer declares the setting: %v", schemaFile, identifiers)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("%q is not the repository root: %v", dir, err)
	}
	return dir
}
