-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The one-time codes a native app's sign-in through a provider hands the app,
-- waiting for the app to exchange them for a session. The code itself is never
-- stored, only its SHA-256, bound to the PKCE challenge the app started with,
-- the account, the tenant and the provider. A table rather than memory, so a
-- code minted on one replica is exchanged on another and taken once across
-- all of them: the exchange deletes the row, and the delete that finds it is
-- the only one that wins. A code lives two minutes; each write deletes the
-- tenant's expired rows, so the table holds only the last few minutes.

CREATE TABLE sso_handoffs (
  id             TEXT PRIMARY KEY,
  tenant_id      TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  provider_id    TEXT NOT NULL REFERENCES sso_providers(id) ON DELETE CASCADE,
  code_hash      TEXT NOT NULL,
  code_challenge TEXT NOT NULL,
  user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  method         TEXT NOT NULL,
  second_factor  INTEGER NOT NULL DEFAULT 0,
  expires_at     TEXT NOT NULL,
  UNIQUE (tenant_id, code_hash)
);
CREATE INDEX idx_sso_handoffs_expiry ON sso_handoffs(tenant_id, expires_at);
