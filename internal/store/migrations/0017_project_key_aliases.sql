-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Keys a project was known by before it was renamed.
--
-- A task reference is rendered from the project's current key, so renaming a
-- project renames every reference at once. The old key is kept here so links,
-- filters and scripts that still carry it resolve to the same project. An alias
-- lives as long as its project: reissuing it to another project would send an
-- old link to somebody else's task.
CREATE TABLE project_key_aliases (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key        TEXT NOT NULL,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, key)
);
CREATE INDEX idx_project_key_aliases_project ON project_key_aliases(tenant_id, project_id);
