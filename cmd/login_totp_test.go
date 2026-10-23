// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestLoginPromptsForTheCodeAfterThePassword is the second step of a sign-in
// on the command line: the password is taken first, the code is asked for on
// the terminal, and a session exists only after both.
func TestLoginPromptsForTheCodeAfterThePassword(t *testing.T) {
	c := newCLI(t)
	_, secret := enrolledUser(t, c, "ada@example.com", "correct horse battery")

	got := c.runTTYEnv([]string{totpKey}, func(string) string { return codeAt(t, secret, 1) + "\n" },
		"login", "--email", "ada@example.com", "--password", "correct horse battery", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("login exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	var session core.Session
	if err := json.Unmarshal([]byte(got.out), &session); err != nil {
		t.Fatalf("login is not json: %v\n%s", err, got.out)
	}
	if session.Token == "" {
		t.Fatalf("login minted no session:\n%s", got.out)
	}
	if !strings.Contains(got.err, "code") {
		t.Fatalf("stderr = %q, want a prompt for the code", got.err)
	}
	if strings.Contains(got.out, "code") {
		t.Fatalf("the prompt reached standard output, which carries the session: %q", got.out)
	}
}

// TestLoginRefusesAWrongCodeAfterACorrectPassword keeps the second step load
// bearing. A password alone must not produce a session.
func TestLoginRefusesAWrongCodeAfterACorrectPassword(t *testing.T) {
	c := newCLI(t)
	enrolledUser(t, c, "ada@example.com", "correct horse battery")

	got := c.runTTYEnv([]string{totpKey}, func(string) string { return "000000\n" },
		"login", "--email", "ada@example.com", "--password", "correct horse battery", "-o", "json")
	if got.code != core.ExitPermission {
		t.Fatalf("login exited %d, want %d\n%s", got.code, core.ExitPermission, got.err)
	}
	if strings.Contains(got.out, "token") {
		t.Fatalf("a session reached standard output on a refused code: %q", got.out)
	}
}

// TestNoFlagOrVariableCarriesTheCode holds the rule the spec states as a
// requirement in its own right: a live second factor must not be reachable
// from shell history or the process table, so the only way in is the prompt.
func TestNoFlagOrVariableCarriesTheCode(t *testing.T) {
	c := newCLI(t)
	root, _ := newRoot(c.environ(), c.home)
	login, _, err := root.Find([]string{"login"})
	if err != nil {
		t.Fatalf("finding login: %v", err)
	}
	for _, name := range []string{"code", "totp", "otp", "factor", "token"} {
		if f := login.Flags().Lookup(name); f != nil {
			t.Errorf("tix login grew a --%s flag", name)
		}
	}
	if !strings.Contains(login.Long, "token") {
		t.Errorf("tix login --help does not point at API tokens for unattended use:\n%s", login.Long)
	}

	rejected := c.run("login", "--email", "ada@example.com", "--password", "x", "--code", "123456")
	if rejected.code != core.ExitUsage {
		t.Fatalf("--code exited %d, want %d\n%s", rejected.code, core.ExitUsage, rejected.err)
	}
}

// TestLoginWithoutATerminalNamesApiTokens is the unattended path. It must
// fail saying where to go rather than block on a prompt nobody can answer.
func TestLoginWithoutATerminalNamesApiTokens(t *testing.T) {
	c := newCLI(t)
	enrolledUser(t, c, "ada@example.com", "correct horse battery")

	got := runEnv(c, []string{totpKey},
		"login", "--email", "ada@example.com", "--password", "correct horse battery")
	if got.code == core.ExitOK {
		t.Fatalf("login succeeded without a terminal:\n%s", got.out)
	}
	if !strings.Contains(got.err, "token") {
		t.Fatalf("stderr = %q, want it to name API tokens", got.err)
	}
}

// TestLoginEnrolsWhenTheTenantRequiresAFactorAndTheUserHasNone is the rule
// that keeps a tenant from locking out the administrator who turned the
// requirement on. The sign-in walks through enrolment and completes.
func TestLoginEnrolsWhenTheTenantRequiresAFactorAndTheUserHasNone(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery",
		"--handle", "ada", "--role", "admin")
	c.requireTOTPOnCLI()

	got := c.runTTYEnv([]string{totpKey}, func(stderr string) string { return codeFrom(t, stderr) + "\n" },
		"login", "--email", "ada@example.com", "--password", "correct horse battery", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("login exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	var session core.Session
	if err := json.Unmarshal([]byte(got.out), &session); err != nil {
		t.Fatalf("login is not json: %v\n%s", err, got.out)
	}
	if session.Token == "" {
		t.Fatalf("login minted no session:\n%s", got.out)
	}
	if !strings.Contains(got.err, "otpauth://") {
		t.Fatalf("enrolment printed no otpauth uri:\n%s", got.err)
	}
	if n := strings.Count(got.err, recoveryCodeHeading); n != 1 {
		t.Fatalf("recovery codes were shown %d times, want exactly once:\n%s", n, got.err)
	}
}
