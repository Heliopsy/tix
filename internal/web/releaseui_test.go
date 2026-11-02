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

// TestTheVersionOpensWhatChanged reads the footer's version block alone, so a
// link elsewhere on the page to the same place does not stand in for it, and
// then follows the link to the section it names.
func TestTheVersionOpensWhatChanged(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")
	start := strings.Index(page, `<p class="version">`)
	if start < 0 {
		t.Fatal("the navigation has no version block")
	}
	block, _, _ := strings.Cut(page[start:], "</p>")
	running := "v" + strings.TrimPrefix(version.Version, "v")
	if !strings.Contains(block, `<a class="running" href="/release#changes"`) || !strings.Contains(block, ">"+running+"</a>") {
		t.Fatalf("the version is not a link to /release#changes:\n%s", block)
	}
	if release := f.as("alice").page("/release"); !strings.Contains(release, `<section class="relsec relchanges" id="changes"`) {
		t.Fatalf("/release has no section with id changes:\n%s", release)
	}
}
