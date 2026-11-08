// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// EnvAnySource is the source part of a binding that covers every source of
// its tenant.
const EnvAnySource = "*"

// EnvBindings are the tenant and source pairs an operator lets read the
// TIX_SYNC_<SOURCE>_* environment on a shared server.
type EnvBindings map[string]map[string]bool

// ParseEnvBindings reads bindings written as TENANT/SOURCE or TENANT/*.
func ParseEnvBindings(entries []string) (EnvBindings, error) {
	out := EnvBindings{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		tenant, source, ok := strings.Cut(entry, "/")
		tenant = strings.ToLower(strings.TrimSpace(tenant))
		source = strings.TrimSpace(source)
		if !ok || tenant == "" || source == "" {
			return nil, core.Invalid("sync environment binding %q must read TENANT/SOURCE or TENANT/*", entry)
		}
		if out[tenant] == nil {
			out[tenant] = map[string]bool{}
		}
		out[tenant][strings.ToLower(source)] = true
	}
	return out, nil
}

// Allows reports whether a tenant's source may read the environment.
func (b EnvBindings) Allows(tenantKey, sourceName string) bool {
	sources := b[strings.ToLower(strings.TrimSpace(tenantKey))]
	return sources[EnvAnySource] || sources[strings.ToLower(strings.TrimSpace(sourceName))]
}
