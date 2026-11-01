// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// CalendarTokenPrefix marks a calendar feed secret. It is not a TokenPrefix
// value, so a feed secret pasted into an Authorization header authenticates
// nothing, and an API token pasted into a feed URL reads nothing.
const CalendarTokenPrefix = "tix_cal_"

// MintCalendarToken returns a fresh feed secret and the hash to store.
func MintCalendarToken() (value, hash string, err error) {
	secret, err := newSecret()
	if err != nil {
		return "", "", err
	}
	value = CalendarTokenPrefix + secret
	return value, HashToken(value), nil
}

// ParseCalendarToken checks a presented feed secret's shape. Every malformed
// value is the same not found a revoked one is.
func ParseCalendarToken(value string) error {
	secret, ok := strings.CutPrefix(value, CalendarTokenPrefix)
	if !ok || len(secret) != tokenEncoding.EncodedLen(tokenBytes) || strings.ToLower(secret) != secret {
		return errNoFeed()
	}
	if _, err := tokenEncoding.DecodeString(strings.ToUpper(secret)); err != nil {
		return errNoFeed()
	}
	return nil
}

// errNoFeed is the one answer to any feed that cannot be read.
func errNoFeed() error { return core.NotFound("calendar feed not found") }
