// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestCalendarTokenRoundTrips(t *testing.T) {
	value, hash, err := MintCalendarToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(value, CalendarTokenPrefix) || hash != HashToken(value) {
		t.Fatalf("minted %q with hash %q", value, hash)
	}
	if err := ParseCalendarToken(value); err != nil {
		t.Fatalf("a minted secret did not parse: %v", err)
	}
}

func TestCalendarTokenRefusesOtherShapes(t *testing.T) {
	value, _, err := MintCalendarToken()
	if err != nil {
		t.Fatal(err)
	}
	secret := strings.TrimPrefix(value, CalendarTokenPrefix)
	for name, bad := range map[string]string{
		"empty":        "",
		"api token":    TokenPrefix + secret,
		"upper case":   CalendarTokenPrefix + strings.ToUpper(secret),
		"short":        value[:len(value)-1],
		"long":         value + "a",
		"not base32":   CalendarTokenPrefix + strings.Repeat("1", len(secret)),
		"with suffix":  value + ".ics",
		"no prefix":    secret,
		"path escapes": CalendarTokenPrefix + "../" + secret[3:],
	} {
		if err := ParseCalendarToken(bad); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("%s: %q parsed as %v, want not found", name, bad, err)
		}
	}
}
