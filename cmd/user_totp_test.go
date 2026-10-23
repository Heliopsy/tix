// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func decodeStatus(t *testing.T, out string) core.TOTPStatus {
	t.Helper()
	var status core.TOTPStatus
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("decoding a totp status from %q: %v", out, err)
	}
	return status
}

func decodeEnrolment(t *testing.T, out string) enrolment {
	t.Helper()
	var e enrolment
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		t.Fatalf("decoding an enrolment from %q: %v", out, err)
	}
	return e
}

// TestUserTOTPEnrolStatusCodesAndRemoveRoundTripThroughTheCLI drives the
// whole group over the real command tree.
func TestUserTOTPEnrolStatusCodesAndRemoveRoundTripThroughTheCLI(t *testing.T) {
	c := newCLI(t)
	token, secret := enrolledUser(t, c, "ada@example.com", "correct horse battery")
	env := []string{totpKey, EnvToken + "=" + token}

	status := decodeStatus(t, runEnv(c, env, "user", "totp", "status", "-o", "json").out)
	switch {
	case !status.Enrolled:
		t.Error("status reports no enrolment after enrolling")
	case status.Pending:
		t.Error("status reports the enrolment as pending after it was confirmed")
	case status.RecoveryCodesLeft != 10:
		t.Errorf("recovery codes left = %d, want 10", status.RecoveryCodesLeft)
	}

	replaced := decodeEnrolment(t, c.mustRunEnv(env, "user", "totp", "codes", "-o", "json").out)
	if len(replaced.RecoveryCodes) != 10 {
		t.Fatalf("codes returned %d codes, want 10", len(replaced.RecoveryCodes))
	}

	if got := c.mustRunEnv(env, "user", "totp", "rm", "-o", "json"); !strings.Contains(got.out, statusOK) {
		t.Fatalf("rm output = %q", got.out)
	}
	after := decodeStatus(t, runEnv(c, env, "user", "totp", "status", "-o", "json").out)
	if after.Enrolled || after.RecoveryCodesLeft != 0 {
		t.Fatalf("after rm: enrolled=%v codes=%d, want false and 0", after.Enrolled, after.RecoveryCodesLeft)
	}

	// The removed secret must not be what a fresh enrolment hands back, or a
	// phone nobody revoked still produces accepted codes.
	again := c.runTTYEnv(env, func(stderr string) string { return codeFrom(t, stderr) + "\n" },
		"user", "totp", "enrol", "-o", "json")
	if again.code != core.ExitOK {
		t.Fatalf("re-enrolling exited %d\n%s", again.code, again.err)
	}
	if strings.Contains(again.err, secret) {
		t.Fatal("re-enrolment handed back the secret that was just removed")
	}
}

// TestTOTPStatusNeverReportsTheSecret is the guard on the one field that must
// never leave the server. It asserts the absence in the rendered document
// rather than on the type, because a type can gain a tag.
func TestTOTPStatusNeverReportsTheSecret(t *testing.T) {
	c := newCLI(t)
	token, secret := enrolledUser(t, c, "ada@example.com", "correct horse battery")
	env := []string{totpKey, EnvToken + "=" + token}

	for _, format := range []string{"json", "yaml", "table"} {
		got := c.mustRunEnv(env, "user", "totp", "status", "-o", format)
		if strings.Contains(got.out, secret) || strings.Contains(got.err, secret) {
			t.Errorf("-o %s leaked the secret:\nstdout: %s\nstderr: %s", format, got.out, got.err)
		}
		if strings.Contains(got.out, "otpauth://") {
			t.Errorf("-o %s printed an otpauth uri", format)
		}
	}
}

// TestEnrolKeepsTheSecretOffStandardOutput holds the split the group is built
// on: the secret is drawn for the person on standard error, and the document
// a caller redirects into a file carries the recovery codes and nothing that
// could produce a code.
func TestEnrolKeepsTheSecretOffStandardOutput(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery",
		"--handle", "ada", "--role", string(core.RoleAdmin))
	env := []string{totpKey, EnvToken + "=" + actorToken(t, c, "ada")}

	got := c.runTTYEnv(env, func(stderr string) string { return codeFrom(t, stderr) + "\n" },
		"user", "totp", "enrol", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("enrol exited %d\n%s", got.code, got.err)
	}
	if !strings.Contains(got.err, "otpauth://") {
		t.Fatalf("enrol printed no otpauth uri on standard error:\n%s", got.err)
	}
	if strings.Contains(got.out, "otpauth://") || strings.Contains(got.out, "secret") {
		t.Fatalf("the secret reached standard output:\n%s", got.out)
	}
	e := decodeEnrolment(t, got.out)
	if !e.Enrolled || len(e.RecoveryCodes) != 10 {
		t.Fatalf("enrol rendered enrolled=%v with %d codes, want true and 10", e.Enrolled, len(e.RecoveryCodes))
	}
}

// TestEnrolDrawsAScannableCodeWhereTheTerminalCanShowOne. The symbol needs
// explicit colours to be legible on any background, so colour availability is
// what decides whether one is drawn; without it the uri still enrols.
func TestEnrolDrawsAScannableCodeWhereTheTerminalCanShowOne(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery",
		"--handle", "ada", "--role", string(core.RoleAdmin))
	env := []string{totpKey, EnvToken + "=" + actorToken(t, c, "ada")}
	reply := func(stderr string) string { return codeFrom(t, stderr) + "\n" }

	drawn := c.runTTYEnv(env, reply, "user", "totp", "enrol", "--color", "-o", "json")
	if drawn.code != core.ExitOK {
		t.Fatalf("enrol exited %d\n%s", drawn.code, drawn.err)
	}
	if !strings.Contains(drawn.err, "▀") {
		t.Fatalf("no scannable code was drawn with colour forced:\n%s", drawn.err)
	}

	c.mustRunEnv(env, "user", "totp", "rm")
	plain := c.runTTYEnv(env, reply, "user", "totp", "enrol", "--no-color", "-o", "json")
	if plain.code != core.ExitOK {
		t.Fatalf("enrol exited %d\n%s", plain.code, plain.err)
	}
	if strings.Contains(plain.err, "▀") {
		t.Fatalf("a code was drawn where colour is off:\n%s", plain.err)
	}
	if !strings.Contains(plain.err, "otpauth://") {
		t.Fatalf("the uri went missing along with the code:\n%s", plain.err)
	}
}

// TestEnrolWithoutASealingKeyIsRefused. There is no plaintext fallback, so
// the refusal has to say what is missing.
func TestEnrolWithoutASealingKeyIsRefused(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery",
		"--handle", "ada", "--role", string(core.RoleAdmin))

	got := c.runTTYEnv([]string{EnvToken + "=" + actorToken(t, c, "ada")},
		func(string) string { return "000000\n" }, "user", "totp", "enrol")
	if got.code != core.ExitPrecondtion {
		t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitPrecondtion, got.err)
	}
	if !strings.Contains(got.err, "TIX_AUTH_TOTP_KEY") {
		t.Fatalf("stderr = %q, want it to name the key", got.err)
	}
}

// TestEnrolWithoutATerminalNamesApiTokens. There is no flag carrying the
// code, so a script has to be sent somewhere it can actually go.
func TestEnrolWithoutATerminalNamesApiTokens(t *testing.T) {
	c := newCLI(t)
	c.mustRun("user", "create", "ada@example.com", "--password", "correct horse battery",
		"--handle", "ada", "--role", string(core.RoleAdmin))

	got := runEnv(c, []string{totpKey, EnvToken + "=" + actorToken(t, c, "ada")},
		"user", "totp", "enrol")
	if got.code != core.ExitPrecondtion {
		t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitPrecondtion, got.err)
	}
	if !strings.Contains(got.err, "token") {
		t.Fatalf("stderr = %q, want it to name API tokens", got.err)
	}
	for _, name := range []string{"code", "totp", "otp"} {
		root, _ := newRoot(c.environ(), c.home)
		enrol, _, err := root.Find([]string{"user", "totp", "enrol"})
		if err != nil {
			t.Fatalf("finding the command: %v", err)
		}
		if f := enrol.Flags().Lookup(name); f != nil {
			t.Errorf("tix user totp enrol grew a --%s flag", name)
		}
	}
}

// TestAdministrativeClearIsHeldByAdminAndNotByMember. Clearing somebody
// else's factor is the way back in after a lost device, and it is also a way
// past their second factor, so the scope matters as much as the command.
func TestAdministrativeClearIsHeldByAdminAndNotByMember(t *testing.T) {
	c := newCLI(t)
	_, _ = enrolledUser(t, c, "ada@example.com", "correct horse battery")
	adaID := c.mustRun("actor", "show", "ada", "-o", "json").out

	var ada core.Actor
	if err := json.Unmarshal([]byte(adaID), &ada); err != nil {
		t.Fatalf("decoding the actor: %v\n%s", err, adaID)
	}
	c.mustRun("user", "create", "bob@example.com", "--password", "correct horse battery",
		"--handle", "bob", "--role", string(core.RoleMember))
	c.mustRun("user", "create", "cid@example.com", "--password", "correct horse battery",
		"--handle", "cid", "--role", string(core.RoleAdmin))

	refused := runEnv(c, []string{EnvToken + "=" + roleToken(t, c, "bob", core.RoleMember)},
		"user", "totp", "clear", ada.ID)
	if refused.code != core.ExitPermission {
		t.Fatalf("a member clearing another user exited %d, want %d\n%s",
			refused.code, core.ExitPermission, refused.err)
	}

	allowed := runEnv(c, []string{EnvToken + "=" + actorToken(t, c, "cid")},
		"user", "totp", "clear", ada.ID, "-o", "json")
	if allowed.code != core.ExitOK {
		t.Fatalf("an admin clearing another user exited %d\n%s", allowed.code, allowed.err)
	}
	if !strings.Contains(allowed.out, statusOK) {
		t.Fatalf("clear output = %q", allowed.out)
	}
}
