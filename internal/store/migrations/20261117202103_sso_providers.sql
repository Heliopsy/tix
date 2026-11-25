-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Single sign-on providers, configured per tenant, and the external identities
-- linked through them.
--
-- sso_providers holds what every kind shares as columns (enabled, auto_create,
-- default_role, trust_mfa, allowed domains, group-to-role mapping) and the
-- kind's own protocol settings as JSON in config, so a later kind such as
-- saml needs no migration. The client secret is sealed with the server's key
-- (auth.totp_key), bound to the provider, as sync credentials are.
--
-- user_identities is per tenant because providers are: the same identity
-- provider configured in two tenants is two providers. A person has at most
-- one identity per provider.
--
-- users.sso_provider and users.sso_subject, the v1 seam, were never written
-- or read. One provider and subject per global account cannot say which
-- tenant's provider vouched for it, so they go rather than being reused.
--
-- tenants.password_login is 'allowed' or 'disabled'; every tenant starts
-- allowed, so nothing changes on upgrade.

CREATE TABLE sso_providers (
  id              TEXT PRIMARY KEY,
  tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key             TEXT NOT NULL,
  kind            TEXT NOT NULL,
  display_name    TEXT NOT NULL,
  enabled         INTEGER NOT NULL DEFAULT 1,
  auto_create     INTEGER NOT NULL DEFAULT 0,
  default_role    TEXT NOT NULL DEFAULT 'member',
  trust_mfa       INTEGER NOT NULL DEFAULT 0,
  allowed_domains TEXT NOT NULL DEFAULT '[]',
  role_mapping    TEXT NOT NULL DEFAULT '{}',
  config          TEXT NOT NULL DEFAULT '{}',
  secret_sealed   TEXT NOT NULL DEFAULT '',
  secret_nonce    TEXT NOT NULL DEFAULT '',
  secret_hint     TEXT NOT NULL DEFAULT '',
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  UNIQUE (tenant_id, key)
);

CREATE TABLE user_identities (
  id            TEXT PRIMARY KEY,
  tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  provider_id   TEXT NOT NULL REFERENCES sso_providers(id) ON DELETE CASCADE,
  subject       TEXT NOT NULL,
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email         TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL,
  last_login_at TEXT,
  UNIQUE (tenant_id, provider_id, subject),
  UNIQUE (tenant_id, provider_id, user_id)
);
CREATE INDEX idx_user_identities_user ON user_identities(tenant_id, user_id);

ALTER TABLE tenants ADD COLUMN password_login TEXT NOT NULL DEFAULT 'allowed';

ALTER TABLE users DROP COLUMN sso_provider;
ALTER TABLE users DROP COLUMN sso_subject;
