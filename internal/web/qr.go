// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// A QR encoder, byte mode, error correction level M, versions 1 to 10. ISO/IEC
// 18004.
//
// It is here rather than in a dependency because the only thing the browser
// has to encode is one otpauth:// URI, and because the alternative was a
// client-side library: a script that would need the secret handed to it in
// the page to draw anything. Drawing the modules on the server keeps the
// enrolment secret out of the script context and keeps the screen working
// with scripting off, which the sign-in path requires anyway.
//
// Level M and ten versions are what that one job needs: an otpauth URI runs
// to about 120 characters and version 10 holds 216 bytes.

// qrLevel is the error correction level every code here is drawn at. Its
// format-information bits are 0, which is where the constant is used.
const qrFormatBitsM = 0

// qrSpec is one version's capacity and its alignment pattern centres.
type qrSpec struct {
	// total is every codeword, data and error correction together.
	total int
	// ec is the error correction codewords in each block.
	ec int
	// blocks is how many blocks the data is split into. A remainder is spread
	// over the last blocks, which then hold one data codeword more.
	blocks int
	align  []int
}

// qrSpecs are versions 1 to 10 at level M.
var qrSpecs = []qrSpec{
	{total: 26, ec: 10, blocks: 1},
	{total: 44, ec: 16, blocks: 1, align: []int{6, 18}},
	{total: 70, ec: 26, blocks: 1, align: []int{6, 22}},
	{total: 100, ec: 18, blocks: 2, align: []int{6, 26}},
	{total: 134, ec: 24, blocks: 2, align: []int{6, 30}},
	{total: 172, ec: 16, blocks: 4, align: []int{6, 34}},
	{total: 196, ec: 18, blocks: 4, align: []int{6, 22, 38}},
	{total: 242, ec: 22, blocks: 4, align: []int{6, 24, 42}},
	{total: 292, ec: 22, blocks: 5, align: []int{6, 26, 46}},
	{total: 346, ec: 26, blocks: 5, align: []int{6, 28, 50}},
}

// dataCodewords is how many of the version's codewords carry data.
func (s qrSpec) dataCodewords() int { return s.total - s.ec*s.blocks }

// qrCode is one encoded symbol, row-major, true where a module is dark.
type qrCode struct {
	size    int
	dark    []bool
	reserve []bool
}

// newQRCode encodes text as a byte-mode symbol at level M.
func newQRCode(text string) (*qrCode, error) {
	version, spec, err := qrVersionFor(len(text))
	if err != nil {
		return nil, err
	}
	codewords := qrInterleave(spec, qrPayload(version, spec, []byte(text)))

	size := 17 + 4*version
	q := &qrCode{size: size, dark: make([]bool, size*size), reserve: make([]bool, size*size)}
	q.drawFunctionPatterns(version, spec)
	q.drawCodewords(codewords)
	q.applyBestMask()
	return q, nil
}

// qrVersionFor picks the smallest version holding n bytes.
func qrVersionFor(n int) (int, qrSpec, error) {
	for i, spec := range qrSpecs {
		version := i + 1
		if 4+qrCountBits(version)+8*n <= 8*spec.dataCodewords() {
			return version, spec, nil
		}
	}
	return 0, qrSpec{}, core.Invalid("%d bytes is more than a qr code of this size can carry", n)
}

// qrCountBits is the width of the character count field in byte mode.
func qrCountBits(version int) int {
	if version < 10 {
		return 8
	}
	return 16
}

// qrPayload builds the data codewords: the mode indicator, the length, the
// bytes, the terminator and the alternating pad.
func qrPayload(version int, spec qrSpec, data []byte) []byte {
	var bits qrBits
	bits.push(0b0100, 4)
	bits.push(uint(len(data)), qrCountBits(version))
	for _, b := range data {
		bits.push(uint(b), 8)
	}
	capacity := 8 * spec.dataCodewords()
	bits.push(0, min(4, capacity-bits.len()))
	bits.push(0, (8-bits.len()%8)%8)
	out := bits.bytes()
	for i := 0; len(out) < spec.dataCodewords(); i++ {
		if i%2 == 0 {
			out = append(out, 0xEC)
			continue
		}
		out = append(out, 0x11)
	}
	return out
}

// qrInterleave splits the data into blocks, appends each block's error
// correction codewords and reads them back column by column, which is the
// order the symbol is written in.
func qrInterleave(spec qrSpec, data []byte) []byte {
	short := spec.dataCodewords() / spec.blocks
	long := spec.blocks - spec.dataCodewords()%spec.blocks

	blocks := make([][]byte, 0, spec.blocks)
	ecs := make([][]byte, 0, spec.blocks)
	for i, start := 0, 0; i < spec.blocks; i++ {
		size := short
		if i >= long {
			size++
		}
		block := data[start : start+size]
		start += size
		blocks = append(blocks, block)
		ecs = append(ecs, qrErrorCorrection(block, spec.ec))
	}

	out := make([]byte, 0, spec.total)
	for i := 0; i <= short; i++ {
		for _, block := range blocks {
			if i < len(block) {
				out = append(out, block[i])
			}
		}
	}
	for i := range spec.ec {
		for _, ec := range ecs {
			out = append(out, ec[i])
		}
	}
	return out
}

// qrBits is a bit stream, most significant bit first.
type qrBits struct{ bits []bool }

func (b *qrBits) len() int { return len(b.bits) }

func (b *qrBits) push(value uint, width int) {
	for i := width - 1; i >= 0; i-- {
		b.bits = append(b.bits, value>>uint(i)&1 == 1)
	}
}

func (b *qrBits) bytes() []byte {
	out := make([]byte, (len(b.bits)+7)/8)
	for i, bit := range b.bits {
		if bit {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}
	return out
}

// qrErrorCorrection is the Reed-Solomon remainder of one block over GF(256).
func qrErrorCorrection(data []byte, count int) []byte {
	divisor := qrGenerator(count)
	remainder := make([]byte, count)
	for _, b := range data {
		factor := b ^ remainder[0]
		copy(remainder, remainder[1:])
		remainder[count-1] = 0
		for i, d := range divisor {
			remainder[i] ^= qrMultiply(d, factor)
		}
	}
	return remainder
}

// qrGenerator is the generator polynomial of degree count, without its
// leading term.
func qrGenerator(count int) []byte {
	out := make([]byte, count)
	out[count-1] = 1
	root := byte(1)
	for range count {
		for i := range out {
			out[i] = qrMultiply(out[i], root)
			if i+1 < count {
				out[i] ^= out[i+1]
			}
		}
		root = qrMultiply(root, 2)
	}
	return out
}

// qrMultiply multiplies in GF(256) modulo the QR primitive polynomial.
func qrMultiply(a, b byte) byte {
	var out byte
	for i := 7; i >= 0; i-- {
		carry := out&0x80 != 0
		out <<= 1
		// The primitive polynomial is 0x11D; its x**8 term is the bit the
		// shift above just dropped, so only the low byte is folded back in.
		if carry {
			out ^= 0x1D
		}
		if b>>uint(i)&1 == 1 {
			out ^= a
		}
	}
	return out
}

// at reports whether a module is dark, false outside the symbol.
func (q *qrCode) at(x, y int) bool {
	if x < 0 || y < 0 || x >= q.size || y >= q.size {
		return false
	}
	return q.dark[y*q.size+x]
}

// set writes one module.
func (q *qrCode) set(x, y int, dark bool) {
	if x < 0 || y < 0 || x >= q.size || y >= q.size {
		return
	}
	q.dark[y*q.size+x] = dark
}

// setFunction writes one module and marks it as carrying no data.
func (q *qrCode) setFunction(x, y int, dark bool) {
	if x < 0 || y < 0 || x >= q.size || y >= q.size {
		return
	}
	q.dark[y*q.size+x] = dark
	q.reserve[y*q.size+x] = true
}

// reserved reports whether a module belongs to a function pattern.
func (q *qrCode) reserved(x, y int) bool { return q.reserve[y*q.size+x] }

// drawFunctionPatterns lays down everything that is not data: the finders and
// their separators, the timing lines, the alignment patterns, the reserved
// format area and, from version 7, the version information.
func (q *qrCode) drawFunctionPatterns(version int, spec qrSpec) {
	for i := range q.size {
		q.setFunction(6, i, i%2 == 0)
		q.setFunction(i, 6, i%2 == 0)
	}
	q.drawFinder(3, 3)
	q.drawFinder(q.size-4, 3)
	q.drawFinder(3, q.size-4)

	for i, x := range spec.align {
		for j, y := range spec.align {
			corner := (i == 0 && j == 0) ||
				(i == 0 && j == len(spec.align)-1) ||
				(i == len(spec.align)-1 && j == 0)
			if !corner {
				q.drawAlignment(x, y)
			}
		}
	}

	// The format area is reserved before the data is placed and written once
	// the mask is chosen, so the mask evaluation never reads a stale value.
	q.drawFormat(0)
	if version >= 7 {
		q.drawVersion(version)
	}
}

// drawFinder draws one 7 by 7 finder pattern and its separator.
func (q *qrCode) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := max(abs(dx), abs(dy))
			q.setFunction(cx+dx, cy+dy, dist != 2 && dist != 4)
		}
	}
}

// drawAlignment draws one 5 by 5 alignment pattern.
func (q *qrCode) drawAlignment(cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			q.setFunction(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

// drawFormat writes the two copies of the format information for one mask.
func (q *qrCode) drawFormat(mask int) {
	data := qrFormatBitsM<<3 | mask
	rem := data
	for range 10 {
		rem = rem<<1 ^ (rem >> 9 * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412

	for i := range 6 {
		q.setFunction(8, i, qrBit(bits, i))
	}
	q.setFunction(8, 7, qrBit(bits, 6))
	q.setFunction(8, 8, qrBit(bits, 7))
	q.setFunction(7, 8, qrBit(bits, 8))
	for i := 9; i < 15; i++ {
		q.setFunction(14-i, 8, qrBit(bits, i))
	}
	for i := range 8 {
		q.setFunction(q.size-1-i, 8, qrBit(bits, i))
	}
	for i := 8; i < 15; i++ {
		q.setFunction(8, q.size-15+i, qrBit(bits, i))
	}
	// The one module that is always dark, beside the lower left finder.
	q.setFunction(8, q.size-8, true)
}

// drawVersion writes the two copies of the version information.
func (q *qrCode) drawVersion(version int) {
	rem := version
	for range 12 {
		rem = rem<<1 ^ (rem >> 11 * 0x1F25)
	}
	bits := version<<12 | rem
	for i := range 18 {
		a, b := q.size-11+i%3, i/3
		q.setFunction(a, b, qrBit(bits, i))
		q.setFunction(b, a, qrBit(bits, i))
	}
}

// qrBit reads one bit of a value, counting from the least significant.
func qrBit(value, i int) bool { return value>>uint(i)&1 == 1 }

// drawCodewords walks the two-module columns from the bottom right, upwards
// and downwards in turn, skipping the function patterns.
func (q *qrCode) drawCodewords(codewords []byte) {
	i := 0
	for right := q.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := range q.size {
			for j := range 2 {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = q.size - 1 - vert
				}
				if q.reserved(x, y) || i >= len(codewords)*8 {
					continue
				}
				q.set(x, y, qrBit(int(codewords[i/8]), 7-i%8))
				i++
			}
		}
	}
}

// applyBestMask tries all eight masks and keeps the one the standard's
// penalty rules score lowest.
func (q *qrCode) applyBestMask() {
	best, bestScore := 0, -1
	for mask := range 8 {
		q.applyMask(mask)
		q.drawFormat(mask)
		if score := q.penalty(); bestScore < 0 || score < bestScore {
			best, bestScore = mask, score
		}
		q.applyMask(mask)
	}
	q.applyMask(best)
	q.drawFormat(best)
}

// applyMask flips the data modules the mask selects. It is its own inverse.
func (q *qrCode) applyMask(mask int) {
	for y := range q.size {
		for x := range q.size {
			if q.reserved(x, y) || !qrMasked(mask, x, y) {
				continue
			}
			q.set(x, y, !q.at(x, y))
		}
	}
}

// qrMasked reports whether the mask covers one module.
func qrMasked(mask, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (y/2+x/3)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	default:
		return ((x+y)%2+x*y%3)%2 == 0
	}
}

// penalty scores a masked symbol by the four rules in the standard: long
// runs, solid blocks, the finder-like pattern, and an unbalanced proportion
// of dark modules.
func (q *qrCode) penalty() int {
	score := 0
	for i := range q.size {
		score += q.runPenalty(i, true) + q.runPenalty(i, false)
	}
	for y := range q.size - 1 {
		for x := range q.size - 1 {
			if q.at(x, y) == q.at(x+1, y) && q.at(x, y) == q.at(x, y+1) &&
				q.at(x, y) == q.at(x+1, y+1) {
				score += 3
			}
		}
	}
	dark := 0
	for _, d := range q.dark {
		if d {
			dark++
		}
	}
	percent := dark * 100 / (q.size * q.size)
	score += 10 * (abs(percent-50) / 5)
	return score
}

// runPenalty scores one row or column for long runs and for the finder-like
// pattern that confuses a scanner looking for the real one.
func (q *qrCode) runPenalty(index int, row bool) int {
	line := make([]bool, q.size)
	for i := range q.size {
		if row {
			line[i] = q.at(i, index)
			continue
		}
		line[i] = q.at(index, i)
	}

	score, run := 0, 1
	for i := 1; i < len(line); i++ {
		if line[i] == line[i-1] {
			run++
			continue
		}
		if run >= 5 {
			score += run - 2
		}
		run = 1
	}
	if run >= 5 {
		score += run - 2
	}

	for i := range line {
		if qrFinderLike(line, i) {
			score += 40
		}
	}
	return score
}

// qrFinderLike reports whether the eleven modules at i are the 1:1:3:1:1
// pattern with four light modules on either side.
func qrFinderLike(line []bool, i int) bool {
	pattern := []bool{true, false, true, true, true, false, true}
	if i+len(pattern) > len(line) {
		return false
	}
	for j, want := range pattern {
		if line[i+j] != want {
			return false
		}
	}
	return qrLightRun(line, i-4, i) || qrLightRun(line, i+len(pattern), i+len(pattern)+4)
}

// qrLightRun reports whether every module between from and to is light,
// treating the quiet zone outside the symbol as light.
func qrLightRun(line []bool, from, to int) bool {
	if from < 0 && to > len(line) {
		return false
	}
	for i := max(from, 0); i < min(to, len(line)); i++ {
		if line[i] {
			return false
		}
	}
	return true
}

// abs is the absolute value of an int.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// qrQuietZone is the light border a scanner needs around the symbol, in
// modules. Four is what the standard asks for.
const qrQuietZone = 4

// qrSVG draws text as an inline SVG QR code, scaled by the stylesheet rather
// than by a pixel size here. The modules are one path of unit squares, so the
// markup stays small enough to sit in a page: a version 6 symbol is about
// four kilobytes.
func qrSVG(text, label string) (template.HTML, error) {
	code, err := newQRCode(text)
	if err != nil {
		return "", err
	}
	span := code.size + 2*qrQuietZone

	var path strings.Builder
	for y := range code.size {
		for x := range code.size {
			if !code.at(x, y) {
				continue
			}
			path.WriteString("M")
			path.WriteString(strconv.Itoa(x + qrQuietZone))
			path.WriteString(" ")
			path.WriteString(strconv.Itoa(y + qrQuietZone))
			path.WriteString("h1v1h-1z")
		}
	}

	var out strings.Builder
	out.WriteString(`<svg class="qr" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	out.WriteString(strconv.Itoa(span))
	out.WriteString(" ")
	out.WriteString(strconv.Itoa(span))
	out.WriteString(`" role="img" aria-label="`)
	out.WriteString(template.HTMLEscapeString(label))
	out.WriteString(`"><rect width="`)
	out.WriteString(strconv.Itoa(span))
	out.WriteString(`" height="`)
	out.WriteString(strconv.Itoa(span))
	out.WriteString(`" fill="#ffffff"/><path fill="#000000" d="`)
	out.WriteString(path.String())
	out.WriteString(`"/></svg>`)
	// #nosec G203 -- every part of this markup is generated here: the module
	// path from the encoder, and the only caller-derived value, the label,
	// escaped above.
	return template.HTML(out.String()), nil
}
