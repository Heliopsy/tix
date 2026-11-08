// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/onboard"

// The connect screen translates the snippet titles and notes internal/onboard
// hands it, and the scope picker its group names, notes and presets.
func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, g := range scopeGroups {
			out = append(out, g.Name)
		}
		for _, n := range scopeNotes {
			out = append(out, n)
		}
		for _, p := range onboard.Presets {
			out = append(out, p.Name, p.Note)
		}
		t := onboard.Target{URL: "u", Tenant: "t", Token: "k", SSH: "h:1"}
		for _, p := range connectPanes {
			out = append(out, p.Title)
		}
		out = append(out, "Install with", "MCP client", "Install for")
		snips := append(onboard.Install(t), onboard.CLI(t)...)
		snips = append(snips, *onboard.Login(t, "e"), onboard.Rotate())
		snips = append(snips, onboard.MCPPreview())
		snips = append(snips, onboard.Skill()...)
		snips = append(snips, onboard.SSH(t)...)
		snips = append(snips, onboard.REST(t)...)
		for _, c := range onboard.MCPClients(t) {
			snips = append(snips, c.Snippets...)
		}
		for _, s := range snips {
			if s.Path == "" {
				out = append(out, s.Title)
			}
			if s.Note != "" {
				out = append(out, s.Note)
			}
		}
		return out
	})
}
