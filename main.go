// SPDX-License-Identifier: AGPL-3.0-or-later

// Command tix is a multi-tenant task management system for humans and AI agents.
package main

import (
	// The zone database is embedded rather than read from the host, because a
	// reader's timezone is their setting and a base image without tzdata would
	// silently downgrade it to the deployment default. Costs 404 KB of stripped
	// binary, paid once, against a wrong timestamp nobody can see is wrong.
	_ "time/tzdata"

	"github.com/heliopsy/tix/cmd"
)

func main() {
	cmd.Execute()
}
