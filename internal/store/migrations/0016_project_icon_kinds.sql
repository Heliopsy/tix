-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A project icon can be a built-in symbol or an uploaded image, beside the
-- text icon migration 0003 added.
--
-- icon keeps its meaning, so every reader that already draws it as text goes
-- on drawing the text icons it finds and nothing for the new kinds. At most one
-- of icon, icon_name and icon_image is set; the service keeps that invariant.
-- icon_image is the uploaded image's digest, which is all a listing needs to
-- build an address for it. The bytes live in project_icons, so a listing never
-- reads them.
--
-- Existing projects get empty values: their text icon stays a text icon.

ALTER TABLE projects ADD COLUMN icon_name TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN icon_image TEXT NOT NULL DEFAULT '';

CREATE TABLE project_icons (
  project_id   TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  content_type TEXT NOT NULL,
  digest       TEXT NOT NULL,
  data         BLOB NOT NULL,
  updated_at   TEXT NOT NULL
);
CREATE INDEX idx_project_icons_tenant ON project_icons(tenant_id);
