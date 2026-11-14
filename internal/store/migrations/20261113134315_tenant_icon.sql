-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A tenant carries an icon, of the same three kinds a project's can be: a text
-- icon (one emoji or a monogram), a built-in symbol, or an uploaded image.
-- At most one of icon, icon_name and icon_image is set; the service keeps
-- that invariant. icon_image is the image's digest, which is all a listing
-- needs to build an address for it.
--
-- The image bytes sit on the tenant row rather than in a table of their own,
-- because a tenant row belongs to no tenant: a person's switcher reads the
-- icon of every tenant they belong to, and a global admin sets the icon of a
-- tenant they stand outside, neither of which a tenant-scoped table admits.
-- No listing selects icon_data, so it is read only when the image is asked for.
--
-- Existing tenants get no icon, and every surface draws a monogram of the name.

ALTER TABLE tenants ADD COLUMN icon TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN icon_name TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN icon_image TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN icon_type TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN icon_data BLOB;
ALTER TABLE tenants ADD COLUMN icon_updated_at TEXT;
