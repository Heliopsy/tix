// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"slices"
	"testing"
)

// The Share menu's (?) opens a heading docs/ has, through the same redirect
// as every screen's help button.
func TestTheShareHelpLandsOnAHeading(t *testing.T) {
	if !slices.Contains(docPages, shareHelp.Page) {
		t.Fatalf("the Share help opens %s, which the docs redirect does not serve", shareHelp.Page)
	}
	if !docAnchors(t, shareHelp.Page)[shareHelp.Section] {
		t.Fatalf("docs/%s has no heading with the anchor #%s", shareHelp.Page, shareHelp.Section)
	}
	if !docsSection.MatchString(shareHelp.Section) {
		t.Fatalf("#%s is not an anchor the redirect passes on", shareHelp.Section)
	}
}
