// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"strconv"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestPageSizeDefaultsAreTwentyFive names the number itself. Fifty filled more
// than three screens of a browser window, and the number a fresh install shows
// is the whole point of the key, so it is asserted rather than inherited.
func TestPageSizeDefaultsAreTwentyFive(t *testing.T) {
	defaults := Defaults()
	for _, got := range []struct {
		key string
		n   int
	}{
		{"cli.page_size", defaults.CLI.PageSize},
		{"web.page_size", defaults.Web.PageSize},
	} {
		if got.n != 25 {
			t.Errorf("%s defaults to %d, want 25", got.key, got.n)
		}
	}
}

// TestPageSizeIsBoundedByTheContract checks the edges of the accepted range
// rather than only a value well inside it: the bound is the whole reason a
// configured page size cannot reach the store as something absurd.
func TestPageSizeIsBoundedByTheContract(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want bool
	}{
		{0, false},
		{-1, false},
		{1, true},
		{core.MaxPageLimit, true},
		{core.MaxPageLimit + 1, false},
	} {
		t.Run(strconv.Itoa(tc.n), func(t *testing.T) {
			cfg := Defaults()
			cfg.Web.PageSize = tc.n
			err := Validate(&cfg, map[string]Layer{})
			if accepted := err == nil; accepted != tc.want {
				t.Fatalf("web.page_size %d accepted = %v (err %v), want accepted = %v",
					tc.n, accepted, err, tc.want)
			}
		})
	}
}
