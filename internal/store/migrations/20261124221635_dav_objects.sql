-- SPDX-License-Identifier: AGPL-3.0-or-later

-- What CalDAV write-back keeps beside a task.
--
-- dav_objects holds the UID and resource name a client chose for a task it
-- created, because a client finds its objects again by both. Each is unique
-- in the tenant. A task tix created has no row: its UID and name are derived
-- from its identifier.
--
-- dav_object_props holds, per person and task, the properties a client sent
-- that tix has no field for (alarms, sort orders, client extensions), as the
-- content lines it sent, so they are written back into that person's own
-- object and nobody else's. It is the client's copy, never shown or read by
-- tix, and is bounded by the service at 8 KiB. Both go with the task, the
-- person or the tenant.
CREATE TABLE dav_objects (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  uid        TEXT NOT NULL,
  name       TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, task_id),
  UNIQUE (tenant_id, uid),
  UNIQUE (tenant_id, name)
);

CREATE TABLE dav_object_props (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  props      TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, actor_id, task_id)
);
CREATE INDEX idx_dav_object_props_task ON dav_object_props(tenant_id, task_id);
