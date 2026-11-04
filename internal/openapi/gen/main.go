// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gen writes api/openapi.yaml and the routes table of docs/api.md
// from the capability registry. Run it with `just openapi`.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/heliopsy/tix/internal/openapi"
)

func main() {
	root := flag.String("root", ".", "module root")
	flag.Parse()
	if err := openapi.Write(*root); err != nil {
		fmt.Fprintln(os.Stderr, "openapi:", err)
		os.Exit(1)
	}
}
