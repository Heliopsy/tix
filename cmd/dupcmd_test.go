// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNoCommandIsRegisteredTwice pins the fix for `tix task --help` listing
// copy, star and unstar twice: a rebase kept both sides of an AddCommand list,
// and Cobra registers a duplicate without complaint.
func TestNoCommandIsRegisteredTwice(t *testing.T) {
	t.Parallel()
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		seen := map[string]bool{}
		for _, sub := range c.Commands() {
			for _, name := range append([]string{sub.Name()}, sub.Aliases...) {
				if seen[name] {
					t.Errorf("%q has %q registered twice", c.CommandPath(), name)
				}
				seen[name] = true
			}
			walk(sub)
		}
	}
	walk(root)
}
