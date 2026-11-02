-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Lease length per project and per task, between the claim's own ttl and the
-- tenant's lease_default. Duration text as on tenants (0012); '' inherits, so
-- every existing project and task resolves exactly as it did before.
--
-- projects.lease_max narrows the tenant's ceiling and can never raise it.
-- Columns rather than a policy table like housekeeping_policies, because the
-- tenant level these sit under already lives on tenants.

ALTER TABLE projects ADD COLUMN lease_default TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN lease_max TEXT NOT NULL DEFAULT '';
ALTER TABLE tasks ADD COLUMN lease TEXT NOT NULL DEFAULT '';
