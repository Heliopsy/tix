// SPDX-License-Identifier: AGPL-3.0-or-later

// Command tixload load-tests a tix server. Run it with `just load`; see
// docs/testing.md#load-testing. It is a development tool and is not released.
package main

import (
	"os"

	"github.com/heliopsy/tix/internal/loadtest"
)

func main() {
	os.Exit(loadtest.Main(os.Args[1:], os.Stdout, os.Stderr))
}
