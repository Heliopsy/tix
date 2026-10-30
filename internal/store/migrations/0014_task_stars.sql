-- SPDX-License-Identifier: AGPL-3.0-or-later

-- One actor's personal star on a task.
--
-- A star belongs to the reader rather than to the task, so two people sharing
-- a queue star different things and neither reorders the other's listing. The
-- key is the actor because the actor is who the service already resolved as
-- the caller, on every surface.
--
-- Both foreign keys cascade: a hard-deleted task, and a removed actor, leave
-- no star behind. A soft delete keeps the row, so a restore is a full undo.
CREATE TABLE task_stars (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, actor_id, task_id)
);
CREATE INDEX idx_task_stars_task ON task_stars(tenant_id, task_id);
