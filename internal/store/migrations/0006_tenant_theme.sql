-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The theme a tenant presents itself with.
--
-- A tenant's accent was decided by hashing its key and id into one of six
-- fixed pairs. Nobody chose it, nobody could change it, and it reached only
-- the web interface: the terminal interface had an unrelated palette of its
-- own, so one tenant was teal in a browser and something else in a terminal.
--
-- This column holds a theme *name*, not a palette. The palettes are built in
-- or defined in configuration, which is where deployment-wide settings belong;
-- a per-tenant table of colours would need its own CRUD, authorization and
-- audit trail to express something an operator writes once.
--
-- The default is the empty string, and empty keeps the hash. Every existing
-- tenant has had its accent since it was created, and defaulting to one
-- concrete theme here would visibly repaint every deployment that upgrades.
-- Empty means "nobody chose", and the derived colour is a better answer to
-- that than picking for them.

ALTER TABLE tenants ADD COLUMN theme TEXT NOT NULL DEFAULT '';
