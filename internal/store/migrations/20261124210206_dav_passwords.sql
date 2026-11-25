-- SPDX-License-Identifier: AGPL-3.0-or-later

-- What an app password exposes over CalDAV. An app password is an api_tokens
-- row carrying the caldav scope alone, so expiry, revocation, last use and
-- the audit trail are the token's; this table keeps only what a token has no
-- column for: the name its owner gave it, which collections it shows and
-- whether it may write. The name is unique among the owner's live app
-- passwords, which the service checks against the tokens' own revocation,
-- since a token revoked through tix token revoke does not touch this row.
-- The row goes with its token, its owner or its tenant.
CREATE TABLE dav_passwords (
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  token_id    TEXT NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
  actor_id    TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  collections TEXT NOT NULL DEFAULT 'assigned',
  read_only   INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL,
  PRIMARY KEY (tenant_id, token_id)
);
CREATE INDEX idx_dav_passwords_actor ON dav_passwords(tenant_id, actor_id);
