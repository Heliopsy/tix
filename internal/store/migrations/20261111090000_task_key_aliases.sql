-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Numbers a task was known by in a project it moved out of.
--
-- Moving a task to another project gives it the target's next number, so its
-- reference changes. The old project and number are kept here so links,
-- mentions and scripts that still carry the old reference reach the task. A
-- project never reissues a number, so an alias cannot collide with a task
-- created later. It goes with the task, or with the project it names.
CREATE TABLE task_key_aliases (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  seq        BIGINT NOT NULL,
  task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, project_id, seq)
);
CREATE INDEX idx_task_key_aliases_task ON task_key_aliases(tenant_id, task_id);
