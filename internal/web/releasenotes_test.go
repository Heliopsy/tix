// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"
)

// realReleaseBody is what the release feed actually returned for v0.20.0,
// kept verbatim. release-please writes every changelog the same way, so this
// is the shape stripLinks has to survive rather than one invented for it.
const realReleaseBody = "## [0.20.0](https://github.com/Heliopsy/tix/compare/v0.19.0...v0.20.0) (2026-10-26)\n" +
	"\n\n### Features\n\n" +
	"* **lease:** bound claim leases per tenant and per deployment " +
	"([425cd60](https://github.com/Heliopsy/tix/commit/425cd605033cf4382e8a4565a57192623420edf5))\n" +
	"\n\n### Bug Fixes\n\n" +
	"* **store:** stop the sqlite planner scanning every task per claim " +
	"([8ce300e](https://github.com/Heliopsy/tix/commit/8ce300e5dc2c59dd03078786dc25ca7fa0c28636))"

// TestStripLinksLeavesNoOriginAndKeepsTheSentence is the whole contract. The
// notes have to survive as prose, because a reader deciding whether to
// upgrade needs them, and they have to carry no external origin, because
// TestPagesReferenceNoExternalOrigin refuses one on any rendered screen and
// what that test defends is a self-hosted page not republishing a remote
// server's links.
func TestStripLinksLeavesNoOriginAndKeepsTheSentence(t *testing.T) {
	out := stripLinks(realReleaseBody)

	for _, forbidden := range []string{"http://", "https://", "](", "github.com"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("stripped notes still contain %q:\n%s", forbidden, out)
		}
	}
	for _, want := range []string{
		"0.20.0",
		"bound claim leases per tenant and per deployment",
		"stop the sqlite planner scanning every task per claim",
		"Bug Fixes",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stripped notes lost %q:\n%s", want, out)
		}
	}
	// The bare sha the commit link collapsed to says nothing to a reader and
	// would sit at the end of every line.
	if strings.Contains(out, "425cd60") {
		t.Errorf("a bare commit hash survived:\n%s", out)
	}
}

// TestStripLinksHandlesABareURL covers notes somebody wrote by hand rather
// than ones release-please generated, where a link arrives without markdown
// around it and the markdown pass would miss it entirely.
func TestStripLinksHandlesABareURL(t *testing.T) {
	out := stripLinks("See https://example.invalid/advisory for the detail.")
	if strings.Contains(out, "https://") {
		t.Fatalf("a bare url survived: %q", out)
	}
	if !strings.Contains(out, "See") || !strings.Contains(out, "for the detail.") {
		t.Fatalf("the sentence around the url was lost: %q", out)
	}
}

// TestUpgradeStateCarriesNotesOnlyOnceChecked guards the empty case, which is
// the ordinary one for an isolated installation: no check has succeeded, so
// there is no version to compare and nothing to show, and the screen must say
// so rather than implying the install is current.
func TestUpgradeStateCarriesNotesOnlyOnceChecked(t *testing.T) {
	var w *releaseWatch
	if got := w.state(); got.Notes != "" || got.Newer {
		t.Fatalf("a watch that never ran reports %+v, want no notes and no upgrade", got)
	}
}
