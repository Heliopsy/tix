// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build smoke

package smoke

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestZeroConfigurationPathWorksWithNoConfigAnywhere is the path a person
// takes the first time: an empty home, no configuration file, no --db, no
// tenant and no project. The first command refuses to create a database
// nobody asked for and says how to get one; after `tix init`, which is the
// whole of the setup, it has to create the default tenant and project on its
// own and keep working across four commands.
func TestZeroConfigurationPathWorksWithNoConfigAnywhere(t *testing.T) {
	s := newScratch(t)

	refused := s.run("task", "add", "buy milk")
	if refused.code != 6 {
		t.Fatalf("task add with nothing configured exited %d, want 6\nstderr:\n%s", refused.code, refused.err)
	}
	for _, want := range []string{"tix ctx add NAME --server URL --token TOKEN --use", "tix init"} {
		if !strings.Contains(refused.err, want) {
			t.Fatalf("the refusal does not name %q:\n%s", want, refused.err)
		}
	}
	if _, err := os.Stat(s.path("data/tix/tix.db")); err == nil {
		t.Fatal("the refused command created the zero-configuration store anyway")
	}

	s.mustRun("init")
	added := s.mustRun("task", "add", "buy milk")
	if !strings.Contains(added.err, "using local database "+s.path("data/tix/tix.db")) {
		t.Fatalf("task add did not name the local database it wrote to:\n%s", added.err)
	}
	if !strings.Contains(added.out, "buy milk") {
		t.Fatalf("task add did not show the task:\n%s", added.out)
	}
	ref := refOf(t, added.out)
	if !strings.HasPrefix(ref, "DEFAULT-") {
		t.Fatalf("ref = %q, want the default project to have been created", ref)
	}

	listed := s.mustRun("task", "ls")
	if !strings.Contains(listed.out, "buy milk") {
		t.Fatalf("task ls did not list the task:\n%s", listed.out)
	}
	assertRendered(t, "task ls", listed.out,
		"REF", "TITLE", "STATUS", "PRIORITY", "ASSIGNEE", "TAGS", "DUE", "REPEAT", "UPDATED", "WAITING")

	claimed := s.mustRun("claim", "next")
	if !strings.Contains(claimed.out, "buy milk") {
		t.Fatalf("claim next did not hand out the task:\n%s", claimed.out)
	}

	shown := s.mustRun("task", "show", ref)
	if !strings.Contains(shown.out, "buy milk") {
		t.Fatalf("task show %s did not show the task:\n%s", ref, shown.out)
	}

	if _, err := os.Stat(s.path("data/tix/tix.db")); err != nil {
		t.Fatalf("the zero-configuration store was not created under XDG_DATA_HOME: %v", err)
	}
	if entries, err := filepath.Glob(s.path("config/tix/*")); err == nil && len(entries) > 0 {
		t.Logf("configuration written on the zero-configuration path: %v", entries)
	}

	// A database already at the default path is the opt-in an install from
	// before the refusal has. Without target: local, it still opens.
	if err := os.Remove(s.path("config/tix/config.yaml")); err != nil {
		t.Fatalf("removing the configuration tix init wrote: %v", err)
	}
	if again := s.mustRun("task", "show", ref); !strings.Contains(again.out, "buy milk") {
		t.Fatalf("an existing database at the default path was not used:\n%s", again.out)
	}
}

// TestListingsRenderForAPerson covers the class of bug a service-level test
// cannot see: the data was right and the screen was not.
func TestListingsRenderForAPerson(t *testing.T) {
	s := newScratch(t)
	db := s.path("smoke.db")

	s.mustRun("--db", db, "task", "add", "buy milk")
	s.mustRun("--db", db, "project", "create", "mine", "Mine")
	s.mustRun("--db", db, "user", "create", "smoke@example.test", "--handle", "smoke", "--role", "admin")

	key := filepath.Join(t.TempDir(), "id_ed25519.pub")
	writeFile(t, key, samplePublicKey)
	s.mustRun("--db", db, "user", "key", "add", "--file", key, "--actor", "smoke", "--label", "laptop")

	t.Run("task ls", func(t *testing.T) {
		out := s.mustRun("--db", db, "task", "ls").out
		assertRendered(t, "task ls", out,
			"REF", "TITLE", "STATUS", "PRIORITY", "ASSIGNEE", "TAGS", "DUE", "REPEAT", "UPDATED", "WAITING")
	})

	t.Run("project ls", func(t *testing.T) {
		out := s.mustRun("--db", db, "project", "ls").out
		assertRendered(t, "project ls", out,
			"KEY", "NAME", "WORKFLOW", "COLOR", "ICON", "ARCHIVED", "CREATED", "UPDATED")
		if !strings.Contains(out, "MINE") {
			t.Errorf("project ls did not list the project that was created:\n%s", out)
		}
	})

	// The listing that actually broke. Its row carries three optional times,
	// two of them unset, and a public key that must not become a column.
	t.Run("user key ls", func(t *testing.T) {
		out := s.mustRun("--db", db, "user", "key", "ls", "--actor", "smoke").out
		assertRendered(t, "user key ls", out,
			"ID", "ACTOR", "LABEL", "FINGERPRINT", "CREATED", "LAST USED", "REVOKED")
		if strings.Contains(out, "AAAAC3NzaC1lZDI1NTE5") {
			t.Errorf("user key ls printed the public key itself as a column:\n%s", out)
		}
		if !strings.Contains(out, "laptop") {
			t.Errorf("user key ls did not list the enrolled key:\n%s", out)
		}
	})

	// Nothing is connected to a command-line process, so this proves the empty
	// state reads as an answer rather than a blank. A populated listing is
	// asserted against a running server in the serve test.
	t.Run("connection ls", func(t *testing.T) {
		got := s.mustRun("--db", db, "connection", "ls")
		if strings.Contains(got.out, "<nil>") {
			t.Errorf("connection ls printed <nil>:\n%s", got.out)
		}
		if !strings.Contains(got.out, "holding 0 connections") {
			t.Errorf("connection ls did not state what the server holds:\n%s", got.out)
		}
		if !strings.Contains(got.out, "No results.") {
			t.Errorf("connection ls left the empty state blank:\n%s", got.out)
		}
	})
}

// samplePublicKey is a public half and nothing else. It never authenticates
// anything here: the tests that connect generate their own key pair.
const samplePublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE0Ghy48BA+Uojq+RRsbC1GoYWo7bn+6oIf6QVL4kb+o smoke\n"

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// refOf reads the reference out of a rendered task table.
func refOf(t *testing.T, out string) string {
	t.Helper()
	rows := tableRows(out)
	if len(rows) < 2 {
		t.Fatalf("no task row to read a reference from:\n%s", out)
	}
	return rows[1][0]
}
