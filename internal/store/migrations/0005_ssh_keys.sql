-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Public keys enrolled against an actor, so the terminal interface can be
-- reached over SSH without a password.
--
-- The unique is on (tenant_id, fingerprint) and not on fingerprint alone: a
-- credential belongs inside the tenant boundary like every other credential
-- here, and one person with memberships in two tenants presents the same key
-- to both. Resolution across tenants is disambiguated by the SSH username.
CREATE TABLE ssh_keys (
  id           TEXT PRIMARY KEY,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id     TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  fingerprint  TEXT NOT NULL,
  public_key   TEXT NOT NULL,
  label        TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  last_used_at TEXT,
  revoked_at   TEXT,
  UNIQUE (tenant_id, fingerprint)
);
CREATE INDEX idx_ssh_keys_actor ON ssh_keys(tenant_id, actor_id);

-- The authentication path looks a fingerprint up across every tenant, before
-- any tenant is known. Restricting the index to live keys keeps revoked rows
-- out of the only query that runs before a connection is admitted.
CREATE INDEX idx_ssh_keys_fingerprint ON ssh_keys(fingerprint) WHERE revoked_at IS NULL;
