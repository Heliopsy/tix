// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "testing"

// TestNewerRelease pins the comparison, including the cases where staying
// quiet is the right answer. Telling somebody to upgrade on the strength of a
// string nobody parsed is worse than saying nothing.
func TestNewerRelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		running, latest string
		want            bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.0", "0.1.0", false},
		{"0.2.0", "1.0.0", true},
		{"1.9.0", "1.10.0", true}, // not a string comparison
		{"0.2.0", "0.2.1", true},
		{"dev", "0.2.0", true}, // any release beats a development build
		{"dev", "", false},     // ...but not to a release nobody fetched
		{"0.2.0", "", false},   // no check has succeeded yet
		{"0.2.0", "not-a-version", false},
		{"not-a-version", "0.2.0", false},
		{"0.2.0-rc.1", "0.2.0", false}, // suffixes are ignored, so these are equal
	}
	for _, tc := range cases {
		if got := newerRelease(tc.running, tc.latest); got != tc.want {
			t.Errorf("newerRelease(%q, %q) = %v, want %v", tc.running, tc.latest, got, tc.want)
		}
	}
}

// TestReleaseStateNeverBlocks is the property that matters at render time: the
// first call returns immediately with whatever is known, which is nothing.
func TestReleaseStateNeverBlocks(t *testing.T) {
	t.Parallel()
	w := newReleaseWatch()
	got := w.state()
	if got.Running == "" {
		t.Error("state() gave no running version")
	}
	if got.Newer {
		t.Error("state() claimed an upgrade before any check had succeeded")
	}
}

// TestNilReleaseWatchStillRenders covers a handler built without one, since a
// page that panics is worse than a page with no version on it.
func TestNilReleaseWatchStillRenders(t *testing.T) {
	t.Parallel()
	var w *releaseWatch
	if got := w.state(); got.Running == "" {
		t.Error("a nil watch gave no running version")
	}
}
