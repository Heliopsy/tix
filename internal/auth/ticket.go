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
	// Method is how the first step was passed, carried so the session the
	// second step mints is recorded as what it was. Empty reads as password.
	Method string
	// App is the native app a sign-in through a provider hands its result
	// to, when an app started it.
	App *AppSignIn
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
	return mintTicket(encoded, ticketLabel, t)
}

// MintHandoffTicket signs the ticket an app sign-in that completed by
// enrolling carries to the Open the app button, which mints the app's code.
// It is signed apart from a sign-in ticket, so neither passes for the other.
func MintHandoffTicket(encoded string, t Ticket) (string, error) {
	if t.App == nil {
		return "", core.Invalid("a handoff ticket requires an app sign-in")
	}
	return mintTicket(encoded, handoffLabel, t)
}

// Labels the two kinds of ticket are signed under.
const (
	ticketLabel  = "tix-login-ticket"
	handoffLabel = "tix-app-handoff"
)

func mintTicket(encoded, label string, t Ticket) (string, error) {
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
	fields := []string{
		t.UserID, t.TenantID, string(t.Surface), strconv.FormatInt(t.ExpiresAt.UTC().Unix(), 10), t.Method,
	}
	if t.App != nil {
		app, err := encodeApp(t.App)
		if err != nil {
			return "", err
		}
		fields = append(fields, app)
	}
	payload := strings.Join(fields, ticketSeparator)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return body + "." + base64.RawURLEncoding.EncodeToString(ticketSignature(key, label, body)), nil
}

// ParseTicket verifies a ticket's signature and expiry and returns what it
// says. Every failure answers the same way: a ticket is an internal step of a
// sign-in, and telling a caller which part of it was wrong tells an attacker
// how far a forgery got.
func ParseTicket(encoded, ticket string, now time.Time) (Ticket, error) {
	out, err := readTicket(encoded, ticketLabel, ticket)
	if err != nil || !out.ExpiresAt.After(now) {
		return Ticket{}, orTicket(err)
	}
	return out, nil
}

// PeekTicket reads a sign-in ticket whose signature holds, expired or not,
// so a refusal can still name the app the sign-in belongs to.
func PeekTicket(encoded, ticket string) (Ticket, error) {
	return readTicket(encoded, ticketLabel, ticket)
}

// ParseHandoffTicket verifies a ticket MintHandoffTicket signed. An expired
// one answers what it says beside the error, so the refusal can go back to
// the app.
func ParseHandoffTicket(encoded, ticket string, now time.Time) (Ticket, error) {
	out, err := readTicket(encoded, handoffLabel, ticket)
	if err != nil || out.App == nil {
		return Ticket{}, orTicket(err)
	}
	if !out.ExpiresAt.After(now) {
		return out, errTicket()
	}
	return out, nil
}

// orTicket is err, or the bad ticket answer when there is none.
func orTicket(err error) error {
	if err != nil {
		return err
	}
	return errTicket()
}

// readTicket verifies a ticket's signature under label and decodes it,
// leaving its expiry to the caller.
func readTicket(encoded, label, ticket string) (Ticket, error) {
	key, err := ParseTOTPKey(encoded)
	if err != nil {
		return Ticket{}, err
	}
	body, sig, found := strings.Cut(ticket, ".")
	if !found {
		return Ticket{}, errTicket()
	}
	given, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(given, ticketSignature(key, label, body)) {
		return Ticket{}, errTicket()
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Ticket{}, errTicket()
	}
	parts := strings.Split(string(raw), ticketSeparator)
	if len(parts) < 4 || len(parts) > 6 {
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
	if len(parts) >= 5 {
		out.Method = parts[4]
	}
	if len(parts) == 6 {
		app, ok := decodeApp(parts[5])
		if !ok {
			return Ticket{}, errTicket()
		}
		out.App = app
	}
	if !out.Surface.Valid() {
		return Ticket{}, errTicket()
	}
	return out, nil
}

// errTicket is the single answer a bad ticket gives.
func errTicket() error { return ErrSignInExpired() }

// ErrSignInExpired is the answer a bad, expired or misplaced ticket gives.
func ErrSignInExpired() error {
	return core.Unauthenticated("this sign-in has expired; start again")
}

func ticketSignature(key []byte, label, body string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(label))
	mac.Write([]byte(body))
	return mac.Sum(nil)
}
