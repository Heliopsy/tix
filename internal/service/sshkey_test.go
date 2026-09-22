package service

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/heliopsy/tix/internal/core"
)

// pubKey is a valid ed25519 public key in authorized_keys form, without a
// comment. Tests build the spellings they need around it.
const pubKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMBtWpANdXlq6pEw/ZGWCcsF7GLyPc5SjZ6y7/i0wZom"

// otherKey is a second, different key, for asserting that two keys are two
// identities.
const otherKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJ9Z3Qw7Z8kQxV8qYqLQ0yq8ZqZqZqZqZqZqZqZqZqZq"

// fingerprintOf is what an ssh client prints for a key, computed the way a
// client computes it rather than the way the service does, so the two are
// compared rather than assumed equal.
func fingerprintOf(t *testing.T, authorized string) string {
	t.Helper()
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorized))
	if err != nil {
		t.Fatalf("parsing the fixture key: %v", err)
	}
	return ssh.FingerprintSHA256(k)
}

func TestEnrolSSHKeyRecordsTheKeyAndItsFingerprint(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ctx := authContext(admin)

	key, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey, Label: "laptop"})
	if err != nil {
		t.Fatalf("EnrolSSHKey: %v", err)
	}
	if got, want := key.Fingerprint, fingerprintOf(t, pubKey); got != want {
		t.Errorf("fingerprint = %q, want %q, which is what an ssh client prints", got, want)
	}
	if key.ActorID != admin.ID || key.TenantID != scope.TenantID {
		t.Errorf("key = %+v, want the caller's actor and tenant", key)
	}
	if key.Label != "laptop" {
		t.Errorf("label = %q, want it kept", key.Label)
	}
	if !key.Active() {
		t.Error("a freshly enrolled key is not active")
	}
}

// TestEnrolSSHKeyStoresTheKeyAloneNotWhatWasSubmitted is task 10.1.
//
// ssh.ParseAuthorizedKey accepts a comment and hands it back separately, so a
// service that stored the submitted bytes would keep it. What is stored has to
// be the key, because the stored value is shown in the web interface, travels
// in exports, and would be honoured by anything that ever wrote these rows to
// a real authorized_keys file.
func TestEnrolSSHKeyStoresTheKeyAloneNotWhatWasSubmitted(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	key, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey + " ada@laptop\n"})
	if err != nil {
		t.Fatalf("EnrolSSHKey: %v", err)
	}
	if strings.Contains(key.PublicKey, "ada@laptop") {
		t.Errorf("stored key %q kept the comment from the submission", key.PublicKey)
	}
	if key.PublicKey != pubKey {
		t.Errorf("stored key = %q, want the canonical form %q", key.PublicKey, pubKey)
	}
}

// TestEnrolSSHKeyRefusesOptionsRatherThanDroppingThem is task 10.1's other half.
//
// An authorized_keys line may carry options ahead of the key type. Accepting
// one and discarding the options would leave a submitter who meant to restrict
// the key believing a restriction was recorded when none was, which is worse
// than a refusal.
func TestEnrolSSHKeyRefusesOptionsRatherThanDroppingThem(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	submitted := `command="/bin/sh",no-pty ` + pubKey + " ada@laptop"
	_, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: submitted})
	if core.KindOf(err) != core.KindInvalid {
		t.Fatalf("err = %v, want it refused as invalid", err)
	}
	if !strings.Contains(err.Error(), "options are not supported") {
		t.Errorf("err = %q, want it to name options as the problem", err)
	}
	// The refusal must not carry the submission back: it is rendered by the
	// web interface, the command line and the API alike.
	for _, leaked := range []string{"command=", "/bin/sh", "no-pty"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("err = %q, want it not to echo %q from the submission", err, leaked)
		}
	}
	assertNoKeys(t, l, admin)
}

// TestEnrolSSHKeyTreatsOneKeyAsOneIdentity is task 10.2.
func TestEnrolSSHKeyTreatsOneKeyAsOneIdentity(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	if _, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey + " one@host"}); err != nil {
		t.Fatalf("first enrolment: %v", err)
	}
	for _, spelling := range []struct{ name, text string }{
		{"a different comment", pubKey + " two@elsewhere"},
		{"surrounding whitespace", "  " + pubKey + "  \n"},
		{"no comment at all", pubKey},
	} {
		t.Run(spelling.name, func(t *testing.T) {
			_, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: spelling.text})
			if core.KindOf(err) != core.KindConflict {
				t.Fatalf("err = %v, want a conflict: this is the same key", err)
			}
		})
	}
	keys := listKeys(t, l, admin)
	if len(keys) != 1 {
		t.Fatalf("got %d keys, want 1: one key is one identity however it was spelled", len(keys))
	}
}

// TestEnrolSSHKeyRefusesSubmissionsThatAreNotOneKey is task 10.3.
func TestEnrolSSHKeyRefusesSubmissionsThatAreNotOneKey(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	for _, tc := range []struct{ name, text, want string }{
		{"two keys", pubKey + "\n" + otherKey + "\n", "one key at a time"},
		{"a private key", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n", "private key"},
		{"empty", "   \n", "public key is required"},
		{"not a key at all", "hello world", "authorized_keys form"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: tc.text})
			if core.KindOf(err) != core.KindInvalid {
				t.Fatalf("err = %v, want it refused as invalid", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to mention %q", err, tc.want)
			}
		})
	}
	assertNoKeys(t, l, admin)
}

func TestRevokeSSHKeyStopsItWithoutHidingIt(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	key, err := l.EnrolSSHKey(ctx, core.EnrolSSHKeyInput{PublicKey: pubKey})
	if err != nil {
		t.Fatalf("EnrolSSHKey: %v", err)
	}
	if err := l.RevokeSSHKey(ctx, key.ID); err != nil {
		t.Fatalf("RevokeSSHKey: %v", err)
	}

	keys := listKeys(t, l, admin)
	if len(keys) != 1 {
		t.Fatalf("got %d keys, want the revoked one still listed", len(keys))
	}
	if keys[0].Active() {
		t.Error("the revoked key still reports itself active")
	}
	if keys[0].RevokedAt == nil {
		t.Error("the revoked key carries no revocation time")
	}
}

func TestRevokeSSHKeyRefusesAnIdentifierItDoesNotHold(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	if err := l.RevokeSSHKey(ctx, "01J000000000000000000A"); core.KindOf(err) != core.KindNotFound {
		t.Fatalf("err = %v, want not found", err)
	}
	if err := l.RevokeSSHKey(ctx, "  "); core.KindOf(err) != core.KindInvalid {
		t.Fatalf("err = %v, want invalid for an empty identifier", err)
	}
}

// TestEnrolSSHKeyIsNotAWayToReachAnotherActor is task 10.6.
//
// A key is a credential that speaks for an actor, so enrolling one against
// somebody else is the same authority as minting them a token. A caller who
// cannot do the latter must not be able to do the former, and neither may
// reach an actor that is not this tenant's.
func TestEnrolSSHKeyIsNotAWayToReachAnotherActor(t *testing.T) {
	t.Run("without the authority to administer credentials", func(t *testing.T) {
		l, _, _, admin := newLocal(t)
		// Everything a member plausibly holds, except credential administration.
		member := scopedActor(admin, core.ScopeTaskRead, core.ScopeTaskWrite)
		_, err := l.EnrolSSHKey(authContext(member), core.EnrolSSHKeyInput{PublicKey: pubKey})
		if core.KindOf(err) != core.KindForbidden {
			t.Fatalf("err = %v, want it forbidden without credential authority", err)
		}
		assertNoKeys(t, l, admin)
	})

	t.Run("naming an actor that is not this tenant's", func(t *testing.T) {
		l, _, _, admin := newLocal(t)
		_, err := l.EnrolSSHKey(authContext(admin), core.EnrolSSHKeyInput{
			ActorID:   "01J00000000000000000FF",
			PublicKey: pubKey,
		})
		if core.KindOf(err) != core.KindNotFound {
			t.Fatalf("err = %v, want not found: the actor is not reachable from here", err)
		}
		assertNoKeys(t, l, admin)
	})
}

func TestListSSHKeysIsRefusedWithoutCredentialAuthority(t *testing.T) {
	l, _, _, admin := newLocal(t)
	member := scopedActor(admin, core.ScopeTaskRead)
	if _, err := l.ListSSHKeys(authContext(member), ""); core.KindOf(err) != core.KindForbidden {
		t.Fatalf("err = %v, want forbidden", err)
	}
}

func listKeys(t *testing.T, l *Local, actor *core.Actor) []core.SSHKey {
	t.Helper()
	keys, err := l.ListSSHKeys(authContext(actor), "")
	if err != nil {
		t.Fatalf("ListSSHKeys: %v", err)
	}
	return keys
}

// assertNoKeys checks a refusal wrote nothing, which is the half of a refusal
// that a message alone does not prove.
func assertNoKeys(t *testing.T, l *Local, actor *core.Actor) {
	t.Helper()
	if keys := listKeys(t, l, actor); len(keys) != 0 {
		t.Fatalf("a refused enrolment left %d keys behind", len(keys))
	}
}
