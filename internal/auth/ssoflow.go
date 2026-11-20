// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// SSOFlowTTL bounds how long a sign-in through a provider may take, from the
// redirect to the provider to the code coming back.
const SSOFlowTTL = 10 * time.Minute

// SSOFlowCookieName is the cookie a browser carries its flow in.
const SSOFlowCookieName = "tix_sso_flow"

// SSOFlow is a sign-in through a provider between its two steps.
//
// It is sealed rather than stored, like the sign-in ticket: an abandoned
// sign-in leaves no row, and any replica holding the same key completes a
// flow another began. It is encrypted, not only signed, because it carries
// the PKCE verifier, which must not be readable from a URL or a log.
type SSOFlow struct {
	TenantID    string       `json:"t"`
	ProviderID  string       `json:"p"`
	State       string       `json:"s"`
	Nonce       string       `json:"n"`
	Verifier    string       `json:"v"`
	RedirectURI string       `json:"r"`
	Surface     core.Surface `json:"f"`
	Remember    bool         `json:"m,omitempty"`
	Next        string       `json:"x,omitempty"`
	// App is a native app's part of the sign-in, when an app started it.
	App       *AppSignIn `json:"a,omitempty"`
	ExpiresAt time.Time  `json:"e"`
}

// NewSSOSecret returns a random URL-safe value for a state, nonce or PKCE
// verifier: 32 bytes, so 43 characters, inside the verifier's 43 to 128.
func NewSSOSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", core.Internal("reading random bytes").Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ssoFlowAEAD derives the flow's cipher from the sealing key, so a sealed
// flow can never be opened as a sealed secret or the other way round.
func ssoFlowAEAD(encoded string) (cipher.AEAD, error) {
	key, err := ParseTOTPKey(encoded)
	if err != nil {
		return nil, core.Precondition("single sign-on needs the server's sealing key, auth.totp_key, which is not set")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("tix-sso-flow"))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, core.Internal("building the sso flow cipher").Wrap(err)
	}
	return cipher.NewGCM(block)
}

// SealSSOFlow encrypts a flow under a key derived from the sealing key.
func SealSSOFlow(encoded string, f SSOFlow) (string, error) {
	aead, err := ssoFlowAEAD(encoded)
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(f)
	if err != nil {
		return "", core.Internal("encoding the sso flow").Wrap(err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", core.Internal("reading random bytes").Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, plain, []byte("tix-sso-flow"))), nil
}

// OpenSSOFlow decrypts a flow and checks its expiry. Every failure is one
// answer, as a bad ticket's is.
func OpenSSOFlow(encoded, sealed string, now time.Time) (SSOFlow, error) {
	f, err := PeekSSOFlow(encoded, sealed)
	if err != nil {
		return SSOFlow{}, err
	}
	if !f.ExpiresAt.After(now) {
		return SSOFlow{}, errSSOFlow()
	}
	return f, nil
}

// PeekSSOFlow decrypts a flow without checking its expiry. A flow that
// opens was sealed here, so an expired one still says truthfully which app
// to tell that it expired.
func PeekSSOFlow(encoded, sealed string) (SSOFlow, error) {
	aead, err := ssoFlowAEAD(encoded)
	if err != nil {
		return SSOFlow{}, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil || len(raw) < aead.NonceSize() {
		return SSOFlow{}, errSSOFlow()
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte("tix-sso-flow"))
	if err != nil {
		return SSOFlow{}, errSSOFlow()
	}
	var f SSOFlow
	if err := json.Unmarshal(plain, &f); err != nil || !f.Surface.Valid() {
		return SSOFlow{}, errSSOFlow()
	}
	return f, nil
}

func errSSOFlow() error {
	return core.Unauthenticated("this single sign-on has expired or was not started here; start again")
}
