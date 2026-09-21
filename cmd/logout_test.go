package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// loginAs creates a credentialed user and returns the session it logs in with.
func loginAs(t *testing.T, c *cli, email, password string) string {
	t.Helper()
	c.mustRun("user", "create", email, "--password", password)
	out := c.mustRun("login", "--email", email, "--password", password, "-o", "json").out
	var session core.Session
	if err := json.Unmarshal([]byte(out), &session); err != nil {
		t.Fatalf("login is not json: %v\n%s", err, out)
	}
	if session.Token == "" {
		t.Fatalf("login returned no token:\n%s", out)
	}
	return session.Token
}

// The session tix login issued can be ended, once. Ending it twice reports
// that there is no session left to end.
func TestLogoutEndsTheSessionExactlyOnce(t *testing.T) {
	c := newCLI(t)
	token := loginAs(t, c, "ada@example.com", "correct horse battery")

	got := c.mustRun("logout", "--session", token, "-o", "json")
	if !strings.Contains(got.out, statusOK) {
		t.Fatalf("logout output = %q", got.out)
	}

	again := c.run("logout", "--session", token)
	if again.code != core.ExitPermission {
		t.Fatalf("second logout exit = %d, want %d\n%s", again.code, core.ExitPermission, again.err)
	}
	if !strings.Contains(again.err, "no session to end") {
		t.Fatalf("stderr = %q", again.err)
	}
}

// The token may come from the flag, from standard input, or from the
// environment, and its absence is a usage error rather than a failed call.
func TestLogoutReadsTheSessionFromEverySource(t *testing.T) {
	t.Run("standard input", func(t *testing.T) {
		c := newCLI(t)
		token := loginAs(t, c, "ada@example.com", "correct horse battery")
		got := c.runIn(token+"\n", "logout", "--session", "-", "-o", "json")
		if got.code != core.ExitOK {
			t.Fatalf("exit = %d\n%s", got.code, got.err)
		}
	})

	t.Run("environment", func(t *testing.T) {
		c := newCLI(t)
		token := loginAs(t, c, "ada@example.com", "correct horse battery")
		got := runEnv(c, []string{EnvSession + "=" + token}, "logout", "-o", "json")
		if got.code != core.ExitOK {
			t.Fatalf("exit = %d\n%s", got.code, got.err)
		}
	})

	t.Run("nothing at all", func(t *testing.T) {
		c := newCLI(t)
		got := c.run("logout")
		if got.code != core.ExitUsage {
			t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitUsage, got.err)
		}
		if !strings.Contains(got.err, EnvSession) {
			t.Fatalf("stderr = %q, want it to name %s", got.err, EnvSession)
		}
	})

	t.Run("a token that names no session", func(t *testing.T) {
		c := newCLI(t)
		got := c.run("logout", "--session", wellFormedToken)
		if got.code != core.ExitPermission {
			t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitPermission, got.err)
		}
	})

	t.Run("a positional argument", func(t *testing.T) {
		c := newCLI(t)
		got := c.run("logout", wellFormedToken)
		if got.code != core.ExitUsage {
			t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitUsage, got.err)
		}
	})
}
