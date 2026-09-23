// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/version"
)

// TestNavShowsTheRunningVersion renders a real page, because the value being
// on the view model proves nothing about it reaching the template.
func TestNavShowsTheRunningVersion(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")
	if !strings.Contains(page, `class="version"`) {
		t.Fatal("the navigation has no version block")
	}
	if !strings.Contains(page, "v"+strings.TrimPrefix(version.Version, "v")) {
		t.Fatalf("the page does not name the running version %q", version.Version)
	}
	// The upgrade note must not appear when nothing was fetched: a server with
	// no route out would otherwise tell every visitor to upgrade to nothing.
	if strings.Contains(page, "is available") {
		t.Error("the page offers an upgrade although no check has succeeded")
	}
}
