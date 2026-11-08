// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A person whose membership was revoked keeps an actor for history, and
// `user ls` used to list them as a current user with no role. They are left
// out now, and --include-removed lists them marked removed, on either engine.
func TestUserLsLeavesOutRemovedPeople(t *testing.T) {
	engines := []struct {
		name string
		db   func(t *testing.T) []string
	}{
		{"sqlite", func(*testing.T) []string { return nil }},
		{"postgres", func(t *testing.T) []string { return []string{"--db", freshPostgresDSN(t)} }},
	}
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			c := newCLI(t)
			db := engine.db(t)
			run := func(args ...string) string { return c.mustRun(append(append([]string{}, db...), args...)...).out }
			var gone, kept core.User
			for email, into := range map[string]*core.User{"gone@example.com": &gone, "kept@example.com": &kept} {
				out := run("user", "create", email, "--password", "correct-horse-battery", "-o", "json")
				if err := json.Unmarshal([]byte(out), into); err != nil {
					t.Fatalf("decoding %q: %v", out, err)
				}
			}
			run("member", "rm", gone.ID)

			listed := func(args ...string) map[string]bool {
				t.Helper()
				var users []core.User
				out := run(append([]string{"user", "ls", "-o", "json"}, args...)...)
				if err := json.Unmarshal([]byte(out), &users); err != nil {
					t.Fatalf("decoding %q: %v", out, err)
				}
				ids := map[string]bool{}
				for _, u := range users {
					ids[u.ID] = u.Removed
				}
				return ids
			}
			current := listed()
			if _, ok := current[gone.ID]; ok {
				t.Error("user ls lists a person whose membership was revoked")
			}
			if removed, ok := current[kept.ID]; !ok || removed {
				t.Errorf("user ls: kept listed %v, marked removed %v; want listed, not removed", ok, removed)
			}
			if removed, ok := listed("--include-removed")[gone.ID]; !ok || !removed {
				t.Errorf("user ls --include-removed: gone listed %v, marked removed %v; want both", ok, removed)
			}
			if table := run("user", "ls", "--include-removed"); !strings.Contains(table, "(removed)") {
				t.Errorf("the table does not mark the removed person:\n%s", table)
			}
		})
	}
}
