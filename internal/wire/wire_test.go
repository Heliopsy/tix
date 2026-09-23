// SPDX-License-Identifier: AGPL-3.0-or-later

package wire

import (
	"strings"
	"testing"
)

// Every route must sit under the versioned prefix, apart from the two health
// endpoints which deliberately do not.
func TestRoutesAreVersioned(t *testing.T) {
	unversioned := map[string]bool{RouteHealth: true, RouteReady: true}
	routes := []string{
		RouteWhoAmI, RouteLogin, RouteUsers, RouteTokens, RouteTenants, RouteDomains,
		RouteProjects, RouteWorkflows, RouteTasks, RouteTaskClaim, RouteClaimNext,
		RouteWebhooks, RouteAudit, RouteExport, RouteImport, RouteEvents, RouteSyncRun,
	}
	for _, r := range routes {
		if unversioned[r] {
			continue
		}
		if !strings.HasPrefix(r, APIPrefix) {
			t.Errorf("route %q is not under %q", r, APIPrefix)
		}
	}
}
