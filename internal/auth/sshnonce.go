// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// SSHNonceTTL bounds how long an SSH sign-in challenge stays redeemable. It
// covers an agent asking for confirmation, not a coffee break.
const SSHNonceTTL = 2 * time.Minute

// SSHNonce is a sign-in challenge. It is signed rather than stored, like the
// sign-in ticket: what it has to prove is that this server issued it for this
// tenant and host a moment ago, which a signature says as well as a row.
type SSHNonce struct {
	TenantID  string
	Audience  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// SSHNonceKey derives the key challenges are signed under from the configured
// sealing key, or draws a random one when none is configured. A random key
// lives as long as the process, so a deployment of several replicas without
// sticky routing needs the configured key, as it does for second factors.
func SSHNonceKey(encodedTOTPKey string) ([]byte, error) {
	if encodedTOTPKey != "" {
		key, err := ParseTOTPKey(encodedTOTPKey)
		if err != nil {
			return nil, err
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte("tix-ssh-login-nonce"))
		return mac.Sum(nil), nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, core.Internal("reading random bytes").Wrap(err)
	}
	return key, nil
}

// MintSSHNonce signs a challenge. The issue time is truncated to microseconds
// so that PostgreSQL's precision cannot make it compare unequal to itself.
func MintSSHNonce(key []byte, n SSHNonce) (string, SSHNonce, error) {
	if n.TenantID == "" || n.Audience == "" {
		return "", SSHNonce{}, core.Invalid("a sign-in challenge requires a tenant and an audience")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", SSHNonce{}, core.Internal("reading random bytes").Wrap(err)
	}
	n.IssuedAt = n.IssuedAt.UTC().Truncate(time.Microsecond)
	n.ExpiresAt = n.IssuedAt.Add(SSHNonceTTL)
	payload := strings.Join([]string{
		n.TenantID, n.Audience,
		strconv.FormatInt(n.IssuedAt.UnixMicro(), 10),
		base64.RawURLEncoding.EncodeToString(random),
	}, ticketSeparator)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return body + "." + base64.RawURLEncoding.EncodeToString(nonceSignature(key, body)), n, nil
}

// ParseSSHNonce verifies a challenge's signature and expiry. A challenge that
// was altered, forged or minted under another key is one answer; one that
// expired is another, because the person can do something about that.
func ParseSSHNonce(key []byte, nonce string, now time.Time) (SSHNonce, error) {
	body, sig, found := strings.Cut(strings.TrimSpace(nonce), ".")
	if !found {
		return SSHNonce{}, errNonce()
	}
	given, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(given, nonceSignature(key, body)) {
		return SSHNonce{}, errNonce()
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return SSHNonce{}, errNonce()
	}
	parts := strings.Split(string(raw), ticketSeparator)
	if len(parts) != 4 {
		return SSHNonce{}, errNonce()
	}
	micros, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return SSHNonce{}, errNonce()
	}
	out := SSHNonce{TenantID: parts[0], Audience: parts[1], IssuedAt: time.UnixMicro(micros).UTC()}
	out.ExpiresAt = out.IssuedAt.Add(SSHNonceTTL)
	if !out.ExpiresAt.After(now) {
		return SSHNonce{}, core.Unauthenticated("the login challenge has expired; start again")
	}
	return out, nil
}

func errNonce() error {
	return core.Unauthenticated("the login challenge is not valid for this server; start again")
}

func nonceSignature(key []byte, body string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("tix-ssh-login-nonce"))
	mac.Write([]byte(body))
	return mac.Sum(nil)
}
