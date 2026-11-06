-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Whether the next password sign-in must choose a new password before it is
-- let in. Off for every existing account, so an upgrade asks nobody anything.
ALTER TABLE users ADD COLUMN require_password_change INTEGER NOT NULL DEFAULT 0;

-- An invitation to sign in to a tenant, accepted by a link carrying a secret.
--
-- The account is created when the invite is issued, so user_id names an actor
-- of this tenant; removing that actor or the tenant removes its invites. Only
-- the secret's hash is kept. Accepted, revoked and replaced rows stay for the
-- audit trail; the partial index allows one pending link per account, which is
-- what re-issuing and revoking act on.
CREATE TABLE user_invites (
  id               TEXT PRIMARY KEY,
  tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id          TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  email            TEXT NOT NULL,
  role             TEXT NOT NULL CHECK (role IN ('viewer','member','admin')),
  invited_by       TEXT NOT NULL,
  existing_account INTEGER NOT NULL DEFAULT 0,
  token_hash       TEXT NOT NULL UNIQUE,
  created_at       TEXT NOT NULL,
  expires_at       TEXT NOT NULL,
  accepted_at      TEXT,
  revoked_at       TEXT
);

CREATE UNIQUE INDEX idx_user_invites_pending ON user_invites(tenant_id, user_id)
  WHERE accepted_at IS NULL AND revoked_at IS NULL;
