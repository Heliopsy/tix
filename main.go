// SPDX-License-Identifier: AGPL-3.0-or-later

// Command tix is a multi-tenant task management system for humans and AI agents.
package main

import (
	// The zone database is embedded rather than read from the host, because a
	// reader's timezone is their setting and a base image without tzdata would
	// silently downgrade it to the deployment default. Costs 404 KB of stripped
	// binary, paid once, against a wrong timestamp nobody can see is wrong.
	_ "time/tzdata"

	_ "embed"

	"github.com/heliopsy/tix/cmd"
	"github.com/heliopsy/tix/internal/changelog"
)

// changelogMD is the release-please changelog, carried so the release screen
// can say what the running version changed without asking anybody.
//
//go:embed CHANGELOG.md
var changelogMD string

func main() {
	changelog.Source = changelogMD
	cmd.Execute()
}
