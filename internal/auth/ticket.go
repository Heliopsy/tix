// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TicketTTL bounds how long the first step of a sign-in stays redeemable. It
// is short because the ticket exists only to carry a person from the password
// form to the code prompt in front of it.
const TicketTTL = 5 * time.Minute

// Ticket is the first step of a sign-in, carried into the second.
//
// It is signed and self-describing rather than a row in a table. A stored
// ticket would be a third credential table with its own expiry sweep, and the
// only thing it has to prove is that this password was accepted for this user
// on this surface a moment ago, which a signature says exactly as well.
//
// It carries no authority on its own: holding one gets the bearer as far as
// being asked for a code, and no further.
type Ticket struct {
	UserID    string
	TenantID  string
	Surface   core.Surface
	ExpiresAt time.Time
}

// ticketSeparator divides the payload fields. None of the fields may contain
// it: identifiers are ULIDs and a surface is one of three literals.
const ticketSeparator = "|"

// MintTicket signs a ticket under the sealing key.
//
// The key is the same one that seals secrets, derived through HMAC rather than
// used directly, so a signature can never be mistaken for ciphertext. A tenant
// that requires a factor always has a key configured, because enrolment is
// refused without one, so there is no path that needs a ticket and lacks a key.
func MintTicket(encoded string, t Ticket) (string, error) {
	key, err := ParseTOTPKey(encoded)
	if err != nil {
		return "", err
	}
	switch {
	case t.UserID == "":
		return "", core.Invalid("a sign-in ticket requires a user")
	case t.TenantID == "":
		return "", core.Invalid("a sign-in ticket requires a tenant")
	case !t.Surface.Valid():
		return "", core.Invalid("a sign-in ticket requires a known surface")
	}
	payload := strings.Join([]string{
		t.UserID, t.TenantID, string(t.Surface), strconv.FormatInt(t.ExpiresAt.UTC().Unix(), 10),
	}, ticketSeparator)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return body + "." + base64.RawURLEncoding.EncodeToString(ticketSignature(key, body)), nil
}

// ParseTicket verifies a ticket's signature and expiry and returns what it
// says. Every failure answers the same way: a ticket is an internal step of a
// sign-in, and telling a caller which part of it was wrong tells an attacker
// how far a forgery got.
func ParseTicket(encoded, ticket string, now time.Time) (Ticket, error) {
	key, err := ParseTOTPKey(encoded)
	if err != nil {
		return Ticket{}, err
	}
	body, sig, found := strings.Cut(ticket, ".")
	if !found {
		return Ticket{}, errTicket()
	}
	given, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(given, ticketSignature(key, body)) {
		return Ticket{}, errTicket()
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Ticket{}, errTicket()
	}
	parts := strings.Split(string(raw), ticketSeparator)
	if len(parts) != 4 {
		return Ticket{}, errTicket()
	}
	unix, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Ticket{}, errTicket()
	}
	out := Ticket{
		UserID:    parts[0],
		TenantID:  parts[1],
		Surface:   core.Surface(parts[2]),
		ExpiresAt: time.Unix(unix, 0).UTC(),
	}
	if !out.Surface.Valid() || !out.ExpiresAt.After(now) {
		return Ticket{}, errTicket()
	}
	return out, nil
}

// errTicket is the single answer a bad ticket gives.
func errTicket() error {
	return core.Unauthenticated("this sign-in has expired; start again")
}

func ticketSignature(key []byte, body string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("tix-login-ticket"))
	mac.Write([]byte(body))
	return mac.Sum(nil)
}
