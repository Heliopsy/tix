-- SPDX-License-Identifier: AGPL-3.0-or-later

-- One actor's subscribable feed of due dates.
--
-- A feed is addressed by a secret in a URL, which a calendar provider fetches
-- from its own servers. Only the hash is kept, as for every other credential
-- here, and it lives apart from api_tokens so that the secret can authenticate
-- the feed and nothing else. A revoked row is kept for the audit trail; the
-- partial index allows one live feed per actor, which is what rotation and
-- revocation act on.
CREATE TABLE calendar_feeds (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  filter     TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  revoked_at TEXT
);

CREATE UNIQUE INDEX idx_calendar_feeds_live ON calendar_feeds(tenant_id, actor_id) WHERE revoked_at IS NULL;
