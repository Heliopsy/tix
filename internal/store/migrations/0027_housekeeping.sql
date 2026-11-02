-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Housekeeping: when finished tasks go to the trash and when the trash empties.
--
-- One row per tenant, project_id '' holding the tenant default, and one per
-- project that overrides it. Each rule is a number of days: NULL inherits
-- (on the tenant row, off), 0 is off. No row at all is off everywhere, which
-- is what every existing tenant gets.
CREATE TABLE housekeeping_policies (
  tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  project_id      TEXT NOT NULL DEFAULT '',
  done_after      INTEGER,
  cancelled_after INTEGER,
  trash_after     INTEGER,
  updated_at      TEXT NOT NULL,
  PRIMARY KEY (tenant_id, project_id)
);
