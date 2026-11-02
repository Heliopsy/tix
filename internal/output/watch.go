// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"strconv"
	"strings"
)

// WatchBanner is the line a watch prints before its first event: what it
// reached, what it is filtered on, and how to stop it. `tix watch` and the SSH
// listener's watch both print it, so the two cannot describe one stream two
// ways.
func WatchBanner(target string, projects, types, actors []string, since int64) string {
	var filters []string
	if len(projects) > 0 {
		filters = append(filters, "project="+strings.Join(projects, ","))
	}
	if len(types) > 0 {
		filters = append(filters, "type="+strings.Join(types, ","))
	}
	if len(actors) > 0 {
		filters = append(filters, "actor="+strings.Join(actors, ","))
	}
	if since > 0 {
		filters = append(filters, "since="+strconv.FormatInt(since, 10))
	}
	msg := "watching " + target
	if len(filters) > 0 {
		msg += ", filtering on " + strings.Join(filters, " ")
	}
	return msg + "; ctrl-c to stop"
}
