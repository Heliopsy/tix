// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TestHumanDropsPrecisionAsTheMagnitudeRises pins the rendering three surfaces
// share. The statistics screens were readable in a browser, which had a
// private copy of this, and printed "384h50m23.606839092s" on the command line
// and in the terminal, which had none.
func TestHumanDuration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   time.Duration
		want string
	}{
		{"seconds", 42 * time.Second, "42s"},
		{"under a minute rounds down", 59*time.Second + 900*time.Millisecond, "59s"},
		{"minutes", 15 * time.Minute, "15m"},
		{"whole hours carry no minutes", 3 * time.Hour, "3h"},
		{"hours and minutes", 3*time.Hour + 25*time.Minute, "3h 25m"},
		{"a day is days", 24 * time.Hour, "1d"},
		{"days and hours", 26 * time.Hour, "1d 2h"},
		{"the lead time that started this", 384*time.Hour + 50*time.Minute + 23*time.Second, "16d"},
		{"negative reads as its magnitude", -26 * time.Hour, "1d 2h"},
		{"zero", 0, "0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := core.Duration(tc.in).Human(); got != tc.want {
				t.Errorf("Duration(%v).Human() = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// String stays Go's own rendering: it is what the JSON and YAML encodings are
// built from, and a snapshot that round-trips has to keep the precision the
// human form deliberately throws away.
func TestHumanDoesNotReplaceString(t *testing.T) {
	t.Parallel()
	d := core.Duration(90 * time.Minute)
	if d.String() == d.Human() {
		t.Errorf("String() and Human() both returned %q, want the machine form to keep its precision", d.String())
	}
}
