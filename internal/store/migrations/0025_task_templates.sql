-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Task templates: a named shape of a task, kept apart from tasks so that no
-- listing, claim, statistic or export can ever count one as work.
--
-- body is the JSON of core.TemplateTask, the task and its subtasks. name_key
-- is the name lowercased, so two templates cannot differ only by case.
-- project_id NULL is a template the whole tenant shares; one that belongs to
-- a project goes with it. due_in is the offset in seconds a use's task falls
-- due after, 0 for none, and uses numbers the uses.
CREATE TABLE task_templates (
  id               TEXT PRIMARY KEY,
  tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name             TEXT NOT NULL,
  name_key         TEXT NOT NULL,
  project_id       TEXT REFERENCES projects(id) ON DELETE CASCADE,
  body             TEXT NOT NULL,
  due_in           INTEGER NOT NULL DEFAULT 0,
  uses             INTEGER NOT NULL DEFAULT 0,
  creator_actor_id TEXT NOT NULL DEFAULT '',
  created_at       TEXT NOT NULL,
  updated_at       TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_task_templates_name ON task_templates(tenant_id, name_key);
CREATE INDEX idx_task_templates_project ON task_templates(tenant_id, project_id);
