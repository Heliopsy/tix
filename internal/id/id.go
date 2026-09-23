// SPDX-License-Identifier: AGPL-3.0-or-later

// Package id generates sortable unique identifiers.
package id

import (
	"crypto/rand"
	"encoding/binary"
	"strings"
	"sync"
	"time"
)

// Length is the number of characters in a generated identifier.
const Length = 26

// crockford is Crockford base32, omitting the ambiguous I, L, O and U.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Generator produces identifiers.
type Generator interface {
	New() string
	NewAt(t time.Time) string
}

// Clock is the time source a generator timestamps identifiers from. It is
// satisfied by clock.Clock, so a service under a fake clock can hand its own
// clock here and keep event identifiers agreeing with OccurredAt.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

// Now returns the current UTC time.
func (systemClock) Now() time.Time { return time.Now().UTC() }

// NewGenerator returns a generator reading the time from c. A nil clock reads
// the system clock.
func NewGenerator(c Clock) Generator {
	if c == nil {
		c = systemClock{}
	}
	return &generator{clock: c}
}

// Default is the package-level generator.
var Default Generator = NewGenerator(nil)

// New returns a new identifier from the default generator.
func New() string { return Default.New() }

// NewAt returns a new identifier timestamped at t.
func NewAt(t time.Time) string { return Default.NewAt(t) }

type generator struct {
	clock    Clock
	mu       sync.Mutex
	lastMS   uint64
	lastRand [10]byte
}

// New returns an identifier timestamped by the generator's clock.
func (g *generator) New() string { return g.NewAt(g.clock.Now()) }

// NewAt returns an identifier timestamped at t.
func (g *generator) NewAt(t time.Time) string {
	ms := uint64(t.UnixMilli())

	g.mu.Lock()
	var entropy [10]byte
	if ms == g.lastMS {
		entropy = g.lastRand
		if incr(&entropy) {
			ms++
			entropy = randomEntropy()
		}
	} else {
		entropy = randomEntropy()
	}
	g.lastMS = ms
	g.lastRand = entropy
	g.mu.Unlock()

	var raw [16]byte
	binary.BigEndian.PutUint64(raw[0:8], ms<<16)
	copy(raw[6:], entropy[:])

	return encode(raw)
}

// incr adds one to the entropy, treated as a big-endian integer. It reports
// whether the addition wrapped, which is the one case where the next
// identifier of a millisecond would not sort after the last.
func incr(b *[10]byte) bool {
	for i := len(b) - 1; i >= 0; i-- {
		b[i]++
		if b[i] != 0 {
			return false
		}
	}
	return true
}

// randomEntropy draws the ten random bytes an identifier carries.
func randomEntropy() [10]byte {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("id: crypto/rand failed: " + err.Error())
	}
	return b
}

// encode renders 16 bytes as 26 Crockford base32 characters.
func encode(raw [16]byte) string {
	var out [Length]byte
	out[0] = crockford[(raw[0]&0xE0)>>5]
	out[1] = crockford[raw[0]&0x1F]

	var bits, acc uint32
	idx := 2
	for _, b := range raw[1:] {
		acc = acc<<8 | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[idx] = crockford[(acc>>bits)&0x1F]
			idx++
		}
	}
	if bits > 0 {
		out[idx] = crockford[(acc<<(5-bits))&0x1F]
	}
	return string(out[:])
}

// Valid reports whether s is a well-formed identifier.
func Valid(s string) bool {
	if len(s) != Length {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune(crockford, upper(c)) {
			return false
		}
	}
	return true
}

func upper(c rune) rune {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// Fixed returns a predetermined sequence, for reproducible tests.
type Fixed struct {
	mu     sync.Mutex
	Values []string
	next   int
}

// NewFixed returns a generator cycling through values.
func NewFixed(values ...string) *Fixed { return &Fixed{Values: values} }

// New returns the next value in the sequence, cycling when exhausted.
func (f *Fixed) New() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Values) == 0 {
		return ""
	}
	v := f.Values[f.next%len(f.Values)]
	f.next++
	return v
}

// NewAt ignores t and returns the next value in the sequence.
func (f *Fixed) NewAt(time.Time) string { return f.New() }

var (
	_ Generator = (*generator)(nil)
	_ Generator = (*Fixed)(nil)
)
