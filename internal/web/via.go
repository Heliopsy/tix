// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"github.com/heliopsy/tix/internal/core"
)

// viaView is how one recorded change's access channel reads on a page: an
// icon, a few words, and the details a hover and the raw view show.
type viaView struct {
	Kind    core.ViaKind
	Icon    string
	Label   string
	Arg     string
	Details []viaDetail
}

// viaDetail is one labelled fact about a channel, such as the client address.
type viaDetail struct {
	Label string
	Value string
}

// viaIcons maps each channel to its symbol in assets/icons.svg.
var viaIcons = map[core.ViaKind]string{
	core.ViaWeb:       "via-web",
	core.ViaAPI:       "via-api",
	core.ViaCLILocal:  "via-cli",
	core.ViaCLIRemote: "via-cli",
	core.ViaSSH:       "via-ssh",
	core.ViaMCP:       "via-mcp",
	core.ViaSync:      "sync",
	core.ViaImport:    "transfer",
	core.ViaSystem:    "settings",
	core.ViaUnknown:   "via-unknown",
}

// viaLabels are the words for each channel, the plain one and the one naming
// the token or job that identifies the caller.
var viaLabels = map[core.ViaKind][2]string{
	core.ViaWeb:       {"via web", "via web"},
	core.ViaAPI:       {"via API", "via token %s"},
	core.ViaCLILocal:  {"via CLI", "via CLI (token %s)"},
	core.ViaCLIRemote: {"via CLI", "via CLI (token %s)"},
	core.ViaSSH:       {"via SSH", "via SSH"},
	core.ViaMCP:       {"via MCP", "via MCP (token %s)"},
	core.ViaSync:      {"via sync", "via sync"},
	core.ViaImport:    {"via import", "via import"},
	core.ViaSystem:    {"via system", "via system (%s)"},
	core.ViaUnknown:   {"via unknown channel", "via unknown channel"},
}

// viaChoiceLabels name each channel in the activity filter's dropdown.
var viaChoiceLabels = map[core.ViaKind]string{
	core.ViaWeb:       "Web",
	core.ViaAPI:       "API token",
	core.ViaCLILocal:  "CLI on the database",
	core.ViaCLIRemote: "CLI through a server",
	core.ViaSSH:       "SSH",
	core.ViaMCP:       "MCP",
	core.ViaSync:      "Sync",
	core.ViaImport:    "Import",
	core.ViaSystem:    "System",
	core.ViaUnknown:   "Unknown (older entries)",
}

// viaDetailLabels are the field names the details carry, in display order.
var viaDetailLabels = []string{
	"Started from", "Token", "Token ID", "Job", "Session", "SSH key",
	"Address", "User agent", "Host", "OS user",
}

// newViaView describes a channel for a page.
func newViaView(v core.Via) viaView {
	v = v.Known()
	out := viaView{Kind: v.Kind, Icon: viaIcons[v.Kind], Label: viaLabels[v.Kind][0]}
	switch {
	case v.TokenName != "" && v.Kind != core.ViaSystem:
		out.Arg = v.TokenName
	case v.Job != "" && v.Kind == core.ViaSystem:
		out.Arg = v.Job
	}
	if out.Arg != "" {
		out.Label = viaLabels[v.Kind][1]
	}
	values := []string{
		string(v.Origin), v.TokenName, v.TokenID, v.Job, v.Session, v.KeyFingerprint,
		v.ClientAddr, v.UserAgent, v.Host, v.OSUser,
	}
	for i, value := range values {
		if value != "" {
			out.Details = append(out.Details, viaDetail{Label: viaDetailLabels[i], Value: value})
		}
	}
	return out
}

// viaChoice is one option of the activity feed's channel control.
type viaChoice struct {
	Value core.ViaKind
	Label string
}

// viaChoices are the channel filter's options, in core.ViaKinds order.
func viaChoices() []viaChoice {
	out := make([]viaChoice, len(core.ViaKinds))
	for i, k := range core.ViaKinds {
		out[i] = viaChoice{Value: k, Label: viaChoiceLabels[k]}
	}
	return out
}

// Channel is how this entry's access channel reads.
func (r historyRow) Channel() viaView { return newViaView(r.Entry.Via) }

// Channel is the access channel of the group's latest hop. Every hop of one
// group came from one request, so they share it.
func (g historyGroup) Channel() viaView { return g.Latest().Channel() }
