-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A host several tenants are served on, and sessions that say how they began.
--
-- tenant_domains.shared marks a domain whose host also serves the tenants a
-- credential or the X-Tix-Tenant header names, as a host no domain claims
-- already does. Without either, the host still resolves to the tenant that
-- owns the domain. Every domain added before this reads 0 and stays bound to
-- its tenant alone.
--
-- sessions.second_factor records whether the sign-in that minted a session
-- passed a second factor, and sessions.method how it signed in (password,
-- ssh-key or switch), so a switch to another tenant can be held to that
-- tenant's policy. Sessions minted before this read as a password sign-in
-- with no second factor, which asks for a code where one is required.
--
-- On PostgreSQL this migration also brings the read-only policy that lets the
-- shared host find the tenant a presented credential belongs to, and the one
-- that lets the membership listing read each tenant's own address (see
-- internal/store/postgres/schema.go).

ALTER TABLE tenant_domains ADD COLUMN shared INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN second_factor INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN method TEXT NOT NULL DEFAULT 'password';
