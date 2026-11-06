// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/handoff"
)

// TestHandoffNamesOnlyRealCommandsAndFlags resolves every tix command the
// task screen's hand-off prompt names against the real command tree, the way
// the skill's own guard does, so a renamed command or flag fails here before
// an agent is told to run it.
func TestHandoffNamesOnlyRealCommandsAndFlags(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	prompt := handoff.Prompt(handoff.Task{Ref: "INFRA-12", Title: "Fix the build", Server: "https://tix.example.com"})

	var spans []string
	for _, span := range inlineCode(prompt) {
		if strings.HasPrefix(span, "tix ") {
			spans = append(spans, span)
		}
	}
	if len(spans) < 4 {
		t.Fatalf("found %d tix commands in %q, want claim, show, comment and release", len(spans), prompt)
	}
	for _, want := range []string{"tix claim task", "tix task show", "tix comment add", "tix claim release"} {
		if !strings.Contains(strings.Join(spans, "\n"), want) {
			t.Errorf("the prompt does not name %s", want)
		}
	}
	for _, span := range spans {
		checkInvocation(t, root, "the hand-off prompt", span)
	}
}
