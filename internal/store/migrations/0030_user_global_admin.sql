-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Whether a user is a global admin, who administers every tenant rather than
-- only their own.
--
-- A column on the global users row rather than a role on a membership: roles
-- are per tenant, and this one is about the installation. It defaults off, so
-- an upgrade grants nobody anything; the first global admin is granted from a
-- shell that holds the database.

ALTER TABLE users ADD COLUMN global_admin INTEGER NOT NULL DEFAULT 0;
