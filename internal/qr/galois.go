// SPDX-License-Identifier: AGPL-3.0-or-later

package qr

// The QR field is GF(256) modulo x^8 + x^4 + x^3 + x^2 + 1, with 2 as the
// generator, per ISO/IEC 18004 section 7.5.2.
const fieldPoly = 0x11D

var (
	expTable [512]byte
	logTable [256]byte
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		expTable[i] = byte(x)
		logTable[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= fieldPoly
		}
	}
	// The upper half repeats the cycle so a product of two logarithms can be
	// looked up without reducing the index first.
	for i := 255; i < 512; i++ {
		expTable[i] = expTable[i-255]
	}
}

// mul multiplies in GF(256).
func mul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return expTable[int(logTable[a])+int(logTable[b])]
}

// generator returns the error correction generator polynomial of degree n,
// coefficients highest power first.
func generator(n int) []byte {
	poly := []byte{1}
	for i := 0; i < n; i++ {
		next := make([]byte, len(poly)+1)
		for j, c := range poly {
			next[j] ^= c
			next[j+1] ^= mul(c, expTable[i])
		}
		poly = next
	}
	return poly
}

// reedSolomon returns the n error correction codewords for one data block.
func reedSolomon(data []byte, n int) []byte {
	gen := generator(n)
	rem := make([]byte, n)
	for _, d := range data {
		factor := d ^ rem[0]
		copy(rem, rem[1:])
		rem[n-1] = 0
		for i, g := range gen[1:] {
			rem[i] ^= mul(g, factor)
		}
	}
	return rem
}
