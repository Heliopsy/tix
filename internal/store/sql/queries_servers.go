// SPDX-License-Identifier: AGPL-3.0-or-later

package sql

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// JoinServerSurfaces renders the surfaces a server serves as the one column
// holds them. The set is closed and has four members, so a comma-joined string
// needs neither a document type nor a second scanner per engine.
//
// Order is the caller's, so a reader sees them in the order core declares them
// rather than in whatever order they were assembled.
func JoinServerSurfaces(surfaces []core.ServerSurface) string {
	parts := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		if s == "" {
			continue
		}
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ",")
}

// ParseServerSurfaces reads the column back. An empty column is no surfaces,
// which is an empty slice rather than a nil one so a caller ranging over it
// need not nil-check.
func ParseServerSurfaces(joined string) []core.ServerSurface {
	out := []core.ServerSurface{}
	for _, part := range strings.Split(joined, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, core.ServerSurface(part))
	}
	return out
}
