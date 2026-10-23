// SPDX-License-Identifier: AGPL-3.0-or-later

// Package qr encodes a short string as a QR symbol.
//
// It exists so that `tix user totp enrol` can draw a code a phone camera can
// read without the repository taking a dependency for one screen. The scope is
// the smallest thing that serves that: byte mode, error correction level M,
// versions 1 to 10, which covers any otpauth:// URI tix can produce. Anything
// outside that is an error rather than a silent downgrade, because a symbol
// that encodes less than it was given scans cleanly and enrols the wrong
// secret.
package qr

import (
	"errors"
	"fmt"
)

// Matrix is a square QR symbol, true where a module is dark. Row zero is the
// top of the symbol.
type Matrix [][]bool

// Size returns the side length in modules.
func (m Matrix) Size() int { return len(m) }

// ErrTooLong reports input no supported version can hold.
var ErrTooLong = errors.New("too long for a supported qr version")

// maxVersion is the largest symbol this encoder builds.
const maxVersion = 10

// block describes how one version splits its data into error correction
// blocks: count blocks of dataWords each.
type block struct {
	count     int
	dataWords int
}

// versionSpec is the level M geometry of one version, from ISO/IEC 18004
// tables 9 and 13.
type versionSpec struct {
	dataWords int
	ecWords   int
	blocks    []block
	// align lists the centre coordinates of the alignment patterns. The three
	// positions that collide with a finder pattern are skipped at placement.
	align []int
	// remainder is the number of zero bits the symbol carries past the last
	// codeword.
	remainder int
}

var versions = map[int]versionSpec{
	1:  {dataWords: 16, ecWords: 10, blocks: []block{{1, 16}}, align: nil, remainder: 0},
	2:  {dataWords: 28, ecWords: 16, blocks: []block{{1, 28}}, align: []int{6, 18}, remainder: 7},
	3:  {dataWords: 44, ecWords: 26, blocks: []block{{1, 44}}, align: []int{6, 22}, remainder: 7},
	4:  {dataWords: 64, ecWords: 18, blocks: []block{{2, 32}}, align: []int{6, 26}, remainder: 7},
	5:  {dataWords: 86, ecWords: 24, blocks: []block{{2, 43}}, align: []int{6, 30}, remainder: 7},
	6:  {dataWords: 108, ecWords: 16, blocks: []block{{4, 27}}, align: []int{6, 34}, remainder: 7},
	7:  {dataWords: 124, ecWords: 18, blocks: []block{{4, 31}}, align: []int{6, 22, 38}, remainder: 0},
	8:  {dataWords: 154, ecWords: 22, blocks: []block{{2, 38}, {2, 39}}, align: []int{6, 24, 42}, remainder: 0},
	9:  {dataWords: 182, ecWords: 22, blocks: []block{{3, 36}, {2, 37}}, align: []int{6, 26, 46}, remainder: 0},
	10: {dataWords: 216, ecWords: 26, blocks: []block{{4, 43}, {1, 44}}, align: []int{6, 28, 50}, remainder: 0},
}

// Encode returns the smallest level M symbol holding text, under the mask the
// penalty rules prefer.
func Encode(text string) (Matrix, error) {
	s, err := unmasked(text)
	if err != nil {
		return nil, err
	}
	return s.finish(s.bestMask()), nil
}

// encodeMask is Encode under a named mask. Mask choice is a legibility
// heuristic and not part of what a reader decodes, so every mask yields a
// valid symbol; fixing one is what lets a test compare against a reference
// encoder that chose differently.
func encodeMask(text string, mask int) (Matrix, error) {
	s, err := unmasked(text)
	if err != nil {
		return nil, err
	}
	return s.finish(mask), nil
}

// unmasked returns the symbol with its function patterns and data in place
// and no mask applied.
func unmasked(text string) (*symbol, error) {
	data := []byte(text)
	version, spec, err := pick(len(data))
	if err != nil {
		return nil, err
	}
	s := newSymbol(version, spec)
	s.reserve()
	s.place(codewords(data, version, spec), spec.remainder)
	return s, nil
}

// finish masks the data region and writes the format and version information.
func (s *symbol) finish(mask int) Matrix {
	s.applyMask(mask)
	s.writeFormat(mask)
	s.writeVersion()
	return s.modules
}

// pick chooses the smallest version whose level M capacity holds n bytes.
func pick(n int) (int, versionSpec, error) {
	for v := 1; v <= maxVersion; v++ {
		spec := versions[v]
		if n <= spec.dataWords-headerBytes(v) {
			return v, spec, nil
		}
	}
	return 0, versionSpec{}, fmt.Errorf("%d bytes: %w", n, ErrTooLong)
}

// headerBytes is the whole number of codewords the mode indicator and the
// character count consume. Byte mode counts in eight bits below version ten
// and sixteen at and above it, so the rounded-up header is two codewords
// either way; rounding up is what keeps pick conservative.
func headerBytes(version int) int {
	if version < 10 {
		return 2
	}
	return 3
}

// countBits is the width of the character count field in byte mode.
func countBits(version int) int {
	if version < 10 {
		return 8
	}
	return 16
}

// padWords alternate once the data runs out, per ISO/IEC 18004 section 7.4.10.
var padWords = [2]byte{0xEC, 0x11}

// codewords renders the data codewords and their error correction, already
// interleaved in the order the symbol reads them.
func codewords(data []byte, version int, spec versionSpec) []byte {
	var b bits
	b.add(0b0100, 4)
	b.add(uint(len(data)), countBits(version))
	for _, c := range data {
		b.add(uint(c), 8)
	}
	terminator := min(4, spec.dataWords*8-b.len())
	b.add(0, terminator)
	b.add(0, (8-b.len()%8)%8)
	for i := 0; b.len() < spec.dataWords*8; i++ {
		b.add(uint(padWords[i%2]), 8)
	}

	groups := make([][]byte, 0, len(spec.blocks))
	ecc := make([][]byte, 0, len(spec.blocks))
	at := 0
	for _, blk := range spec.blocks {
		for i := 0; i < blk.count; i++ {
			chunk := b.bytes[at : at+blk.dataWords]
			at += blk.dataWords
			groups = append(groups, chunk)
			ecc = append(ecc, reedSolomon(chunk, spec.ecWords))
		}
	}
	return interleave(groups, ecc)
}

// interleave reads one codeword from each block in turn, data first and then
// error correction, which is what spreads a burst of damage across blocks.
func interleave(data, ecc [][]byte) []byte {
	out := make([]byte, 0, 256)
	longest := 0
	for _, d := range data {
		longest = max(longest, len(d))
	}
	for i := 0; i < longest; i++ {
		for _, d := range data {
			if i < len(d) {
				out = append(out, d[i])
			}
		}
	}
	for i := 0; i < len(ecc[0]); i++ {
		for _, e := range ecc {
			out = append(out, e[i])
		}
	}
	return out
}

// bits accumulates a big-endian bit stream.
type bits struct {
	bytes []byte
	n     int
}

func (b *bits) len() int { return b.n }

func (b *bits) add(value uint, width int) {
	for i := width - 1; i >= 0; i-- {
		if b.n%8 == 0 {
			b.bytes = append(b.bytes, 0)
		}
		if value&(1<<uint(i)) != 0 {
			b.bytes[b.n/8] |= 1 << uint(7-b.n%8)
		}
		b.n++
	}
}
