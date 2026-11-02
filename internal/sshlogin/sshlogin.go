// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sshlogin is the client half of signing in with an SSH key: finding
// the keys a person holds, signing the server's challenge with each in turn,
// and stopping at the first one the server accepts.
package sshlogin

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// DefaultKeyFiles are the private keys tried under ~/.ssh when no agent offers
// one that works, in the order OpenSSH itself prefers.
var DefaultKeyFiles = []string{"id_ed25519", "id_ecdsa", "id_rsa"}

// Candidate is one key that may be tried. Signer is called only when the key
// is about to be used, so an encrypted file asks for its passphrase then and
// not before.
type Candidate struct {
	Origin string
	Signer func() (ssh.Signer, error)
}

// Passphrase asks for the passphrase of an encrypted key file, without echo.
type Passphrase func(path string) ([]byte, error)

// FromFile is the single key at path.
func FromFile(path string, ask Passphrase) Candidate {
	return Candidate{Origin: path, Signer: func() (ssh.Signer, error) { return loadFile(path, ask) }}
}

// Discover lists the keys of an agent at socket, then the default key files
// under home that exist. The returned closer releases the agent connection.
func Discover(socket, home string, ask Passphrase) ([]Candidate, func() error, error) {
	closer := func() error { return nil }
	var out []Candidate
	if strings.TrimSpace(socket) != "" {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			return nil, closer, core.Precondition("reaching ssh-agent at %q: %v", socket, err)
		}
		closer = conn.Close
		signers, err := agent.NewClient(conn).Signers()
		if err != nil {
			_ = conn.Close()
			return nil, func() error { return nil }, core.Precondition("listing ssh-agent keys: %v", err)
		}
		for _, s := range signers {
			out = append(out, Candidate{Origin: "agent", Signer: func() (ssh.Signer, error) { return s, nil }})
		}
	}
	if home != "" {
		for _, name := range DefaultKeyFiles {
			path := filepath.Join(home, ".ssh", name)
			if _, err := os.Stat(path); err == nil {
				out = append(out, FromFile(path, ask))
			}
		}
	}
	if len(out) == 0 {
		return nil, closer, core.Precondition(
			"no ssh key found: ssh-agent holds none and there is no default key under ~/.ssh; pass --ssh-key PATH")
	}
	return out, closer, nil
}

func loadFile(path string, ask Passphrase) (ssh.Signer, error) {
	pem, err := os.ReadFile(path) // #nosec G304 -- the person named this key
	if err != nil {
		return nil, core.Invalid("reading ssh key %q: %v", path, err)
	}
	signer, err := ssh.ParsePrivateKey(pem)
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		if ask == nil {
			return nil, core.Precondition("ssh key %q is encrypted and there is no terminal to ask for its passphrase", path)
		}
		phrase, askErr := ask(path)
		if askErr != nil {
			return nil, askErr
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(pem, phrase)
		clear(phrase)
	}
	if err != nil {
		return nil, core.Invalid("ssh key %q is not a private key tix can use: %v", path, err)
	}
	return signer, nil
}

// Login asks for one challenge and tries each candidate with it until one is
// accepted. Only a key the server reports as not enrolled or revoked moves on
// to the next: any other refusal is about the challenge or the server, and the
// next key would meet it too.
//
// The namespace is computed from audience, the host this client was told to
// reach, and never adopted from the server: a server relaying another's
// challenge is caught here, before anything is signed.
func Login(ctx context.Context, svc core.Service, audience, tenant string, candidates []Candidate,
) (*core.LoginResult, error) {
	challenge, err := svc.SSHLoginChallenge(ctx)
	if err != nil {
		return nil, err
	}
	namespace := core.SSHLoginNamespace(audience)
	if challenge.Namespace != namespace {
		return nil, core.Precondition(
			"the server asked for a signature for %q, but this client is signing in to %q; check the server address",
			challenge.Namespace, namespace)
	}
	var refusals []string
	for _, c := range candidates {
		signer, err := c.Signer()
		if err != nil {
			if len(candidates) == 1 {
				return nil, err
			}
			refusals = append(refusals, c.Origin+": "+message(err))
			continue
		}
		signature, err := auth.SignSSH(signer, namespace, []byte(challenge.Nonce))
		if err != nil {
			return nil, err
		}
		result, err := svc.SSHLogin(ctx, core.SSHLoginInput{
			PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))),
			Signature: signature,
			Nonce:     challenge.Nonce,
			Tenant:    tenant,
		})
		if err == nil {
			return result, nil
		}
		if !tryNext(err) {
			return nil, err
		}
		refusals = append(refusals, c.Origin+": "+message(err))
	}
	if len(refusals) == 1 {
		return nil, core.Unauthenticated("%s", refusals[0])
	}
	return nil, core.Unauthenticated("no key was accepted:\n  %s", strings.Join(refusals, "\n  "))
}

func tryNext(err error) bool {
	var e *core.Error
	if !errors.As(err, &e) {
		return false
	}
	reason, _ := e.Details["reason"].(string)
	return reason == core.ReasonSSHKeyNotEnrolled || reason == core.ReasonSSHKeyRevoked
}

func message(err error) string {
	var e *core.Error
	if errors.As(err, &e) {
		return e.Message
	}
	return err.Error()
}
