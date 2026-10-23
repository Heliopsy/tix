// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"

	"github.com/heliopsy/tix/internal/core"
)

// TOTPKeyLength is the key size the sealer takes, in bytes.
const TOTPKeyLength = 32

// Sealer encrypts a TOTP secret at rest.
//
// A password is verified by hashing the input and comparing, so the stored
// form can be one-way. A TOTP secret has to be fed into HMAC on every
// verification, so it has to be recoverable and hashing is not available.
// Sealing is what keeps a copy of the database from yielding working second
// factors for every enrolled user.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer builds a sealer from a base64 key of exactly TOTPKeyLength bytes.
func NewSealer(encoded string) (*Sealer, error) {
	key, err := ParseTOTPKey(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, core.Internal("building the totp cipher").Wrap(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, core.Internal("building the totp cipher").Wrap(err)
	}
	return &Sealer{aead: aead}, nil
}

// ParseTOTPKey decodes a configured key and checks its length, so a malformed
// value fails at start-up rather than at the first enrolment.
func ParseTOTPKey(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, core.Invalid("no totp key is configured")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, core.Invalid("the totp key is not valid base64")
	}
	if len(key) != TOTPKeyLength {
		return nil, core.Invalid("the totp key must decode to %d bytes, not %d", TOTPKeyLength, len(key))
	}
	return key, nil
}

// NewTOTPKey mints a key an operator can paste into configuration.
func NewTOTPKey() (string, error) {
	key := make([]byte, TOTPKeyLength)
	if _, err := rand.Read(key); err != nil {
		return "", core.Internal("reading random bytes").Wrap(err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// Seal encrypts secret for one user, returning the ciphertext and the nonce,
// both base64.
//
// The user's identifier is the additional authenticated data, so a ciphertext
// moved into another user's row fails to open rather than enrolling the
// attacker's phone against that account.
func (s *Sealer) Seal(userID, secret string) (ciphertext, nonce string, err error) {
	if s == nil {
		return "", "", core.Invalid("no totp key is configured")
	}
	if userID == "" {
		return "", "", core.Invalid("sealing a totp secret needs the user it belongs to")
	}
	raw := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(raw); err != nil {
		return "", "", core.Internal("reading random bytes").Wrap(err)
	}
	sealed := s.aead.Seal(nil, raw, []byte(secret), []byte(userID))
	return base64.StdEncoding.EncodeToString(sealed), base64.StdEncoding.EncodeToString(raw), nil
}

// Open decrypts a sealed secret for one user. A ciphertext that belongs to
// another user does not open here.
func (s *Sealer) Open(userID, ciphertext, nonce string) (string, error) {
	if s == nil {
		return "", core.Invalid("no totp key is configured")
	}
	sealed, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", core.Internal("the stored totp secret is not valid base64")
	}
	raw, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return "", core.Internal("the stored totp nonce is not valid base64")
	}
	if len(raw) != s.aead.NonceSize() {
		return "", core.Internal("the stored totp nonce is %d bytes, want %d", len(raw), s.aead.NonceSize())
	}
	out, err := s.aead.Open(nil, raw, sealed, []byte(userID))
	if err != nil {
		return "", core.Internal("the stored totp secret does not open for this user")
	}
	return string(out), nil
}
