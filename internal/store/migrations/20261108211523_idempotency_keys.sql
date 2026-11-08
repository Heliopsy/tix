-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A client's Idempotency-Key and what its first request answered, so a
-- retried write replays the answer instead of applying the change again.
--
-- One row per tenant, actor and key. fingerprint is the SHA-256 of the
-- method, request URI and body the key was first used with. state is
-- reserved while the request runs, executed once its write committed (the
-- mark is made in the write's own transaction) and done once the response is
-- kept in status, content_type and body. lease_expires_at bounds how long a
-- reserved or executed row counts as still running; expires_at is when the
-- key stops answering and the pruner may delete it.
CREATE TABLE idempotency_keys (
  tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id               TEXT NOT NULL,
  actor_id         TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  idem_key         TEXT NOT NULL,
  fingerprint      TEXT NOT NULL,
  state            TEXT NOT NULL,
  status           INTEGER NOT NULL DEFAULT 0,
  content_type     TEXT NOT NULL DEFAULT '',
  body             BLOB,
  created_at       TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  expires_at       TEXT NOT NULL,
  PRIMARY KEY (tenant_id, id)
);
CREATE UNIQUE INDEX idx_idempotency_keys_key ON idempotency_keys(tenant_id, actor_id, idem_key);
CREATE INDEX idx_idempotency_keys_expiry ON idempotency_keys(tenant_id, expires_at);
