-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A tenant's second factor policy, beside the per-surface require_totp_*
-- flags rather than in place of them. The flags ask for a code at sign-in;
-- the policy refuses a person with no factor on every request, tokens they
-- own included. Folding one into the other would change what an existing
-- tenant's flags do on upgrade, so every tenant starts off.
--
-- mfa_grace is duration text as lease_default is ('' is none), and
-- mfa_required_at is when the policy last turned required: the grace runs
-- from it.

ALTER TABLE tenants ADD COLUMN mfa_policy TEXT NOT NULL DEFAULT 'off';
ALTER TABLE tenants ADD COLUMN mfa_grace TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN mfa_required_at TEXT;
