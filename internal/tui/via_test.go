// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// The live tail carries the channel on each event, so the activity bar can
// filter by it where it could never filter by source.
func TestTheLiveTailFiltersByChannel(t *testing.T) {
	f, err := ParseActivityFilter("via:mcp token:agent")
	if err != nil {
		t.Fatalf("the tail refused a channel term: %v", err)
	}
	mcp := core.Event{Type: core.EventTaskCreated, Via: &core.Via{Kind: core.ViaMCP, TokenName: "agent"}}
	web := core.Event{Type: core.EventTaskCreated, Via: &core.Via{Kind: core.ViaWeb}}
	old := core.Event{Type: core.EventTaskCreated}
	if !MatchesActivity(f, mcp) || MatchesActivity(f, web) || MatchesActivity(f, old) {
		t.Errorf("via:mcp token:agent matched mcp %v, web %v, old %v",
			MatchesActivity(f, mcp), MatchesActivity(f, web), MatchesActivity(f, old))
	}
}

func TestTheHistoryLineNamesTheChannel(t *testing.T) {
	line := HistoryRow(core.AuditEntry{Seq: 3, Action: "task.claim", Source: core.SourceTUI,
		Via: core.Via{Kind: core.ViaSSH}}, output.TimeStyle{}, nil)
	if !strings.Contains(line, "via ssh") {
		t.Errorf("history line = %q, want the ssh channel", line)
	}
	line = HistoryRow(core.AuditEntry{Seq: 4, Action: "task.create", Source: core.SourceAPI,
		Via: core.Via{Kind: core.ViaAPI, TokenName: "ci"}}, output.TimeStyle{}, nil)
	if !strings.Contains(line, "via api (ci)") {
		t.Errorf("history line = %q, want the token named", line)
	}
}
