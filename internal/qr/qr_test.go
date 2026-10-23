// SPDX-License-Identifier: AGPL-3.0-or-later

package qr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// render turns a symbol into one line per row, which is the form the golden
// digests below were taken over.
func render(m Matrix) string {
	rows := make([]string, 0, len(m))
	for _, row := range m {
		var b strings.Builder
		for _, dark := range row {
			if dark {
				b.WriteByte('X')
			} else {
				b.WriteByte('.')
			}
		}
		rows = append(rows, b.String())
	}
	return strings.Join(rows, "\n")
}

func digest(m Matrix) string {
	sum := sha256.Sum256([]byte(render(m)))
	return hex.EncodeToString(sum[:])
}

// v1Symbol is one golden in full, so that a reader can see what the digests
// below stand for rather than taking them on trust.
const v1Symbol = `XXXXXXX....X..XXXXXXX
X.....X.XXX...X.....X
X.XXX.X....X..X.XXX.X
X.XXX.X..XX...X.XXX.X
X.XXX.X.XX..X.X.XXX.X
X.....X.....X.X.....X
XXXXXXX.X.X.X.XXXXXXX
..........XXX........
X.X.X.X....X....X..X.
XX.XXX..X.X...X..XXXX
XXX...XX.X..X...XXXXX
XXX..X..X.X...X....X.
XX..XXXXX...X.X.XX.XX
........XX.X.X.X.XX.X
XXXXXXX....X.XXX..XXX
X.....X..XXXXX.XXX..X
X.XXX.X.X..X.XXX..X.X
X.XXX.X...X...X...XX.
X.XXX.X.XXX.X...X...X
X.....X..XX...X...XX.
XXXXXXX.XX..X.X.X.XXX`

// golden symbols come from libqrencode 4.1.1, run as
//
//	qrencode -t ASCII -m 0 -l M -8 TEXT
//
// and are compared under the mask that reference symbol chose. Mask selection
// is a legibility heuristic that no reader decodes, so two encoders may
// disagree on it while producing equally valid symbols; everything else, from
// the version chosen through the error correction to module placement, has
// exactly one right answer and is what these pin.
var golden = []struct {
	name string
	text string
	mask int
	size int
	sum  string
}{
	{name: "a short string", text: "tix", mask: 0, size: 21,
		sum: "9a6d1b90276e0b3a6e34dfe6b5885ebc6afbe588e8b8467b9a58f5b5c115a0b9"},
	{name: "an otpauth uri", mask: 2, size: 41,
		text: "otpauth://totp/tix:ada@example.com?secret=JBSWY3DPEHPK3PXP" +
			"&issuer=tix&algorithm=SHA1&digits=6&period=30",
		sum: "7d15928da8818f95f622a1a1706922ce1d96a1cef8d5f2bd2ed4e45c433c15b4"},
	{name: "version 1 filled", text: strings.Repeat("a", 14), mask: 1, size: 21,
		sum: "40478d675b19c1063e8d71fcf3f5297fb9b439bb7de6b13a7740d33e5e6a06d7"},
	{name: "one byte past version 1", text: strings.Repeat("b", 15), mask: 1, size: 25,
		sum: "37184891494a7a469f222b2fc9c762a61f62fddbc6023b64f5621064f97d6e9d"},
	{name: "version 4, the first with two blocks", text: strings.Repeat("c", 62), mask: 2, size: 33,
		sum: "3daa047154db34e9321e76dff006d54ce72144a599ed0c09a0bbe18b49761065"},
	{name: "version 6", text: strings.Repeat("d", 106), mask: 2, size: 41,
		sum: "701c8fefbdaee83884563dfece1aa11630b91b7589da6d013d152974dff893ee"},
	{name: "version 7, the first carrying version information", text: strings.Repeat("e", 122), mask: 1, size: 45,
		sum: "e5ab2ea6147449fb44f38d1bd5c0f697394f08ad591cdc2a38524b634ca7811c"},
	{name: "version 8, the first with two block sizes", text: strings.Repeat("f", 152), mask: 2, size: 49,
		sum: "cdc115bb575a369e9b80bef7a672aa0f13518cbc7073d9b6af30b40f0699b44e"},
	{name: "version 9", text: strings.Repeat("g", 180), mask: 2, size: 53,
		sum: "abd90183c6540ffd45c013fa03d7ad5ae738b1a3f254253142cdb07698211dfa"},
	{name: "version 10, the largest supported", text: strings.Repeat("h", 213), mask: 1, size: 57,
		sum: "efee23f3dca6932fce523819773e14d9e5bc8ef7a53e85d15c04ae0b42ad0ff3"},
}

// TestSymbolsMatchAReferenceEncoder is the guard that the symbol is readable
// at all. Nothing else in this repository can tell a scannable code from a
// convincing grid of squares.
func TestSymbolsMatchAReferenceEncoder(t *testing.T) {
	t.Parallel()
	for _, tc := range golden {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := encodeMask(tc.text, tc.mask)
			if err != nil {
				t.Fatalf("encoding %d bytes: %v", len(tc.text), err)
			}
			if m.Size() != tc.size {
				t.Fatalf("size = %d, want %d", m.Size(), tc.size)
			}
			if got := digest(m); got != tc.sum {
				t.Fatalf("symbol digest = %s, want %s\n%s", got, tc.sum, render(m))
			}
		})
	}
}

// TestTheSmallestSymbolIsTheOneWritten keeps one golden legible, so that a
// digest failure above can be read against something.
func TestTheSmallestSymbolIsTheOneWritten(t *testing.T) {
	t.Parallel()
	m, err := encodeMask("tix", 0)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if got := render(m); got != v1Symbol {
		t.Fatalf("symbol =\n%s\nwant\n%s", got, v1Symbol)
	}
}

// TestEncodePicksAMaskWithoutChangingWhatIsEncoded holds the property that
// matters about mask selection: whichever one it lands on, the symbol is the
// one that mask produces, so nothing else was disturbed in choosing it.
func TestEncodePicksAMaskWithoutChangingWhatIsEncoded(t *testing.T) {
	t.Parallel()
	for _, tc := range golden {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			auto, err := Encode(tc.text)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			for mask := 0; mask < 8; mask++ {
				fixed, err := encodeMask(tc.text, mask)
				if err != nil {
					t.Fatalf("encodeMask(%d): %v", mask, err)
				}
				if digest(fixed) == digest(auto) {
					return
				}
			}
			t.Fatalf("Encode produced a symbol no mask accounts for:\n%s", render(auto))
		})
	}
}

// TestVersionGrowsWithTheInput pins the boundary at every supported version,
// because a version chosen one too small truncates and a reader still decodes
// what is left, which is the failure that does not look like one.
func TestVersionGrowsWithTheInput(t *testing.T) {
	t.Parallel()
	capacities := []struct{ version, bytes int }{
		{1, 14}, {2, 26}, {3, 42}, {4, 62}, {5, 84},
		{6, 106}, {7, 122}, {8, 152}, {9, 180}, {10, 213},
	}
	for _, c := range capacities {
		got, _, err := pick(c.bytes)
		if err != nil {
			t.Fatalf("%d bytes: %v", c.bytes, err)
		}
		if got != c.version {
			t.Errorf("%d bytes chose version %d, want %d", c.bytes, got, c.version)
		}
		if c.version == maxVersion {
			continue
		}
		next, _, err := pick(c.bytes + 1)
		if err != nil {
			t.Fatalf("%d bytes: %v", c.bytes+1, err)
		}
		if next != c.version+1 {
			t.Errorf("%d bytes chose version %d, want %d", c.bytes+1, next, c.version+1)
		}
	}
}

// TestInputTooLongIsRefused keeps the refusal explicit. A symbol that held
// part of an otpauth URI would scan and enrol nothing.
func TestInputTooLongIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := Encode(strings.Repeat("x", 214)); !errors.Is(err, ErrTooLong) {
		t.Fatalf("err = %v, want ErrTooLong", err)
	}
}

// TestGeneratorPolynomials checks the Galois arithmetic against the
// coefficients ISO/IEC 18004 Appendix A publishes, so a field bug is named
// here rather than as an unreadable symbol.
func TestGeneratorPolynomials(t *testing.T) {
	t.Parallel()
	cases := map[int][]byte{
		2:  {1, 3, 2},
		5:  {1, 31, 198, 63, 147, 116},
		10: {1, 216, 194, 159, 111, 199, 94, 95, 113, 157, 193},
	}
	for degree, want := range cases {
		got := generator(degree)
		if len(got) != len(want) {
			t.Fatalf("degree %d: got %d coefficients, want %d", degree, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("degree %d: coefficient %d = %d, want %d", degree, i, got[i], want[i])
			}
		}
	}
}

// TestTerminalRenderingKeepsTheSymbolIntact reads the modules back out of the
// rendered text. Drawing is where a correct symbol becomes an unscannable
// picture, and nothing downstream of here can tell the difference.
func TestTerminalRenderingKeepsTheSymbolIntact(t *testing.T) {
	t.Parallel()
	m, err := Encode("otpauth://totp/tix:ada@example.com?secret=JBSWY3DPEHPK3PXP&issuer=tix")
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	lines := strings.Split(strings.TrimRight(m.Terminal(), "\n"), "\n")
	side := m.Size() + 2*QuietZone
	if want := (side + 1) / 2; len(lines) != want {
		t.Fatalf("rendered %d lines, want %d", len(lines), want)
	}

	// read returns the module at (x, y) of the padded grid, decoded from the
	// colour pair of the cell that carries it.
	read := func(x, y int) bool {
		line := lines[y/2]
		fields := strings.Split(line, "\x1b[")
		pair := fields[x+1]
		code, _, _ := strings.Cut(pair, "m")
		top, bottom := code == "30;40" || code == "30;107", code == "30;40" || code == "97;40"
		if y%2 == 0 {
			return top
		}
		return bottom
	}
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			inside := x >= QuietZone && y >= QuietZone && x < QuietZone+m.Size() && y < QuietZone+m.Size()
			want := inside && m[y-QuietZone][x-QuietZone]
			if got := read(x, y); got != want {
				t.Fatalf("module (%d,%d) rendered dark=%v, want %v", x, y, got, want)
			}
		}
	}
}

// TestTheQuietZoneIsDrawn keeps the margin a reader needs. A symbol flush
// against its surroundings scans as nothing at all.
func TestTheQuietZoneIsDrawn(t *testing.T) {
	t.Parallel()
	m, err := Encode("tix")
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	lines := strings.Split(strings.TrimRight(m.Terminal(), "\n"), "\n")
	for _, i := range []int{0, 1, len(lines) - 1} {
		if strings.Contains(lines[i], bothDark) || strings.Contains(lines[i], darkOnLight) ||
			strings.Contains(lines[i], lightOnDark) {
			t.Fatalf("line %d carries a dark module inside the quiet zone: %q", i, lines[i])
		}
	}
}

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

	got := reedSolomon(data, len(want))
	if len(got) != len(want) {
		t.Fatalf("got %d error correction codewords, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("codeword %d = %#02x, want %#02x (got %#v)", i, got[i], want[i], got)
		}
	}
}
