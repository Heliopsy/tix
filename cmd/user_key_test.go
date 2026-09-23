// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/core"
)

// sshTestKey returns an authorized_keys line for a fresh key, the canonical
// form of that key alone, and the fingerprint an SSH client prints for it.
func sshTestKey(t *testing.T, comment string) (line, canonical, fingerprint string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	key, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	canonical = strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key)))
	return canonical + " " + comment, canonical, gossh.FingerprintSHA256(key)
}

// writeKeyFile puts a key where the command can read it.
func (c *cli) writeKeyFile(name, body string) string {
	c.t.Helper()
	path := filepath.Join(c.home, name)
	if err := os.WriteFile(path, []byte(body+"\n"), 0o600); err != nil {
		c.t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func decodeSSHKey(t *testing.T, out string) core.SSHKey {
	t.Helper()
	var k core.SSHKey
	if err := json.Unmarshal([]byte(out), &k); err != nil {
		t.Fatalf("decoding an ssh key from %q: %v", out, err)
	}
	return k
}

func decodeSSHKeys(t *testing.T, out string) []core.SSHKey {
	t.Helper()
	var keys []core.SSHKey
	if err := json.Unmarshal([]byte(out), &keys); err != nil {
		t.Fatalf("decoding ssh keys from %q: %v", out, err)
	}
	return keys
}

// TestUserKeyAddListAndRevokeRoundTripThroughTheCLI drives the whole group
// over the real command tree: what `add` records, what `ls` reports, and what
// is left after `rm`.
func TestUserKeyAddListAndRevokeRoundTripThroughTheCLI(t *testing.T) {
	c := newCLI(t)
	line, canonical, fingerprint := sshTestKey(t, "alice@laptop")
	path := c.writeKeyFile("id_ed25519.pub", line)

	added := decodeSSHKey(t, c.mustRun("user", "key", "add", "--file", path, "--label", "laptop", "-o", "json").out)
	if added.Fingerprint != fingerprint {
		t.Fatalf("enrolled fingerprint = %q, want the one an ssh client prints: %q", added.Fingerprint, fingerprint)
	}
	if added.PublicKey != canonical {
		t.Fatalf("stored key = %q, want the key alone: %q", added.PublicKey, canonical)
	}
	if strings.Contains(added.PublicKey, "alice@laptop") {
		t.Fatalf("the submitted comment was stored: %q", added.PublicKey)
	}
	if added.Label != "laptop" || added.ID == "" || added.ActorID == "" {
		t.Fatalf("enrolled = %+v, want a labelled key against an actor", added)
	}
	if added.RevokedAt != nil {
		t.Fatalf("a fresh enrolment is already revoked: %+v", added)
	}

	listed := decodeSSHKeys(t, c.mustRun("user", "key", "ls", "-o", "json").out)
	if len(listed) != 1 || listed[0].ID != added.ID {
		t.Fatalf("ls reported %+v, want the one key just enrolled", listed)
	}

	revoked := c.mustRun("user", "key", "rm", added.ID, "-o", "json")
	if !strings.Contains(revoked.out, statusOK) {
		t.Fatalf("rm reported %q, want an outcome", revoked.out)
	}
	if !strings.Contains(revoked.err, "idle timeout") {
		t.Errorf("rm did not say that live sessions are not cut: %q", revoked.err)
	}

	// A revoked key is reported as revoked rather than omitted: a key that
	// stopped working is the one being looked for.
	after := decodeSSHKeys(t, c.mustRun("user", "key", "ls", "-o", "json").out)
	if len(after) != 1 {
		t.Fatalf("ls after a revocation reported %d keys, want the revoked one still listed: %+v", len(after), after)
	}
	if after[0].ID != added.ID {
		t.Fatalf("ls reported %q, want the revoked key %q", after[0].ID, added.ID)
	}
	if after[0].RevokedAt == nil {
		t.Fatalf("the revoked key is listed as live: %+v", after[0])
	}

	// The default rendering shows it too, so an operator who did not ask for
	// JSON still sees the key that stopped working.
	table := c.mustRun("user", "key", "ls").out
	if !strings.Contains(table, added.ID) {
		t.Fatalf("the default listing omits the revoked key: %s", table)
	}
	if !strings.Contains(table, after[0].RevokedAt.Format("2006-01-02")) {
		t.Fatalf("the default listing does not mark the key revoked: %s", table)
	}
}

// TestUserKeyAddReadsAKeyFromStandardInput covers `--file -`, which is how a
// key is piped in without ever reaching the process table.
func TestUserKeyAddReadsAKeyFromStandardInput(t *testing.T) {
	c := newCLI(t)
	line, canonical, fingerprint := sshTestKey(t, "alice@laptop")

	got := c.runIn(line+"\n", "user", "key", "add", "--file", "-", "--label", "piped", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	added := decodeSSHKey(t, got.out)
	if added.Fingerprint != fingerprint || added.PublicKey != canonical {
		t.Fatalf("piped enrolment = %+v, want fingerprint %q and key %q", added, fingerprint, canonical)
	}
	if added.Label != "piped" {
		t.Fatalf("label = %q, want piped", added.Label)
	}

	listed := decodeSSHKeys(t, c.mustRun("user", "key", "ls", "-o", "json").out)
	if len(listed) != 1 || listed[0].Fingerprint != fingerprint {
		t.Fatalf("ls reported %+v, want the piped key", listed)
	}
}

// TestUserKeyAddDryRunRecordsNothing asserts the plan is a plan.
func TestUserKeyAddDryRunRecordsNothing(t *testing.T) {
	c := newCLI(t)
	line, _, _ := sshTestKey(t, "alice@laptop")
	path := c.writeKeyFile("id_ed25519.pub", line)

	got := c.mustRun("user", "key", "add", "--file", path, "--label", "laptop", "--dry-run", "-o", "json")
	if !strings.Contains(got.out, statusPlanned) {
		t.Fatalf("output = %q, want a plan", got.out)
	}
	if !strings.Contains(got.out, "sshkey.enrol") {
		t.Fatalf("the plan does not name the action: %q", got.out)
	}

	listed := decodeSSHKeys(t, c.mustRun("user", "key", "ls", "-o", "json").out)
	if len(listed) != 0 {
		t.Fatalf("a dry run enrolled %+v", listed)
	}

	// A dry run must not validate less than the real thing: text that is not a
	// key is refused before anything is planned.
	bad := c.writeKeyFile("not-a-key.pub", "this is not a public key")
	if got := c.run("user", "key", "add", "--file", bad, "--dry-run"); got.code != core.ExitUsage {
		t.Fatalf("a dry run of an invalid key exited %d, want %d\n%s", got.code, core.ExitUsage, got.err)
	}
}

// TestUserKeyExitCodesAreTheOnesTheHelpPromises checks the promise and the
// behaviour against each other. Help text drifts away from the code silently,
// so each case names the phrase the help must carry and the code the command
// must actually return.
func TestUserKeyExitCodesAreTheOnesTheHelpPromises(t *testing.T) {
	c := newCLI(t)
	line, _, _ := sshTestKey(t, "alice@laptop")
	path := c.writeKeyFile("id_ed25519.pub", line)
	other, _, _ := sshTestKey(t, "alice@desktop")
	otherPath := c.writeKeyFile("id_other.pub", other)
	badPath := c.writeKeyFile("not-a-key.pub", "this is not a public key")

	enrolled := decodeSSHKey(t, c.mustRun("user", "key", "add", "--file", path, "-o", "json").out)

	// A token holding a scope that is not token admin: the caller is known and
	// refused, which is permission denied rather than a bad credential.
	var minted struct {
		Token string `json:"token"`
	}
	created := c.mustRun("token", "create", "agent", "--scope", "task:read", "--expires", "2030-01-01", "-o", "json").out
	if err := json.Unmarshal([]byte(created), &minted); err != nil {
		t.Fatalf("decoding the token from %q: %v", created, err)
	}
	if minted.Token == "" {
		t.Fatalf("no token value in %q", created)
	}

	tests := []struct {
		name    string
		help    []string
		promise string
		args    []string
		want    int
	}{
		{
			name:    "invalid key",
			help:    []string{"user", "key", "add", "--help"},
			promise: "2 invalid key",
			args:    []string{"user", "key", "add", "--file", badPath},
			want:    core.ExitUsage,
		},
		{
			name:    "no key at all",
			help:    []string{"user", "key", "add", "--help"},
			promise: "2 invalid key",
			args:    []string{"user", "key", "add"},
			want:    core.ExitUsage,
		},
		{
			name:    "unknown actor on add",
			help:    []string{"user", "key", "add", "--help"},
			promise: "3 unknown actor",
			args:    []string{"user", "key", "add", "--file", otherPath, "--actor", "nobody"},
			want:    core.ExitNotFound,
		},
		{
			name:    "already enrolled",
			help:    []string{"user", "key", "add", "--help"},
			promise: "4 key already enrolled",
			args:    []string{"user", "key", "add", "--file", path},
			want:    core.ExitConflict,
		},
		{
			name:    "permission denied on add",
			help:    []string{"user", "key", "add", "--help"},
			promise: "5 permission denied",
			args:    []string{"--token", minted.Token, "user", "key", "add", "--file", otherPath},
			want:    core.ExitPermission,
		},
		{
			name:    "unknown actor on ls",
			help:    []string{"user", "key", "ls", "--help"},
			promise: "3 unknown actor",
			args:    []string{"user", "key", "ls", "--actor", "nobody"},
			want:    core.ExitNotFound,
		},
		{
			name:    "permission denied on ls",
			help:    []string{"user", "key", "ls", "--help"},
			promise: "5 permission denied",
			args:    []string{"--token", minted.Token, "user", "key", "ls"},
			want:    core.ExitPermission,
		},
		{
			name:    "unknown key on rm",
			help:    []string{"user", "key", "rm", "--help"},
			promise: "3 unknown key",
			args:    []string{"user", "key", "rm", "01JB2K3M4N5P6Q7R8S9TNOBODY"},
			want:    core.ExitNotFound,
		},
		{
			name:    "permission denied on rm",
			help:    []string{"user", "key", "rm", "--help"},
			promise: "5 permission denied",
			args:    []string{"--token", minted.Token, "user", "key", "rm", enrolled.ID},
			want:    core.ExitPermission,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			help := c.mustRun(tc.help...).out
			if !strings.Contains(help, tc.promise) {
				t.Fatalf("the help does not promise %q:\n%s", tc.promise, help)
			}
			got := c.run(tc.args...)
			if got.code != tc.want {
				t.Fatalf("exit = %d, want %d as %q promises\nstdout: %s\nstderr: %s",
					got.code, tc.want, tc.promise, got.out, got.err)
			}
		})
	}

	// Nothing above was recorded: every refusal left the one enrolled key alone.
	listed := decodeSSHKeys(t, c.mustRun("user", "key", "ls", "-o", "json").out)
	if len(listed) != 1 || listed[0].ID != enrolled.ID {
		t.Fatalf("a refused enrolment was recorded: %+v", listed)
	}
}
