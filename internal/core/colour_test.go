// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"math"
	"strings"
	"testing"
)

func TestParseHexTakesOnlySixDigits(t *testing.T) {
	tests := []struct {
		in   string
		want RGB
		ok   bool
	}{
		{"#1a7f5a", RGB{0x1a, 0x7f, 0x5a}, true},
		{"#1A7F5A", RGB{0x1a, 0x7f, 0x5a}, true},
		{"#000000", RGB{}, true},
		{"1a7f5a", RGB{}, false},
		{"#1a7", RGB{}, false},
		{"#1a7f5a00", RGB{}, false},
		{"#1a7f5g", RGB{}, false},
		{"", RGB{}, false},
	}
	for _, tc := range tests {
		got, ok := ParseHex(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseHex(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
		if ok && got.Hex() != strings.ToLower(tc.in) {
			t.Errorf("Hex() = %q, want %q", got.Hex(), strings.ToLower(tc.in))
		}
	}
}

func TestContrastMatchesTheWCAGReferenceValues(t *testing.T) {
	white, black := RGB{255, 255, 255}, RGB{}
	tests := []struct {
		a, b RGB
		want float64
	}{
		{white, black, 21},
		{black, white, 21},
		{white, white, 1},
		{RGB{0x76, 0x76, 0x76}, white, 4.54},
		{RGB{0x0f, 0x76, 0x6e}, white, 5.47},
	}
	for _, tc := range tests {
		if got := Contrast(tc.a, tc.b); math.Abs(got-tc.want) > 0.01 {
			t.Errorf("Contrast(%s, %s) = %.3f, want %.2f", tc.a.Hex(), tc.b.Hex(), got, tc.want)
		}
	}
}

func TestReadableMovesOnlyAsFarAsTheFloor(t *testing.T) {
	white, page := RGB{255, 255, 255}, RGB{0x14, 0x17, 0x1b}
	reads := RGB{0x0f, 0x76, 0x6e}
	if got := Readable(reads, 4.5, white); got != reads {
		t.Errorf("a colour that already reads was changed to %s", got.Hex())
	}
	for _, c := range []RGB{{255, 255, 0}, {0xff, 0xd1, 0xdc}, {255, 255, 255}, {0, 0, 0}, {0x1a, 0x1a, 0x80}} {
		light := Readable(c, 4.5, white, RGB{0xf6, 0xf7, 0xf9})
		if Contrast(light, white) < 4.5 || Contrast(light, RGB{0xf6, 0xf7, 0xf9}) < 4.5 {
			t.Errorf("%s on light came out %s, which does not clear 4.5", c.Hex(), light.Hex())
		}
		dark := Readable(c, 4.5, page)
		if Contrast(dark, page) < 4.5 {
			t.Errorf("%s on dark came out %s, which does not clear 4.5", c.Hex(), dark.Hex())
		}
		if Contrast(dark.Mix(page, 0.02), page) >= 4.5 && dark != c {
			t.Errorf("%s on dark moved further than it had to: %s", c.Hex(), dark.Hex())
		}
	}
}

func TestNearestPaletteColorPicksTheObviousNeighbour(t *testing.T) {
	tests := []struct {
		in   RGB
		want ProjectColor
	}{
		{RGB{0xdc, 0x26, 0x26}, ColorRed},
		{RGB{0xff, 0x00, 0x00}, ColorRed},
		{RGB{0x00, 0x80, 0x40}, ColorGreen},
		{RGB{0x20, 0x60, 0xff}, ColorBlue},
		{RGB{0x80, 0x80, 0x90}, ColorSlate},
	}
	for _, tc := range tests {
		if got := NearestPaletteColor(tc.in); got != tc.want {
			t.Errorf("NearestPaletteColor(%s) = %q, want %q", tc.in.Hex(), got, tc.want)
		}
	}
	for _, c := range ProjectColors() {
		shade, ok := PaletteShade(c)
		if !ok {
			t.Fatalf("palette colour %q has no shade", c)
		}
		if got := NearestPaletteColor(shade); got != c {
			t.Errorf("the shade of %q is nearest to %q", c, got)
		}
	}
}

func TestACustomThemeIsDerivedFromItsAccent(t *testing.T) {
	r, err := NewThemeRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	pale := r.Resolve(&Tenant{Theme: "#FFD1DC"})
	if pale.Name != "#ffd1dc" {
		t.Errorf("name = %q, want the lowercased accent", pale.Name)
	}
	white := RGB{255, 255, 255}
	for field, v := range map[string]string{"accent": pale.Accent, "dark": pale.AccentDark} {
		c, ok := ParseHex(v)
		if !ok {
			t.Fatalf("%s %q is not #rrggbb", field, v)
		}
		bg := white
		if field == "dark" {
			bg = darkPage
		}
		if Contrast(c, bg) < AccentFloor {
			t.Errorf("%s %s reaches %.2f, under %.1f", field, v, Contrast(c, bg), AccentFloor)
		}
	}
	for _, soft := range []string{pale.AccentSoft, pale.AccentSoftDark} {
		if _, ok := ParseHex(soft); !ok {
			t.Errorf("soft accent %q is not #rrggbb", soft)
		}
	}
	if got := r.Resolve(&Tenant{Theme: "#7a3e"}); got.Name != DefaultThemeName {
		t.Errorf("a malformed accent resolved to %q, want the default", got.Name)
	}
}
