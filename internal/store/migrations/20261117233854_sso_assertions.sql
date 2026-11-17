-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The SAML assertions that have signed somebody in, kept until they expire so
-- that a captured response cannot be posted a second time. A table rather
-- than memory, so that a replica other than the one that took the response
-- refuses it too. Each write deletes the tenant's expired rows, so it holds
-- only the last few minutes of sign-ins.

CREATE TABLE sso_assertions (
  id           TEXT PRIMARY KEY,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  provider_id  TEXT NOT NULL REFERENCES sso_providers(id) ON DELETE CASCADE,
  assertion_id TEXT NOT NULL,
  expires_at   TEXT NOT NULL,
  UNIQUE (tenant_id, provider_id, assertion_id)
);
CREATE INDEX idx_sso_assertions_expiry ON sso_assertions(tenant_id, expires_at);
