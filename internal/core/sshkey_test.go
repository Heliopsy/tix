// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

const edKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample ada@laptop"

func TestEnrolSSHKeyInputValidate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   core.EnrolSSHKeyInput
		want string
	}{
		{"an ed25519 key", core.EnrolSSHKeyInput{PublicKey: edKey}, ""},
		{"an ecdsa key", core.EnrolSSHKeyInput{PublicKey: "ecdsa-sha2-nistp256 AAAAE2Vj"}, ""},
		{"a security key", core.EnrolSSHKeyInput{PublicKey: "sk-ssh-ed25519@openssh.com AAAAGnNr"}, ""},
		{"surrounding space", core.EnrolSSHKeyInput{PublicKey: "\n " + edKey + " \n"}, ""},
		{"all tenants for oneself", core.EnrolSSHKeyInput{PublicKey: edKey, AllTenants: true}, ""},
		{"nothing", core.EnrolSSHKeyInput{PublicKey: "  "}, "a public key is required"},
		{"a private key", core.EnrolSSHKeyInput{PublicKey: "-----BEGIN OPENSSH PRIVATE KEY-----\nb3Blbg=="},
			"that is a private key"},
		{"options ahead of the key", core.EnrolSSHKeyInput{PublicKey: `command="ls",no-pty ` + edKey},
			"authorized_keys options are not supported"},
		{"a file path", core.EnrolSSHKeyInput{PublicKey: "~/.ssh/id_ed25519.pub"},
			"a public key in authorized_keys form is required"},
		{"a long label", core.EnrolSSHKeyInput{PublicKey: edKey, Label: strings.Repeat("x", 201)},
			"label must be 200 characters or fewer"},
		{"all tenants for somebody else", core.EnrolSSHKeyInput{PublicKey: edKey, AllTenants: true, ActorID: "bob"},
			"takes no actor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.in.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil || core.KindOf(err) != core.KindInvalid || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an invalid error saying %q", err, tc.want)
			}
		})
	}
}

func TestEnrolSSHKeyInputNeverEchoesTheSubmission(t *testing.T) {
	t.Parallel()
	pasted := `from="<script>alert(1)</script>" ` + edKey
	err := core.EnrolSSHKeyInput{PublicKey: pasted}.Validate()
	if err == nil || strings.Contains(err.Error(), "<script>") {
		t.Fatalf("the refusal quoted the submission back: %v", err)
	}
}

func TestSSHKeyActiveUntilRevoked(t *testing.T) {
	t.Parallel()
	k := core.SSHKey{}
	if !k.Active() {
		t.Fatal("a key never revoked is not active")
	}
	now := time.Now()
	k.RevokedAt = &now
	if k.Active() {
		t.Fatal("a revoked key is still active")
	}
}

func TestSSHLoginNamespaceNamesTheAudience(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"":                   "tix-login@local",
		"   ":                "tix-login@local",
		"Tix.Example.COM":    "tix-login@tix.example.com",
		" tix.example.com  ": "tix-login@tix.example.com",
	} {
		if got := core.SSHLoginNamespace(in); got != want {
			t.Errorf("SSHLoginNamespace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSSHLoginInputValidate(t *testing.T) {
	t.Parallel()
	whole := core.SSHLoginInput{PublicKey: edKey, Signature: "sig", Nonce: "n"}
	if err := whole.Validate(); err != nil {
		t.Fatalf("a whole answer was refused: %v", err)
	}
	for want, in := range map[string]core.SSHLoginInput{
		"a public key is required":      {Signature: "sig", Nonce: "n"},
		"a signature is required":       {PublicKey: edKey, Nonce: "n"},
		"a challenge nonce is required": {PublicKey: edKey, Signature: "sig", Nonce: " "},
	} {
		if err := in.Validate(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want %q", err, want)
		}
	}
}
