// SPDX-License-Identifier: AGPL-3.0-or-later

package query_test

import (
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

func TestTheActivityFilterSelectsByChannelAndToken(t *testing.T) {
	t.Parallel()
	ssh := query.AuditRow(core.AuditEntry{Action: "task.claim", Via: core.Via{Kind: core.ViaSSH}})
	ci := query.AuditRow(core.AuditEntry{Action: "task.create", Via: core.Via{Kind: core.ViaAPI, TokenName: "ci-agent"}})
	old := query.AuditRow(core.AuditEntry{Action: "task.create"})
	for _, tc := range []struct {
		expr string
		want []bool
	}{
		{"via:ssh", []bool{true, false, false}},
		{"via:SSH via:api", []bool{true, true, false}},
		{"-via:ssh", []bool{false, true, true}},
		{"via:unknown", []bool{false, false, true}},
		{"token:ci-agent", []bool{false, true, false}},
		{"-token:ci-agent", []bool{true, false, true}},
		{"ci-agent", []bool{false, true, false}},
	} {
		f, err := query.ParseActivity(tc.expr)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		got := []bool{f.Matches(ssh), f.Matches(ci), f.Matches(old)}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s matched %v, want %v", tc.expr, got, tc.want)
		}
		if !f.Active() {
			t.Errorf("%s reads as no filter at all", tc.expr)
		}
	}
	if _, err := query.ParseActivity("via:telepathy"); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an unknown channel = %v, want invalid", err)
	}
	f, _ := query.ParseActivity("via:mcp token:agent")
	pushed := f.AuditFilter(core.AuditFilter{})
	if !slices.Equal(pushed.Vias, []core.ViaKind{core.ViaMCP}) || !slices.Equal(pushed.Tokens, []string{"agent"}) {
		t.Errorf("the store was asked for %+v", pushed)
	}
}
