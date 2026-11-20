// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// SSOHandoffTTL bounds how long the code an app sign-in hands the app may
// wait to be exchanged.
const SSOHandoffTTL = 2 * time.Minute

// AppSignIn is a native app's part of a sign-in through a provider: where the
// browser hands the result back, the PKCE challenge the code is bound to and
// the state the app gets back unchanged. It travels sealed in the flow and
// signed in the second factor's ticket, never as a plain parameter.
type AppSignIn struct {
	Redirect   string `json:"r"`
	Challenge  string `json:"c"`
	State      string `json:"s,omitempty"`
	ProviderID string `json:"p"`
}

// Answer is the app's redirect carrying the query: a code, or an error,
// with the app's state.
func (a AppSignIn) Answer(q url.Values) string {
	q.Set("state", a.State)
	sep := "?"
	if strings.Contains(a.Redirect, "?") {
		sep = "&"
	}
	return a.Redirect + sep + q.Encode()
}

// Refusal is the app's redirect carrying an error reason.
func (a AppSignIn) Refusal(reason string) string {
	return a.Answer(url.Values{"error": {reason}})
}

// PKCEChallenge is the S256 challenge of a PKCE verifier (RFC 7636).
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// VerifyPKCE reports, in constant time, whether verifier answers the S256
// challenge.
func VerifyPKCE(challenge, verifier string) bool {
	return subtle.ConstantTimeCompare([]byte(PKCEChallenge(verifier)), []byte(challenge)) == 1
}

// ValidChallenge reports whether c is an S256 challenge: the base64url of a
// SHA-256 digest, without padding.
func ValidChallenge(c string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	return err == nil && len(raw) == sha256.Size
}

// NewHandoffCode returns a one-time code for an app sign-in: 32 random
// bytes, base64url.
func NewHandoffCode() (string, error) { return NewSSOSecret() }

// encodeApp renders an app sign-in as a ticket field.
func encodeApp(a *AppSignIn) (string, error) {
	if a == nil {
		return "", nil
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return "", core.Internal("encoding the app sign-in").Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeApp reads a ticket field encodeApp wrote.
func decodeApp(field string) (*AppSignIn, bool) {
	if field == "" {
		return nil, true
	}
	raw, err := base64.RawURLEncoding.DecodeString(field)
	if err != nil {
		return nil, false
	}
	var a AppSignIn
	if json.Unmarshal(raw, &a) != nil || a.Redirect == "" || a.Challenge == "" {
		return nil, false
	}
	return &a, true
}
