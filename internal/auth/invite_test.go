// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A minted invite secret parses, its hash is the token hash, and every
// malformed value is the same not found.
func TestInviteTokensRoundTripAndMalformedOnesAreNotFound(t *testing.T) {
	value, hash, err := MintInviteToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(value, InviteTokenPrefix) || hash != HashToken(value) {
		t.Fatalf("minted %q with hash %q", value, hash)
	}
	if err := ParseInviteToken(value); err != nil {
		t.Errorf("a minted secret does not parse: %v", err)
	}
	other, _, _ := MintInviteToken()
	if other == value {
		t.Error("two invites share a secret")
	}
	secret := strings.TrimPrefix(value, InviteTokenPrefix)
	for _, bad := range []string{
		"", secret, CalendarTokenPrefix + secret, value + "a", strings.ToUpper(value),
		InviteTokenPrefix + strings.Repeat("1", len(secret)),
	} {
		err := ParseInviteToken(bad)
		if !core.IsKind(err, core.KindNotFound) || err.Error() != ErrNoInvite().Error() {
			t.Errorf("ParseInviteToken(%q) = %v, want the one not found", bad, err)
		}
	}
}
