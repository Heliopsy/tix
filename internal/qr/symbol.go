// SPDX-License-Identifier: AGPL-3.0-or-later

package qr

// symbol is a matrix under construction, alongside the map of which modules
// belong to a function pattern. Data placement, masking and the penalty score
// all have to tell the two apart, and recomputing that from coordinates at
// each step is where an off-by-one hides.
type symbol struct {
	version int
	spec    versionSpec
	size    int
	modules Matrix
	fixed   [][]bool
}

func newSymbol(version int, spec versionSpec) *symbol {
	size := version*4 + 17
	s := &symbol{version: version, spec: spec, size: size}
	s.modules = make(Matrix, size)
	s.fixed = make([][]bool, size)
	for i := range s.modules {
		s.modules[i] = make([]bool, size)
		s.fixed[i] = make([]bool, size)
	}
	return s
}

// set writes one module and marks it as a function pattern.
func (s *symbol) set(x, y int, dark bool) {
	if x < 0 || y < 0 || x >= s.size || y >= s.size {
		return
	}
	s.modules[y][x] = dark
	s.fixed[y][x] = true
}

// reserve draws every function pattern and reserves the format and version
// areas, so that what is left is exactly the data region.
func (s *symbol) reserve() {
	for _, p := range [][2]int{{0, 0}, {s.size - 7, 0}, {0, s.size - 7}} {
		s.finder(p[0], p[1])
	}
	for i := 8; i < s.size-8; i++ {
		dark := i%2 == 0
		s.set(i, 6, dark)
		s.set(6, i, dark)
	}
	s.alignment()
	s.set(8, s.size-8, true)
	s.reserveFormat()
	if s.version >= 7 {
		s.reserveVersion()
	}
}

// finder draws one finder pattern with its separator, from its top left.
func (s *symbol) finder(ox, oy int) {
	for dy := -1; dy <= 7; dy++ {
		for dx := -1; dx <= 7; dx++ {
			inner := dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4
			ring := dx == 0 || dx == 6 || dy == 0 || dy == 6
			within := dx >= 0 && dx <= 6 && dy >= 0 && dy <= 6
			s.set(ox+dx, oy+dy, within && (ring || inner))
		}
	}
}

// alignment draws the alignment patterns, skipping the three centres that
// would land on a finder pattern.
func (s *symbol) alignment() {
	last := len(s.spec.align) - 1
	for i, cy := range s.spec.align {
		for j, cx := range s.spec.align {
			if (i == 0 && j == 0) || (i == 0 && j == last) || (i == last && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					edge := dx == -2 || dx == 2 || dy == -2 || dy == 2
					s.set(cx+dx, cy+dy, edge || (dx == 0 && dy == 0))
				}
			}
		}
	}
}

// formatCells lists the two copies of each of the fifteen format bits, by
// bit index from the least significant, as (x, y) pairs. The layout is not
// derivable from a formula: the first copy skips the timing modules in row
// and column eight, which is why index eight lands at (7, 8) rather than
// where the surrounding run would put it.
func (s *symbol) formatCells() [15][2][2]int {
	var out [15][2][2]int
	for i := 0; i < 15; i++ {
		var x, y int
		switch {
		case i < 6:
			x, y = 8, i
		case i == 6:
			x, y = 8, 7
		case i == 7:
			x, y = 8, 8
		case i == 8:
			x, y = 7, 8
		default:
			x, y = 14-i, 8
		}
		out[i][0] = [2]int{x, y}
		if i < 8 {
			out[i][1] = [2]int{s.size - 1 - i, 8}
		} else {
			out[i][1] = [2]int{8, s.size - 15 + i}
		}
	}
	return out
}

func (s *symbol) reserveFormat() {
	for _, pair := range s.formatCells() {
		for _, c := range pair {
			s.set(c[0], c[1], false)
		}
	}
}

func (s *symbol) reserveVersion() {
	for i := 0; i < 18; i++ {
		x, y := i/3, s.size-11+i%3
		s.set(x, y, false)
		s.set(y, x, false)
	}
}

// place writes the codeword stream into the data region, two modules wide,
// bottom to top and back again, skipping the vertical timing column.
func (s *symbol) place(words []byte, remainder int) {
	bit := 0
	total := len(words)*8 + remainder
	upward := true
	for right := s.size - 1; right > 0; right -= 2 {
		if right == 6 {
			right--
		}
		for step := 0; step < s.size; step++ {
			y := s.size - 1 - step
			if !upward {
				y = step
			}
			for _, x := range [2]int{right, right - 1} {
				if s.fixed[y][x] {
					continue
				}
				if bit < total && bit < len(words)*8 {
					s.modules[y][x] = words[bit/8]&(1<<uint(7-bit%8)) != 0
				}
				bit++
			}
		}
		upward = !upward
	}
}

// maskAt reports whether the named mask inverts the module at (x, y), per
// ISO/IEC 18004 table 10.
func maskAt(mask, x, y int) bool {
	switch mask {
	case 0:
		return (y+x)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (y+x)%3 == 0
	case 4:
		return (y/2+x/3)%2 == 0
	case 5:
		return (y*x)%2+(y*x)%3 == 0
	case 6:
		return ((y*x)%2+(y*x)%3)%2 == 0
	default:
		return ((y+x)%2+(y*x)%3)%2 == 0
	}
}

func (s *symbol) applyMask(mask int) {
	for y := 0; y < s.size; y++ {
		for x := 0; x < s.size; x++ {
			if !s.fixed[y][x] && maskAt(mask, x, y) {
				s.modules[y][x] = !s.modules[y][x]
			}
		}
	}
}

// bestMask picks the mask with the lowest penalty, trying each in turn and
// undoing it. The format bits are written afterwards, so they take no part in
// the score; that matches the standard, which scores the masked symbol.
func (s *symbol) bestMask() int {
	best, bestScore := 0, -1
	for mask := 0; mask < 8; mask++ {
		s.applyMask(mask)
		score := s.penalty()
		s.applyMask(mask)
		if bestScore < 0 || score < bestScore {
			best, bestScore = mask, score
		}
	}
	return best
}

// penalty scores the four features ISO/IEC 18004 section 7.8.3 penalises.
func (s *symbol) penalty() int {
	return s.penaltyRuns() + s.penaltyBlocks() + s.penaltyFinderLike() + s.penaltyBalance()
}

func (s *symbol) penaltyRuns() int {
	total := 0
	for i := 0; i < s.size; i++ {
		total += runScore(s.row(i)) + runScore(s.column(i))
	}
	return total
}

func runScore(line []bool) int {
	total, run := 0, 1
	for i := 1; i < len(line); i++ {
		if line[i] == line[i-1] {
			run++
			continue
		}
		if run >= 5 {
			total += run - 2
		}
		run = 1
	}
	if run >= 5 {
		total += run - 2
	}
	return total
}

func (s *symbol) penaltyBlocks() int {
	total := 0
	for y := 0; y < s.size-1; y++ {
		for x := 0; x < s.size-1; x++ {
			v := s.modules[y][x]
			if s.modules[y][x+1] == v && s.modules[y+1][x] == v && s.modules[y+1][x+1] == v {
				total += 3
			}
		}
	}
	return total
}

// finderLike is the 1:1:3:1:1 run the finder patterns use. Four light modules
// on either side of it carry the same penalty, and a run can match at both
// ends, so both are counted rather than one.
var finderLike = []bool{true, false, true, true, true, false, true}

func (s *symbol) penaltyFinderLike() int {
	total := 0
	for i := 0; i < s.size; i++ {
		total += finderRuns(s.row(i)) + finderRuns(s.column(i))
	}
	return total
}

func finderRuns(line []bool) int {
	total := 0
	for i := 0; i+len(finderLike) <= len(line); i++ {
		if !matchAt(line, i, finderLike) {
			continue
		}
		if light(line, i-4, i) {
			total += 40
		}
		if light(line, i+len(finderLike), i+len(finderLike)+4) {
			total += 40
		}
	}
	return total
}

func matchAt(line []bool, at int, want []bool) bool {
	for i, w := range want {
		if line[at+i] != w {
			return false
		}
	}
	return true
}

// light reports whether [from, to) is inside the line and entirely light.
func light(line []bool, from, to int) bool {
	if from < 0 || to > len(line) {
		return false
	}
	for i := from; i < to; i++ {
		if line[i] {
			return false
		}
	}
	return true
}

func (s *symbol) penaltyBalance() int {
	dark := 0
	for _, row := range s.modules {
		for _, m := range row {
			if m {
				dark++
			}
		}
	}
	percent := dark * 100 / (s.size * s.size)
	deviation := percent - 50
	if deviation < 0 {
		deviation = -deviation
	}
	return deviation / 5 * 10
}

func (s *symbol) row(y int) []bool { return s.modules[y] }

func (s *symbol) column(x int) []bool {
	out := make([]bool, s.size)
	for y := 0; y < s.size; y++ {
		out[y] = s.modules[y][x]
	}
	return out
}

// formatMaskPattern is XORed into the format bits so that a symbol with level
// M and mask 0 is not fifteen light modules, per ISO/IEC 18004 section 7.9.
const formatMaskPattern = 0b101010000010010

// levelM is the two-bit error correction level indicator for level M.
const levelM = 0b00

func (s *symbol) writeFormat(mask int) {
	data := levelM<<3 | mask
	value := (data<<10 | bch(data<<10, 0b10100110111, 10)) ^ formatMaskPattern
	for i, pair := range s.formatCells() {
		dark := value&(1<<uint(i)) != 0
		for _, c := range pair {
			s.modules[c[1]][c[0]] = dark
		}
	}
}

func (s *symbol) writeVersion() {
	if s.version < 7 {
		return
	}
	value := s.version<<12 | bch(s.version<<12, 0b1111100100101, 12)
	for i := 0; i < 18; i++ {
		dark := value&(1<<uint(i)) != 0
		x, y := i/3, s.size-11+i%3
		s.modules[y][x] = dark
		s.modules[x][y] = dark
	}
}

// bch returns the remainder of value under the BCH generator, reducing while
// the value is wider than the remainder.
func bch(value, gen, width int) int {
	genWidth := bitLen(gen)
	for bitLen(value) >= genWidth {
		value ^= gen << uint(bitLen(value)-genWidth)
	}
	return value & (1<<uint(width) - 1)
}

func bitLen(v int) int {
	n := 0
	for ; v != 0; v >>= 1 {
		n++
	}
	return n
}
