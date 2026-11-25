// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

func TestTheTokenFormMintsAClaimRequiringToken(t *testing.T) {
	for _, tt := range []struct {
		answer string
		want   bool
	}{{claimRequired, true}, {claimNotRequired, false}} {
		t.Run(tt.answer, func(t *testing.T) {
			m, svc := accountModel(t)
			m.form = TokenForm(m.actor)
			if got := m.form.Value("claim"); got != claimNotRequired {
				t.Fatalf("the form proposes %q for the claim, want %q", got, claimNotRequired)
			}
			m, _ = fill(t, m, map[string]string{"name": "agent-1", "preset": "Agent", "claim": tt.answer})
			if m.acct.picker == nil {
				t.Fatal("the token form did not open the scope picker")
			}
			heading := m.acct.picker.Lines()[0].Text
			if strings.Contains(heading, "requires a claim") != tt.want {
				t.Errorf("the picker heading reads %q", heading)
			}
			m, cmd := m.reduce(pressKey("enter"))
			drainAccount(t, m, cmd)
			if len(svc.createdTokens) != 1 || svc.createdTokens[0].RequireClaim != tt.want {
				t.Fatalf("CreateToken got %+v, want require_claim %v", svc.createdTokens, tt.want)
			}
		})
	}
}

func TestATokenSummarySaysItRequiresAClaim(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tok := core.APIToken{Scopes: []core.Scope{core.ScopeTaskRead}, RequireClaim: true}
	if got := TokenSummary(tok, now, output.TimeStyle{}); !strings.Contains(got, "claim required") {
		t.Errorf("summary = %q, want it to say the claim is required", got)
	}
	tok.RequireClaim = false
	if got := TokenSummary(tok, now, output.TimeStyle{}); strings.Contains(got, "claim") {
		t.Errorf("summary = %q for a token that requires no claim", got)
	}
}
