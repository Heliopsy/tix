-- SPDX-License-Identifier: AGPL-3.0-or-later

-- How dates are shown to a person, at the tenant and at the user.
--
-- Both levels default to the empty string, which inherits: a user with
-- nothing set reads the tenant's choice, and a tenant with nothing set reads
-- the deployment's output.time_format and output.timezone. An upgrade
-- therefore changes nothing anybody reads until someone chooses.
--
-- The user's columns sit on the global users row rather than on a tenant
-- actor, so the choice follows the person to every tenant and device.

ALTER TABLE tenants ADD COLUMN time_format TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN timezone TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN time_format TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT '';
