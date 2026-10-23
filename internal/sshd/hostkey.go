// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/heliopsy/tix/internal/core"
	"golang.org/x/crypto/ssh"
)

// hostKeyPerm is the mode a host key is written with, and the mode an existing
// one is narrowed back to. A host key readable by anyone else on the machine
// lets them impersonate this listener.
const hostKeyPerm fs.FileMode = 0o600

// chmodHostKey is os.Chmod behind a seam, because the refusal below fires only
// when the chmod fails, and making that happen for real needs a file this
// process does not own. Tests replace it; nothing else does.
var chmodHostKey = os.Chmod

// hostKey loads the persisted host key, generating one on first run.
func hostKey(path string, log *slog.Logger) (ssh.Signer, error) {
	if path == "" {
		return nil, core.Invalid("a host key path is required")
	}
	pemBytes, err := os.ReadFile(path) // #nosec G304 -- the operator names this path
	switch {
	case err == nil:
		return parseHostKey(path, pemBytes, log)
	case errors.Is(err, fs.ErrNotExist):
		return generateHostKey(path)
	default:
		return nil, core.Internal("reading host key %q: %v", path, err)
	}
}

// parseHostKey decodes an existing key, narrowing one others can read.
//
// This used to refuse outright, which turned a recoverable state into a
// listener that would not start. The common way to reach that state is not an
// operator being careless: Kubernetes with an fsGroup and the default
// fsGroupChangePolicy of Always adds group-rw to every file on the volume at
// mount time, so the 0600 this process wrote itself comes back as 0660 on the
// next mount. That makes the failure look like an upgrade regression, because
// the first restart after the key exists is usually a rollout, and it strands
// anyone following the documented Kubernetes path with ssh enabled.
//
// The process owns the file, so it can simply put the mode back and say so. A
// refusal protected nothing the narrowing does not: whoever could read the key
// while it was wide has already read it, and refusing to start neither unreads
// it nor rotates it. What is not tolerated is a mode this process cannot fix,
// which means the file is not ours and the operator has to look.
func parseHostKey(path string, pemBytes []byte, log *slog.Logger) (ssh.Signer, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, core.Internal("inspecting host key %q: %v", path, err)
	}
	if found := info.Mode().Perm(); found&^hostKeyPerm != 0 {
		if err := chmodHostKey(path, hostKeyPerm); err != nil {
			return nil, core.Invalid(
				"host key %q is mode %04o and could not be narrowed to %04o (%v); "+
					"it must not be readable by anyone but its owner, so chmod 600 it",
				path, found, hostKeyPerm, err)
		}
		// Read the mode back rather than trusting the chmod. A filesystem that
		// cannot represent the mode reports no error and keeps the old one, and
		// silently serving a readable key is the one outcome worth refusing.
		again, err := os.Stat(path)
		if err != nil {
			return nil, core.Internal("inspecting host key %q after narrowing it: %v", path, err)
		}
		if now := again.Mode().Perm(); now&^hostKeyPerm != 0 {
			return nil, core.Invalid(
				"host key %q is mode %04o and stayed that way after chmod %04o; "+
					"it must not be readable by anyone but its owner",
				path, now, hostKeyPerm)
		}
		log.Warn("host key was wider than its owner and has been narrowed",
			"path", path, "found", fmt.Sprintf("%04o", found), "now", fmt.Sprintf("%04o", hostKeyPerm))
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, core.Invalid("parsing host key %q: %v", path, err)
	}
	return signer, nil
}

// generateHostKey writes a fresh ed25519 key and returns its signer.
func generateHostKey(path string) (ssh.Signer, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, core.Internal("creating host key directory %q: %v", dir, err)
		}
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, core.Internal("generating host key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, core.Internal("encoding host key: %v", err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), hostKeyPerm); err != nil {
		return nil, core.Internal("writing host key %q: %v", path, err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, core.Internal("loading generated host key: %v", err)
	}
	return signer, nil
}
