// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The directory is what a person or an agent is assigned by, so listing it
// has to name the agents too. The seeded fixture holds four people and three
// agents, and all seven must come back.
func TestActorLsListsPeopleAndAgents(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "actors.db")
	c.mustRun("--db", db, "demo", "seed", "--days", "7")

	var actors []core.Actor
	if err := json.Unmarshal([]byte(c.mustRun("--db", db, "actor", "ls", "-o", "json").out), &actors); err != nil {
		t.Fatalf("actor ls is not json: %v", err)
	}
	handles := map[string]bool{}
	for _, a := range actors {
		if a.Handle == "" {
			t.Errorf("actor %q has no handle to offer", a.ID)
		}
		handles[a.Handle] = true
	}

	cases := []struct {
		name   string
		handle string
	}{
		{"a person", "nadia"},
		{"another person", "raj"},
		{"a build agent", "atlas"},
		{"a triage agent", "scout"},
		{"a docs agent", "mint"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !handles[tc.handle] {
				t.Errorf("actor ls did not name %q: %v", tc.handle, handles)
			}
		})
	}
}

func TestActorLsLimitIsHonoured(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "actors.db")
	c.mustRun("--db", db, "demo", "seed", "--days", "7")

	got := c.mustRun("--db", db, "actor", "ls", "--limit", "2", "-o", "json")
	var actors []core.Actor
	if err := json.Unmarshal([]byte(got.out), &actors); err != nil {
		t.Fatalf("actor ls is not json: %v", err)
	}
	if len(actors) != 2 {
		t.Fatalf("actor ls --limit 2 returned %d actors", len(actors))
	}
	if got.err == "" {
		t.Error("a truncated listing reported no next cursor on stderr")
	}
}
