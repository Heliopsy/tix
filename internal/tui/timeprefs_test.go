// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
)

// A date row the machine leaves unset says where the account's value came
// from, and that a value picked here stays on this machine.
func TestAnAccountDateRowSaysWhereItCameFrom(t *testing.T) {
	t.Parallel()
	for layer, want := range map[string]string{
		AccountLayer + "user":   "your own choice",
		AccountLayer + "tenant": "the tenant default",
		AccountLayer + "global": "the server default",
	} {
		note := OverrideNote(layer)
		if !strings.Contains(note, want) || !strings.Contains(note, "this machine only") {
			t.Errorf("OverrideNote(%q) = %q, want it to name %q and this machine", layer, note, want)
		}
	}
	lines := SettingsView(SettingsState{
		Prefs:   Preferences{TimeFormat: "dotted", Timezone: "Asia/Tokyo"},
		Sources: Preferences{TimeFormat: AccountLayer + "tenant", Timezone: AccountLayer + "user"},
	})
	var text []string
	for _, l := range lines {
		text = append(text, l.Text)
	}
	joined := strings.Join(text, "\n")
	for _, want := range []string{"output.time_format: from the tenant default", "output.timezone: from your own choice"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the settings screen does not say %q:\n%s", want, joined)
		}
	}
}
