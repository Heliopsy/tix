package sshd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/heliopsy/tix/internal/core"
	"golang.org/x/crypto/ssh"
)

// hostKeyPerm is the mode a host key is written with, and the mode an existing
// one must not exceed. A host key readable by anyone else on the machine lets
// them impersonate this listener.
const hostKeyPerm fs.FileMode = 0o600

// hostKey loads the persisted host key, generating one on first run.
func hostKey(path string) (ssh.Signer, error) {
	if path == "" {
		return nil, core.Invalid("a host key path is required")
	}
	pemBytes, err := os.ReadFile(path) // #nosec G304 -- the operator names this path
	switch {
	case err == nil:
		return parseHostKey(path, pemBytes)
	case errors.Is(err, fs.ErrNotExist):
		return generateHostKey(path)
	default:
		return nil, core.Internal("reading host key %q: %v", path, err)
	}
}

// parseHostKey decodes an existing key, refusing one others can read.
func parseHostKey(path string, pemBytes []byte) (ssh.Signer, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, core.Internal("inspecting host key %q: %v", path, err)
	}
	if mode := info.Mode().Perm(); mode&^hostKeyPerm != 0 {
		return nil, core.Invalid(
			"host key %q is mode %04o; it must not be readable by anyone but its owner, so chmod 600 it",
			path, mode)
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
