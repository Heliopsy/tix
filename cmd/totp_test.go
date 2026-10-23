// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// totpKey is a throwaway sealing key. It is a fixture rather than a generated
// value so that a failure is reproducible.
const totpKey = "TIX_AUTH_TOTP_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

// tty is a standard input an invocation reads as a terminal.
//
// The reply is produced on the first read rather than supplied up front,
// because what a prompt asks for is usually derived from what the command has
// already printed: a TOTP code cannot be known before the secret it is
// computed from has been shown.
type tty struct {
	reply func() string
	buf   *strings.Reader
}

func (t *tty) IsTerminal() bool { return true }

func (t *tty) Read(p []byte) (int, error) {
	if t.buf == nil {
		t.buf = strings.NewReader(t.reply())
	}
	return t.buf.Read(p)
}

// runTTYEnv runs the command tree with a standard input that claims to be a
// terminal, answering each prompt with what reply makes of the diagnostics
// written so far. Every path that prompts also needs the sealing key, so the
// environment is not optional.
func (c *cli) runTTYEnv(extra []string, reply func(stderr string) string, args ...string) result {
	c.t.Helper()
	var out, errb bytes.Buffer
	in := &tty{reply: func() string { return reply(errb.String()) }}
	code := Run(args, in, &out, &errb, append(c.environ(), extra...), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

// secretPattern pulls the base32 secret out of the otpauth URI an enrolment
// prints, which is the only place a test can learn it from.
var secretPattern = regexp.MustCompile(`secret=([A-Z2-7]+)`)

// codeFrom computes the code an authenticator would show for the secret the
// given output carries.
func codeFrom(t *testing.T, printed string) string {
	t.Helper()
	m := secretPattern.FindStringSubmatch(printed)
	if m == nil {
		t.Fatalf("no otpauth secret in:\n%s", printed)
	}
	code, err := auth.TOTPCode(m[1], auth.TOTPStep(clock.New().Now()))
	if err != nil {
		t.Fatalf("generating a code: %v", err)
	}
	return code
}

// requireTOTPOnCLI turns on the tenant flag for the command line. There is no
// command for it yet, so the test reaches the column directly; the surface
// that sets it is a separate piece of the same change.
func (c *cli) requireTOTPOnCLI() {
	c.t.Helper()
	db, err := sql.Open("sqlite", c.dbPath())
	if err != nil {
		c.t.Fatalf("opening %s: %v", c.dbPath(), err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`UPDATE tenants SET require_totp_cli = 1`); err != nil {
		c.t.Fatalf("requiring a factor on the command line: %v", err)
	}
}

// enrolledUser creates a user, mints a token acting as them and enrols a
// second factor through the command line, returning the token and the secret.
func enrolledUser(t *testing.T, c *cli, email, password string) (token, secret string) {
	t.Helper()
	c.mustRun("user", "create", email, "--password", password, "--handle", "ada", "--role", "admin")
	token = actorToken(t, c, "ada")
	got := c.runTTYEnv([]string{totpKey, EnvToken + "=" + token},
		func(stderr string) string { return codeFrom(t, stderr) + "\n" },
		"user", "totp", "enrol", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("enrol exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	m := secretPattern.FindStringSubmatch(got.err)
	if m == nil {
		t.Fatalf("enrolment printed no otpauth secret:\n%s", got.err)
	}
	return token, m[1]
}

// actorToken issues a full-scope token acting as one actor.
func actorToken(t *testing.T, c *cli, handle string) string {
	t.Helper()
	return roleToken(t, c, handle, core.RoleAdmin)
}

// roleToken issues a token carrying exactly the scopes a role grants, which
// is what makes a refusal in a test a refusal about the scope rather than
// about a token minted too narrowly to prove anything.
func roleToken(t *testing.T, c *cli, handle string, role core.Role) string {
	t.Helper()
	args := []string{"token", "create", handle + "-" + string(role), "--actor", handle}
	for _, scope := range role.Scopes() {
		args = append(args, "--scope", string(scope))
	}
	out := c.mustRun(append(args, "-o", "json")...).out
	var issued struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(out), &issued); err != nil {
		t.Fatalf("token create is not json: %v\n%s", err, out)
	}
	if issued.Token == "" {
		t.Fatalf("token create returned no secret:\n%s", out)
	}
	return issued.Token
}

// codeAt computes the code for a known secret, steps away from now.
//
// An offset is needed rather than optional: a confirmed enrolment records the
// step it accepted and refuses anything at or below it, so a second code
// taken inside the same thirty seconds is correctly rejected. One step ahead
// is inside the acceptance window and above the mark.
func codeAt(t *testing.T, secret string, steps int64) string {
	t.Helper()
	code, err := auth.TOTPCode(secret, auth.TOTPStep(clock.New().Now())+steps)
	if err != nil {
		t.Fatalf("generating a code: %v", err)
	}
	return code
}

// mustRunEnv fails the test when an invocation with extra environment did not
// succeed.
func (c *cli) mustRunEnv(extra []string, args ...string) result {
	c.t.Helper()
	got := runEnv(c, extra, args...)
	if got.code != core.ExitOK {
		c.t.Fatalf("tix %s exited %d\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), got.code, got.out, got.err)
	}
	return got
}
