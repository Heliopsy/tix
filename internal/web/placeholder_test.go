// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"
)

// TestPlaceholdersAndNativeControlsFollowTheScheme pins the fix for field
// hints painted near-black on the dark and dim surfaces.
//
// The sheet never coloured ::placeholder and never declared color-scheme, so
// the browser drew every placeholder, and every native control part it owns,
// in its light-scheme defaults whichever theme the tokens had switched to.
func TestPlaceholdersAndNativeControlsFollowTheScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("").get("/assets/app.css"))

	hint := declarations(t, sheet, "::placeholder")
	if !strings.Contains(hint, "color: var(--muted)") {
		t.Errorf("placeholders are not drawn in the muted token, so they keep the browser's light-scheme grey:\n%s", hint)
	}

	tests := []struct {
		selector string
		scheme   string
	}{
		{":root {", "light"},
		{`:root[data-theme="dim"] {`, "dark"},
		{`:root[data-theme="dark"] {`, "dark"},
		{`:root:not([data-theme="light"]):not([data-theme="dim"]) {`, "dark"},
	}
	for _, tt := range tests {
		got := declarations(t, sheet, tt.selector)
		if !strings.Contains(got, "color-scheme: "+tt.scheme) {
			t.Errorf("%s does not declare color-scheme: %s, so native controls keep the wrong scheme", tt.selector, tt.scheme)
		}
	}
}
