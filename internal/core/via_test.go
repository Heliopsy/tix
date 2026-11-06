// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/heliopsy/tix/internal/core"
)

// A client hint narrows a token request and nothing else. Every other channel
// is decided by the transport, so a header naming one must change nothing.
func TestAClientHintOnlyRefinesTheAPIChannel(t *testing.T) {
	cases := []struct {
		base core.ViaKind
		hint string
		want core.ViaKind
	}{
		{core.ViaAPI, "cli", core.ViaCLIRemote},
		{core.ViaAPI, " MCP ", core.ViaMCP},
		{core.ViaAPI, "", core.ViaAPI},
		{core.ViaAPI, "ssh", core.ViaAPI},
		{core.ViaAPI, "web", core.ViaAPI},
		{core.ViaAPI, "system", core.ViaAPI},
		{core.ViaAPI, "cli-local", core.ViaAPI},
		{core.ViaWeb, "cli", core.ViaWeb},
		{core.ViaWeb, "mcp", core.ViaWeb},
		{core.ViaSSH, "cli", core.ViaSSH},
		{core.ViaSystem, "mcp", core.ViaSystem},
		{core.ViaCLILocal, "mcp", core.ViaCLILocal},
	}
	for _, tc := range cases {
		got := core.Via{Kind: tc.base, TokenName: "ci"}.Refine(tc.hint)
		if got.Kind != tc.want || got.TokenName != "ci" {
			t.Errorf("%s refined by %q = %+v, want kind %s with the token kept", tc.base, tc.hint, got, tc.want)
		}
	}
}

func TestDeriveKeepsWhoStartedTheWork(t *testing.T) {
	started := core.Via{Kind: core.ViaWeb, Session: "s-1", ClientAddr: "192.0.2.1"}
	got := started.Derive(core.ViaSync)
	if got.Kind != core.ViaSync || got.Origin != core.ViaWeb || got.Session != "s-1" {
		t.Errorf("derived = %+v, want sync started from web with the session kept", got)
	}
	if again := got.Derive(core.ViaSync); again != got {
		t.Errorf("deriving the same channel twice changed it: %+v", again)
	}
	if fresh := (core.Via{}).Derive(core.ViaImport); fresh.Origin != "" || fresh.Kind != core.ViaImport {
		t.Errorf("an import nobody's channel started = %+v, want no origin", fresh)
	}
}

func TestAnEventCarriesNoNetworkDetail(t *testing.T) {
	full := core.Via{
		Kind: core.ViaMCP, TokenID: "t1", TokenName: "agent", Session: "s-1",
		KeyFingerprint: "SHA256:x", ClientAddr: "192.0.2.1", UserAgent: "ua", Host: "h", OSUser: "u",
	}
	got := full.Public()
	want := core.Via{Kind: core.ViaMCP, TokenID: "t1", TokenName: "agent"}
	if got != want {
		t.Errorf("public = %+v, want %+v", got, want)
	}
}

func TestAContextWithoutAChannelReadsAsUnknown(t *testing.T) {
	if got := core.ViaFrom(context.Background()).Kind; got != core.ViaUnknown {
		t.Errorf("no channel = %q, want unknown", got)
	}
	ctx := core.WithVia(context.Background(), core.Via{Kind: core.ViaSSH})
	if got := core.ViaFrom(ctx).Kind; got != core.ViaSSH {
		t.Errorf("channel = %q, want ssh", got)
	}
	if got := (core.Via{Kind: "telepathy"}).Known().Kind; got != core.ViaUnknown {
		t.Errorf("a kind nobody defined = %q, want unknown", got)
	}
}

func TestTheUserAgentIsCutOnACharacter(t *testing.T) {
	long := strings.Repeat("é", core.MaxViaUserAgent)
	got := core.TruncateUserAgent(long)
	if len(got) > core.MaxViaUserAgent || !utf8.ValidString(got) {
		t.Errorf("truncated to %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
	if got := core.TruncateUserAgent("  tix-client  "); got != "tix-client" {
		t.Errorf("short agent = %q", got)
	}
}
