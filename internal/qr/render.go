// SPDX-License-Identifier: AGPL-3.0-or-later

package qr

import "strings"

// QuietZone is the margin of light modules a reader needs around a symbol,
// from ISO/IEC 18004 section 9.1. A symbol drawn flush against surrounding
// text does not scan, and that failure looks like a bad camera rather than a
// bad margin.
const QuietZone = 4

// Colours are stated rather than left to the terminal's palette. A symbol
// drawn in the foreground colour over whatever background the person chose is
// a coin toss: half of all terminals would render it inverted, and an
// inverted symbol does not scan.
const (
	darkOnLight = "\x1b[30;107m"
	lightOnDark = "\x1b[97;40m"
	bothDark    = "\x1b[30;40m"
	bothLight   = "\x1b[97;107m"
	reset       = "\x1b[0m"
)

// upperHalf fills the top half of a cell, so one text row carries two module
// rows and a symbol stays roughly square on a terminal whose cells are twice
// as tall as they are wide.
const upperHalf = "▀"

// Terminal renders the symbol as ANSI half blocks with its quiet zone, one
// line per two module rows, each line terminated and colour-reset.
//
// It is the caller's job to decide whether the destination can show it. A
// terminal that strips escape sequences receives a block of identical
// characters, which is worse than the URI on its own.
func (m Matrix) Terminal() string {
	size := m.Size() + 2*QuietZone
	dark := func(x, y int) bool {
		x, y = x-QuietZone, y-QuietZone
		if x < 0 || y < 0 || x >= m.Size() || y >= m.Size() {
			return false
		}
		return m[y][x]
	}
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			top, bottom := dark(x, y), dark(x, y+1)
			switch {
			case top && bottom:
				b.WriteString(bothDark)
			case top:
				b.WriteString(darkOnLight)
			case bottom:
				b.WriteString(lightOnDark)
			default:
				b.WriteString(bothLight)
			}
			b.WriteString(upperHalf)
		}
		b.WriteString(reset + "\n")
	}
	return b.String()
}
