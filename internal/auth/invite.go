// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// InviteTokenPrefix marks an invite secret. Like the calendar prefix it is not
// a TokenPrefix value, so an invite secret pasted into an Authorization header
// authenticates nothing.
const InviteTokenPrefix = "tix_inv_"

// MintInviteToken returns a fresh invite secret and the hash to store.
func MintInviteToken() (value, hash string, err error) {
	secret, err := newSecret()
	if err != nil {
		return "", "", err
	}
	value = InviteTokenPrefix + secret
	return value, HashToken(value), nil
}

// ParseInviteToken checks a presented invite secret's shape. Every malformed
// value is the same not found an unknown, used or expired one is.
func ParseInviteToken(value string) error {
	secret, ok := strings.CutPrefix(value, InviteTokenPrefix)
	if !ok || len(secret) != tokenEncoding.EncodedLen(tokenBytes) || strings.ToLower(secret) != secret {
		return ErrNoInvite()
	}
	if _, err := tokenEncoding.DecodeString(strings.ToUpper(secret)); err != nil {
		return ErrNoInvite()
	}
	return nil
}

// ErrNoInvite is the one answer to any invite link that cannot be used.
func ErrNoInvite() error {
	return core.NotFound("this invite link is not valid: it may have expired, been used or been revoked; " +
		"ask whoever invited you for a new one")
}
