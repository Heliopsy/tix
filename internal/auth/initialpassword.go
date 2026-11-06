// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"math/big"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// InitialPasswordLength is how many characters a generated password has. Over
// the alphabet below that is about 135 bits.
const InitialPasswordLength = 24

// lookAlikes are the characters a person copying by hand confuses.
const lookAlikes = "0Oo1lI"

// initialPasswordAlphabet is the letters and digits less the look-alikes, so
// a password copied by hand from a screen arrives intact. It is built from
// ranges rather than spelled out, which also keeps a high-entropy literal out
// of the source for a secret scanner to mistake for a key.
var initialPasswordAlphabet = readableAlphabet()

func readableAlphabet() string {
	var b strings.Builder
	for _, span := range [][2]byte{{'a', 'z'}, {'A', 'Z'}, {'0', '9'}} {
		for c := span[0]; c <= span[1]; c++ {
			if !strings.ContainsRune(lookAlikes, rune(c)) {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// initialPasswordGroup separates the password into groups a person can keep
// their place in. The hyphens are part of the password.
const initialPasswordGroup = 6

// NewInitialPassword returns a random password from crypto/rand, for an
// account whose password nobody chose.
func NewInitialPassword() (string, error) {
	limit := big.NewInt(int64(len(initialPasswordAlphabet)))
	var b strings.Builder
	for i := range InitialPasswordLength {
		if i > 0 && i%initialPasswordGroup == 0 {
			b.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", core.Internal("reading random bytes").Wrap(err)
		}
		b.WriteByte(initialPasswordAlphabet[n.Int64()])
	}
	return b.String(), nil
}
