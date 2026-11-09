// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

const (
	firstPassword = "correct horse battery"
	nextPassword  = "a whole new passphrase"
)

// TestLoginWithoutATerminalNamesTheWebForAPasswordChange is the scripted
// path: a flagged account gets no session and is told where to go.
func TestLoginWithoutATerminalNamesTheWebForAPasswordChange(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", firstPassword, "--handle", "ada",
		"--role", "member", "--require-password-change")

	got := c.run("login", "--email", "ada@example.com", "--password", firstPassword, "-o", "json")
	if got.code != core.ExitPrecondtion {
		t.Fatalf("login exited %d, want %d\nstdout: %s\nstderr: %s", got.code, core.ExitPrecondtion, got.out, got.err)
	}
	if !strings.Contains(got.err, "password change is required") || !strings.Contains(got.err, "web") {
		t.Fatalf("stderr = %q, want it to say a change is required and name the web", got.err)
	}
	if strings.Contains(got.out, "token") {
		t.Fatalf("a session reached standard output: %q", got.out)
	}
}

// TestLoginOnATerminalChangesThePasswordAndSignsIn walks the prompt: the new
// password twice, then a session, and the flag is gone afterwards.
func TestLoginOnATerminalChangesThePasswordAndSignsIn(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", firstPassword, "--handle", "ada",
		"--role", "member", "--require-password-change")

	mismatch := c.runTTYEnv(nil, func(string) string { return nextPassword + "\nsomething else entirely\n" },
		"login", "--email", "ada@example.com", "--password", firstPassword, "-o", "json")
	if mismatch.code != core.ExitUsage || strings.Contains(mismatch.out, "token") {
		t.Fatalf("mismatched passwords exited %d with %q, want %d and no session", mismatch.code, mismatch.out, core.ExitUsage)
	}

	got := c.runTTYEnv(nil, func(string) string { return nextPassword + "\n" + nextPassword + "\n" },
		"login", "--email", "ada@example.com", "--password", firstPassword, "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("login exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	if !strings.Contains(got.err, "new password") {
		t.Fatalf("stderr = %q, want a prompt for the new password", got.err)
	}
	var session core.Session
	if err := json.Unmarshal([]byte(got.out), &session); err != nil || session.Token == "" {
		t.Fatalf("login printed no session: %v\n%s", err, got.out)
	}

	again := c.mustRun("login", "--email", "ada@example.com", "--password", nextPassword, "-o", "json")
	if !strings.Contains(again.out, "token") {
		t.Fatalf("the new password does not sign in on its own: %s", again.out)
	}
	if old := c.run("login", "--email", "ada@example.com", "--password", firstPassword); old.code == core.ExitOK {
		t.Fatal("the old password still signs in")
	}
}

// TestUserEditSetsTheFlag is the administrator asking for a change later.
func TestUserEditSetsTheFlag(t *testing.T) {
	c := newCLI(t)
	var user core.User
	out := c.mustRun("user", "create", "ada@example.com", "--password", firstPassword, "--handle", "ada", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &user); err != nil {
		t.Fatalf("user create is not json: %v", err)
	}
	edited := c.mustRun("user", "edit", user.ID, "--require-password-change", "-o", "json").out
	if !strings.Contains(edited, `"require_password_change": true`) {
		t.Fatalf("user edit did not set the flag:\n%s", edited)
	}
	if got := c.run("login", "--email", "ada@example.com", "--password", firstPassword); got.code != core.ExitPrecondtion {
		t.Fatalf("login after the edit exited %d, want %d", got.code, core.ExitPrecondtion)
	}
	cleared := c.mustRun("user", "edit", user.ID, "--require-password-change=false", "-o", "json").out
	if !strings.Contains(cleared, `"require_password_change": false`) {
		t.Fatalf("=false did not clear the flag:\n%s", cleared)
	}
}

// TestUserInviteManagesPendingInvites covers invite, ls, reissue and revoke.
func TestUserInviteManagesPendingInvites(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("user", "invite", "ada@example.com", "--role", "member", "-o", "json")
	var issued core.IssuedInvite
	if err := json.Unmarshal([]byte(got.out), &issued); err != nil {
		t.Fatalf("user invite is not json: %v\n%s", err, got.out)
	}
	if !strings.HasPrefix(issued.Token, "tix_inv_") || issued.Path != "/invite/"+issued.Token {
		t.Fatalf("issued = %+v, want a tix_inv_ token at /invite/TOKEN", issued)
	}
	if !strings.Contains(got.err, "send this link to them") || strings.Contains(got.out, "send this") {
		t.Fatalf("stderr = %q, want who sends the link, on standard error only", got.err)
	}
	if shown := c.mustRun("user", "invite", "bob@example.com").out; !strings.Contains(shown, "/invite/tix_inv_") {
		t.Fatalf("the table output carries no link:\n%s", shown)
	}

	table := c.mustRun("user", "invite", "ls").out
	if !strings.Contains(table, "ada@example.com") || strings.Contains(table, issued.Token) {
		t.Fatalf("invite ls = %q, want the address and no secret", table)
	}

	var next core.IssuedInvite
	if err := json.Unmarshal([]byte(c.mustRun("user", "invite", "reissue", issued.ID, "--expires", "2d", "-o", "json").out), &next); err != nil {
		t.Fatalf("reissue is not json: %v", err)
	}
	if next.Token == "" || next.Token == issued.Token {
		t.Fatalf("reissue gave token %q, want a new one", next.Token)
	}
	if life := next.ExpiresAt.Sub(next.CreatedAt); life != 48*time.Hour {
		t.Fatalf("the re-issued link lives %s, want the 48h --expires asked for", life)
	}

	c.mustRun("user", "invite", "revoke", next.ID)
	if listed := c.mustRun("user", "invite", "ls", "-o", "json").out; strings.Contains(listed, "ada@example.com") {
		t.Fatalf("a revoked invite is still listed: %s", listed)
	}
}

// A local database with server.public_url configured knows the address
// people open tix at, so the invite prints the absolute link, gives it as url
// in JSON beside path, and drops the hint to put an address in front.
func TestUserInviteWithAPublicAddressPrintsTheAbsoluteLink(t *testing.T) {
	c := newCLI(t)
	env := append(c.environ(), "TIX_SERVER_PUBLIC_URL=https://tix.z09.org")
	runWith := func(args ...string) result {
		var out, errb bytes.Buffer
		code := Run(args, strings.NewReader(""), &out, &errb, env, c.home)
		return result{code: code, out: out.String(), err: errb.String()}
	}
	got := runWith("user", "invite", "ada@example.com", "--role", "admin", "-o", "json")
	var issued core.IssuedInvite
	if err := json.Unmarshal([]byte(got.out), &issued); err != nil {
		t.Fatalf("user invite: exit %d, %v\n%s\n%s", got.code, err, got.out, got.err)
	}
	if want := "https://tix.z09.org/invite/" + issued.Token; issued.URL != want || issued.Path != "/invite/"+issued.Token {
		t.Fatalf("url %q, path %q; want url %q and the path kept", issued.URL, issued.Path, want)
	}
	if strings.Contains(got.err, "local database") {
		t.Fatalf("the local database hint was printed with a public address set: %q", got.err)
	}

	plain := c.mustRun("user", "invite", "bob@example.com", "-o", "json")
	if strings.Contains(plain.out, `"url"`) || !strings.Contains(plain.err, "local database") {
		t.Fatalf("without server.public_url: out %q, err %q; want no url and the hint", plain.out, plain.err)
	}
}

// TestUserInviteRefusesAMember is the conflict exit code.
func TestUserInviteRefusesAMember(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", firstPassword, "--handle", "ada")
	if got := c.run("user", "invite", "ada@example.com"); got.code != core.ExitConflict {
		t.Fatalf("inviting a member exited %d, want %d\n%s", got.code, core.ExitConflict, got.err)
	}
	if got := c.run("user", "invite", "bob@example.com", "--expires", "90d"); got.code != core.ExitUsage {
		t.Fatalf("a 90 day expiry exited %d, want %d\n%s", got.code, core.ExitUsage, got.err)
	}
}

// An account of another tenant is invited like anybody else, since its owner
// accepts with their own password. One with no password yet is the conflict.
func TestUserInviteReachesAnotherTenantsAccount(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", firstPassword, "--handle", "ada")
	c.mustRun("user", "invite", "bob@example.com")
	c.mustRun("tenant", "create", "acme", "Acme")

	c.mustRun("--tenant", "acme", "user", "invite", "ada@example.com")
	if got := c.run("--tenant", "acme", "user", "invite", "bob@example.com"); got.code != core.ExitConflict {
		t.Fatalf("inviting an account with no password exited %d, want %d\n%s", got.code, core.ExitConflict, got.err)
	}
}
