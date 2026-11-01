// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"fmt"
	"math"
	"strings"
)

// RGB is an sRGB colour with eight bits per channel.
type RGB struct{ R, G, B uint8 }

// ParseHex reads #rrggbb in either case, and nothing else.
func ParseHex(s string) (RGB, bool) {
	if len(s) != 7 || s[0] != '#' {
		return RGB{}, false
	}
	var ch [3]uint8
	for i := range ch {
		hi, ok1 := hexDigit(s[1+2*i])
		lo, ok2 := hexDigit(s[2+2*i])
		if !ok1 || !ok2 {
			return RGB{}, false
		}
		ch[i] = hi<<4 | lo
	}
	return RGB{ch[0], ch[1], ch[2]}, true
}

func hexDigit(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}

// Hex renders the colour as lowercase #rrggbb.
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// Luminance is the WCAG 2 relative luminance, from 0 for black to 1 for white.
func (c RGB) Luminance() float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast is the WCAG 2 contrast ratio between two colours, from 1 to 21.
func Contrast(a, b RGB) float64 {
	la, lb := a.Luminance(), b.Luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Mix blends t of other into c, channel by channel in sRGB, the way CSS
// color-mix(in srgb) does.
func (c RGB) Mix(other RGB, t float64) RGB {
	m := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-t) + float64(b)*t)) }
	return RGB{m(c.R, other.R), m(c.G, other.G), m(c.B, other.B)}
}

// Readable returns c when it reaches floor against every background, and
// otherwise the least blend of c towards black or white that does, whichever
// direction the backgrounds call for.
func Readable(c RGB, floor float64, backgrounds ...RGB) RGB {
	if clears(c, floor, backgrounds) {
		return c
	}
	var sum float64
	for _, bg := range backgrounds {
		sum += bg.Luminance()
	}
	target := RGB{255, 255, 255}
	if len(backgrounds) == 0 || sum/float64(len(backgrounds)) > 0.18 {
		target = RGB{}
	}
	lo, hi := 0.0, 1.0
	for range 24 {
		mid := (lo + hi) / 2
		if clears(c.Mix(target, mid), floor, backgrounds) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return c.Mix(target, hi)
}

func clears(c RGB, floor float64, backgrounds []RGB) bool {
	for _, bg := range backgrounds {
		if Contrast(c, bg) < floor {
			return false
		}
	}
	return true
}

// paletteShades are the light-scheme shades of the project palette, the
// reference a custom colour is matched against where only a named colour can
// be drawn. They mirror the --proj-* tokens in the web stylesheet.
var paletteShades = map[ProjectColor]RGB{
	ColorSlate:  {0x5e, 0x6e, 0x85},
	ColorRed:    {0xdc, 0x26, 0x26},
	ColorAmber:  {0xb4, 0x53, 0x09},
	ColorGreen:  {0x15, 0x80, 0x3d},
	ColorTeal:   {0x0f, 0x76, 0x6e},
	ColorBlue:   {0x25, 0x63, 0xeb},
	ColorViolet: {0x7c, 0x3a, 0xed},
	ColorPink:   {0xbe, 0x18, 0x5d},
}

// PaletteShade returns the light-scheme shade of a palette colour.
func PaletteShade(c ProjectColor) (RGB, bool) {
	rgb, ok := paletteShades[c]
	return rgb, ok
}

// NearestPaletteColor returns the palette colour closest to c, by the
// redmean approximation of perceived distance.
func NearestPaletteColor(c RGB) ProjectColor {
	best, bestDist := ColorSlate, math.Inf(1)
	for _, name := range projectColors {
		if d := distance(c, paletteShades[name]); d < bestDist {
			best, bestDist = name, d
		}
	}
	return best
}

func distance(a, b RGB) float64 {
	rmean := (float64(a.R) + float64(b.R)) / 2
	dr, dg, db := float64(a.R)-float64(b.R), float64(a.G)-float64(b.G), float64(a.B)-float64(b.B)
	return (2+rmean/256)*dr*dr + 4*dg*dg + (2+(255-rmean)/256)*db*db
}

// normalizeHex lowercases a #rrggbb value, reporting whether it was one.
func normalizeHex(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if _, ok := ParseHex(s); !ok {
		return "", false
	}
	return s, true
}
