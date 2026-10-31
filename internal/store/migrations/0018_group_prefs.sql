-- SPDX-License-Identifier: AGPL-3.0-or-later

-- One actor's arrangement of one group on one view: a board column or a lane
-- hidden, collapsed, starred to the front or moved.
--
-- Keyed by the actor for the reason task_stars is: the arrangement belongs to
-- the reader, so two people on one board hide different columns. view_name is
-- 'board', 'lanes' or 'project:<project id>', and a lane's group_key is the
-- project id, so a project whose key changes keeps its arrangement. A row
-- exists only while it says something; the service deletes a plain one.
CREATE TABLE group_prefs (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  view_name  TEXT NOT NULL,
  group_key  TEXT NOT NULL,
  hidden     INTEGER NOT NULL DEFAULT 0,
  collapsed  INTEGER NOT NULL DEFAULT 0,
  starred    INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, actor_id, view_name, group_key)
);
