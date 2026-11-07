// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"os"
	"testing"

	"github.com/heliopsy/tix/internal/service"
)

// TestMain drops password hashing to test cost for the services these tests
// build without saying which hasher to use. Production cost is asserted in
// internal/service, where nothing lowers it.
func TestMain(m *testing.M) {
	service.UseCheapHashingInTests()
	os.Exit(m.Run())
}
