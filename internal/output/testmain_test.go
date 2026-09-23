// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"testing"

	"github.com/heliopsy/tix/internal/testenv"
)

// TestMain reports whichever environment capabilities this package's tests
// could not use, so a pass states what it actually covered.
func TestMain(m *testing.M) {
	testenv.Main(m)
}
