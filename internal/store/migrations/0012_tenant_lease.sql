-- SPDX-License-Identifier: AGPL-3.0-or-later

-- How long a claim on a task may hold, per tenant.
--
-- Until now the only lease settings were a workflow's default_lease and
-- whatever ttl a caller passed, bounded by two constants compiled into the
-- binary: a 15 minute fallback and a 24 hour ceiling. That left an operator
-- with no way to say "no agent on this tenant may hold a task for a day".
-- A caller could always ask for 23h59m and be granted it, because nothing
-- between the claim and the constant had an opinion.
--
-- Two columns rather than a lease_policies table, following tenants.theme and
-- the require_totp_* flags: these are two scalar settings on a tenant, not a
-- subsystem with a lifecycle of its own. retention_policies earns its table by
-- being what the pruner reads on a schedule; a lease bound is read inside the
-- claim that is already loading the tenant.
--
-- Both default to the empty string, and empty means "nobody chose" exactly as
-- it does for theme. An unset value falls through to the deployment
-- configuration, and then to the built-in constant, so an existing tenant
-- upgrades into precisely the behaviour it already had. A concrete default
-- here would silently shorten every live lease on upgrade.
--
-- They hold Go duration text such as '45m' or '2h30m', the same shape
-- retention_policies stores and core.ParseDuration reads, which also accepts
-- the 'd' suffix that time.ParseDuration does not.

ALTER TABLE tenants ADD COLUMN lease_default TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN lease_max TEXT NOT NULL DEFAULT '';
