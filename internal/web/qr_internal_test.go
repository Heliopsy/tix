// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strings"
	"testing"
)

// The published worked example in ISO/IEC 18004 for a version 1 level M
// symbol: these sixteen data codewords produce these ten error correction
// codewords. It pins the Reed-Solomon arithmetic on its own, because a fault
// there would otherwise only show as a symbol that scans wrong somewhere no
// test can see.
func TestErrorCorrectionMatchesThePublishedVector(t *testing.T) {
	t.Parallel()
	data := []byte{0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D,
		0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11}
	want := []byte{0xC4, 0x23, 0x27, 0x77, 0xEB, 0xD7, 0xE7, 0xE2, 0x5D, 0x17}

	got := qrErrorCorrection(data, len(want))
	if len(got) != len(want) {
		t.Fatalf("got %d error correction codewords, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("codeword %d = %#02x, want %#02x (got %#v)", i, got[i], want[i], got)
		}
	}
}

// The symbols themselves, module for module, against the ones libqrencode
// draws for the same input at the same level in the same mode:
//
//	qrencode -8 -l M -m 0 -t ASCII "<text>" | sed 's/##/#/g; s/  /./g'
//
// Comparing whole matrices rather than a property of them is the point. A
// placement, masking or interleaving fault produces a symbol that passes
// every structural check and scans as nothing, and there is no decoder here
// to catch it, so the guard is an independent encoder's answer.
func TestSymbolsMatchAnIndependentEncoder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "version one",
			text: "HELLO",
			want: `
#######.#..#..#######
#.....#.####..#.....#
#.###.#...#.#.#.###.#
#.###.#.#.#.#.#.###.#
#.###.#....#..#.###.#
#.....#....##.#.....#
#######.#.#.#.#######
........#..##........
#.##.###.#.##.#..#.##
.##.##.#.######..##..
#...#.#..#.#.......##
#.##...#...#..####.#.
.#.######...#..#..#.#
........####..#...#.#
#######.#..##..#.....
#.....#.#.#....#####.
#.###.#.....######.##
#.###.#.#.##..#.####.
#.###.#.##..#.##..#..
#.....#...#..#.##...#
#######.#.#..#.#.....`,
		},
		{
			name: "an otpauth uri",
			text: "otpauth://totp/Acme%20Works:alice@example.test" +
				"?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=Acme%20Works",
			want: `
#######.###..###....###..####.#.#.#######
#.....#.#..#.#.##...#.##....#...#.#.....#
#.###.#..#...#.....##..#.####.##..#.###.#
#.###.#.###.#.##.#..#.....##.#..#.#.###.#
#.###.#..##.#..##..#..#.#.###.##..#.###.#
#.....#..#..#.#.#####...##..#####.#.....#
#######.#.#.#.#.#.#.#.#.#.#.#.#.#.#######
........##..#########.#...#.###..........
#.##.###.###..#.####.##....#.#.#..#..#.##
#..###..###.##.#...###.#....####...#####.
#####.###.###..#......###....#..#.#.##.#.
.....#..##.#.####...#.#.###.......##....#
.#..###.#..#....##..#..###.###.##....###.
#......##.....##.#.#..#...##.#.#.#.......
....###.##.#...#...####.##..#.######.####
.###...#..###.##..#####..............#...
....###..##.###.#####.##....####.#..#..##
###.#.....#####..#.##...#####..###.#.###.
.###..#.#..####..##.#...#..####..#.##.##.
#.###..#.####.##.####......#.##..###.####
#.#######.####...###.####.######.###.#..#
#..###...#..#.#.#..#####.#..##.######.###
.##...#...#.#####.....####....#.#.##.#.#.
....##.#...........##...###.#.##.##..#...
#...#.#.#...#....#..#...#.####..##.######
#.#..#.######.####...##.###########..###.
##.#####....#.#...##.#..#.#....#..####.##
#..#.#..#..####......####...#....#..##.#.
....###.###....##.#.####.#.##.###..###..#
.#......#..#..#.#.#.#..##.##.#..#..#.#.#.
#..#.##.#####.#.#...#.#....##.#.....##.#.
..#.##.#.#.#.##..###..###.#.##.#....###..
.####.#.#..####...###..#.##.#.###########
........########..##.##...###...#...#...#
#######.######.##......#.##..##.#.#.#..#.
#.....#.#...#...#.##..#..##.#.#.#...#..#.
#.###.#...###.#...##.##.######..######.##
#.###.#.#..##.####.#..#.#.####.#.#.##.#.#
#.###.#.###..#..##.####..#....##...#..###
#.....#...#.###..###.#....###.#.....##.#.
#######.##...#..##...#.#.#.#..####.#.###.`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, err := newQRCode(tc.text)
			if err != nil {
				t.Fatalf("encoding: %v", err)
			}
			want := strings.TrimSpace(tc.want)
			if got := code.render(); got != want {
				t.Errorf("symbol differs from the reference encoder\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// render draws the modules the way the reference encoder's ASCII output is
// normalised, one character per module.
func (q *qrCode) render() string {
	var out strings.Builder
	for y := range q.size {
		if y > 0 {
			out.WriteString("\n")
		}
		for x := range q.size {
			if q.at(x, y) {
				out.WriteString("#")
				continue
			}
			out.WriteString(".")
		}
	}
	return out.String()
}

func TestQRRefusesWhatItCannotHold(t *testing.T) {
	t.Parallel()
	if _, err := qrSVG(strings.Repeat("x", 214), "too much"); err == nil {
		t.Fatal("a payload past version ten was encoded anyway")
	}
	if _, err := qrSVG(strings.Repeat("x", 213), "the largest"); err != nil {
		t.Fatalf("the largest payload version ten holds was refused: %v", err)
	}
}

func TestQRSVGIsSelfContainedMarkup(t *testing.T) {
	t.Parallel()
	svg, err := qrSVG("otpauth://totp/tix:alice@example.test?secret=JBSWY3DPEHPK3PXP", `Scan "me"`)
	if err != nil {
		t.Fatalf("drawing: %v", err)
	}
	markup := string(svg)
	for _, want := range []string{`<svg class="qr"`, `viewBox="0 0 41 41"`, `role="img"`, `<path fill="#000000"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("the markup carries no %s:\n%s", want, markup)
		}
	}
	if strings.Contains(markup, `aria-label="Scan "me""`) {
		t.Error("the label was not escaped into the attribute")
	}
	if strings.Contains(markup, "<script") || strings.Contains(markup, "<image") ||
		strings.Contains(markup, "href=") {
		t.Error("the drawing reaches outside the page")
	}
}
