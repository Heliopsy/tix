// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/heliopsy/tix/internal/core"
)

// TestParseDurationAcceptsTheVocabularyTheProductPrints covers the grammar
// itself: Go's own syntax, the day unit Go lacks, and the spaced form Human
// emits.
func TestParseDurationAcceptsTheVocabularyTheProductPrints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		want time.Duration
	}{
		{"go syntax still parses", "15m", 15 * time.Minute},
		{"go compound", "2h30m", 2*time.Hour + 30*time.Minute},
		{"fractional go syntax", "1.5h", 90 * time.Minute},
		{"sub-second units", "250ms", 250 * time.Millisecond},
		{"micros", "5µs", 5 * time.Microsecond},
		{"zero", "0", 0},
		{"the retention window that had to be typed as 720h", "30d", 720 * time.Hour},
		{"a year of events", "365d", 8760 * time.Hour},
		{"days and hours, spaced, as Human writes them", "16d 1h", 385 * time.Hour},
		{"days and hours, adjacent", "16d1h", 385 * time.Hour},
		{"fractional days", "1.5d", 36 * time.Hour},
		{"spaces between go terms", "1h 30m", 90 * time.Minute},
		{"surrounding whitespace", "  30d  ", 720 * time.Hour},
		{"negative", "-30d", -720 * time.Hour},
		{"explicit plus", "+2d", 48 * time.Hour},
		{"day zero", "0d", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := core.ParseDuration(tc.in)
			if err != nil {
				t.Fatalf("ParseDuration(%q): %v", tc.in, err)
			}
			if got.D() != tc.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tc.in, got.D(), tc.want)
			}
		})
	}
}

// TestParseDurationRejectsWhatIsNotADuration keeps the grammar from degrading
// into "accepts anything with a digit in it".
func TestParseDurationRejectsWhatIsNotADuration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in string }{
		{"empty", ""},
		{"whitespace only", "   "},
		{"no unit", "30"},
		{"unknown unit", "30x"},
		{"a week, which the product never prints", "4w"},
		{"months are calendar arithmetic", "2mo"},
		{"words", "thirty days"},
		{"trailing junk", "30d!"},
		{"two decimal points", "1.5.5d"},
		{"unit with no number", "d"},
		{"embedded sign", "1h-30m"},
		{"overflow", "999999999d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got, err := core.ParseDuration(tc.in); err == nil {
				t.Errorf("ParseDuration(%q) = %v, want an error", tc.in, got)
			}
		})
	}
}

// TestHumanOutputReparsesToAnIdenticalHumanRendering is the property that ties
// the two halves together: for every magnitude Human handles, parsing its
// output and rendering the result again gives the identical string. It is
// stated as "renders identically" rather than "equal nanoseconds" because
// Human deliberately drops precision as the magnitude rises, so "16d 1h" can
// never carry back the minutes and seconds it was rendered from.
func TestHumanOutputReparsesToAnIdenticalHumanRendering(t *testing.T) {
	t.Parallel()
	spread := []time.Duration{
		0,
		time.Second,
		42 * time.Second,
		59*time.Second + 900*time.Millisecond,
		time.Minute,
		15 * time.Minute,
		59*time.Minute + 59*time.Second,
		time.Hour,
		3*time.Hour + 25*time.Minute,
		23*time.Hour + 59*time.Minute,
		24 * time.Hour,
		26*time.Hour + 13*time.Minute,
		384*time.Hour + 50*time.Minute + 23*time.Second,
		720 * time.Hour,
		8760 * time.Hour,
		-26 * time.Hour,
	}
	for _, d := range spread {
		first := core.Duration(d).Human()
		parsed, err := core.ParseDuration(first)
		if err != nil {
			t.Errorf("Human(%v) = %q, which does not parse: %v", d, first, err)
			continue
		}
		if second := parsed.Human(); second != first {
			t.Errorf("Human(%v) = %q, reparsed and rendered as %q", d, first, second)
		}
	}
}

// TestDurationUnmarshalSharesTheGrammar keeps the document paths from being a
// second, narrower vocabulary.
func TestDurationUnmarshalSharesTheGrammar(t *testing.T) {
	t.Parallel()
	var fromJSON core.Duration
	if err := json.Unmarshal([]byte(`"30d"`), &fromJSON); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if fromJSON.D() != 720*time.Hour {
		t.Errorf("UnmarshalJSON gave %v, want 720h", fromJSON.D())
	}

	var fromYAML core.Duration
	if err := yaml.Unmarshal([]byte("16d 1h\n"), &fromYAML); err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}
	if fromYAML.D() != 385*time.Hour {
		t.Errorf("UnmarshalYAML gave %v, want 385h", fromYAML.D())
	}
}

// TestDurationStringStaysGoSyntax guards the snapshot format: String and
// MarshalJSON feed round-trips that time.ParseDuration must keep reading.
func TestDurationStringStaysGoSyntax(t *testing.T) {
	t.Parallel()
	d := core.Duration(720 * time.Hour)
	if got := d.String(); got != "720h0m0s" {
		t.Errorf("String() = %q, want Go syntax 720h0m0s", got)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(b) != `"720h0m0s"` {
		t.Errorf("MarshalJSON = %s, want Go syntax", b)
	}
	if _, err := time.ParseDuration(d.String()); err != nil {
		t.Errorf("String() is no longer readable by time.ParseDuration: %v", err)
	}
}
