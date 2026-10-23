// SPDX-License-Identifier: AGPL-3.0-or-later

package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

func testKey(t *testing.T) string {
	t.Helper()
	key, err := auth.NewTOTPKey()
	if err != nil {
		t.Fatalf("minting a key: %v", err)
	}
	return key
}

func TestTicketRoundTrip(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	clk := clock.NewFakeAt()
	want := auth.Ticket{
		UserID:    "u1",
		TenantID:  "t1",
		Surface:   core.SurfaceWeb,
		ExpiresAt: clk.Now().Add(auth.TicketTTL).UTC().Truncate(time.Second),
	}
	minted, err := auth.MintTicket(key, want)
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	got, err := auth.ParseTicket(key, minted, clk.Now())
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if got.UserID != want.UserID || got.TenantID != want.TenantID || got.Surface != want.Surface {
		t.Fatalf("ticket read back as %+v, want %+v", got, want)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("ticket expires at %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}
}

func TestTicketRefusals(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	other := testKey(t)
	clk := clock.NewFakeAt()
	valid, err := auth.MintTicket(key, auth.Ticket{
		UserID: "u1", TenantID: "t1", Surface: core.SurfaceCLI,
		ExpiresAt: clk.Now().Add(auth.TicketTTL),
	})
	if err != nil {
		t.Fatalf("minting: %v", err)
	}

	// A payload that is well formed in every way except that nobody signed it
	// is the forgery the signature exists to stop. Random bytes in its place
	// are refused for being unreadable, which proves nothing about the
	// signature, so the substitute here is a payload the parser would accept.
	elsewhere, err := auth.MintTicket(other, auth.Ticket{
		UserID: "intruder", TenantID: "t1", Surface: core.SurfaceCLI,
		ExpiresAt: clk.Now().Add(auth.TicketTTL),
	})
	if err != nil {
		t.Fatalf("minting the forgery: %v", err)
	}
	forged, _, _ := strings.Cut(elsewhere, ".")
	_, sig, _ := strings.Cut(valid, ".")
	tampered := forged + "." + sig

	cases := []struct {
		name   string
		key    string
		ticket string
		at     time.Time
	}{
		{"another key", other, valid, clk.Now()},
		{"no signature", key, "nonsense", clk.Now()},
		{"tampered payload", key, tampered, clk.Now()},
		{"expired", key, valid, clk.Now().Add(auth.TicketTTL + time.Second)},
		{"empty", key, "", clk.Now()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := auth.ParseTicket(tc.key, tc.ticket, tc.at); !core.IsKind(err, core.KindUnauthenticated) {
				t.Fatalf("parsing a %s ticket returned %v, want it refused", tc.name, err)
			}
		})
	}
}

func TestTicketNeedsAKeyAndCompleteContents(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	at := clock.NewFakeAt().Now().Add(auth.TicketTTL)
	cases := []struct {
		name   string
		key    string
		ticket auth.Ticket
	}{
		{"no key", "", auth.Ticket{UserID: "u1", TenantID: "t1", Surface: core.SurfaceWeb, ExpiresAt: at}},
		{"no user", key, auth.Ticket{TenantID: "t1", Surface: core.SurfaceWeb, ExpiresAt: at}},
		{"no tenant", key, auth.Ticket{UserID: "u1", Surface: core.SurfaceWeb, ExpiresAt: at}},
		{"unknown surface", key, auth.Ticket{UserID: "u1", TenantID: "t1", Surface: "fax", ExpiresAt: at}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := auth.MintTicket(tc.key, tc.ticket); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("minting with %s returned %v, want it refused", tc.name, err)
			}
		})
	}
}
