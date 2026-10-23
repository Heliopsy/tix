-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A second factor enrolled against the global user, and required per tenant
-- per surface.
--
-- user_totp keys on users(id) rather than on an actor, because a second factor
-- is part of a credential set and users is where tix keeps one credential set
-- per human. Enrolment per actor would make one person scan a QR code once per
-- tenant they belong to, and would let "require TOTP" in one tenant be
-- satisfied by not enrolling there. Both tables therefore carry no tenant_id
-- and are named in the unscoped allow-list in internal/store/sql.
--
-- The secret is stored sealed, never in the clear: it has to be fed into HMAC
-- on every verification, so hashing is not available, and a plaintext column
-- would mean that reading the database file yields working second factors for
-- every enrolled user. The key lives in configuration and never here.
--
-- last_step is the high-water mark that makes a code single use. The
-- acceptance window spans three steps so that a clock drifting in either
-- direction still verifies; without this column that same window would leave
-- an observed code replayable for ninety seconds.
CREATE TABLE user_totp (
  user_id       TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret_cipher TEXT NOT NULL,
  secret_nonce  TEXT NOT NULL,
  confirmed_at  TEXT,
  last_step     INTEGER NOT NULL DEFAULT 0,
  failures      INTEGER NOT NULL DEFAULT 0,
  locked_until  TEXT,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);

-- A spent code is marked rather than deleted, so an audit entry can name which
-- one was used. The unique is on the hash within the user, so regeneration
-- cannot collide with a code the same user already holds.
CREATE TABLE user_recovery_codes (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash  TEXT NOT NULL,
  created_at TEXT NOT NULL,
  used_at    TEXT,
  UNIQUE (user_id, code_hash)
);

CREATE INDEX idx_recovery_codes_unused ON user_recovery_codes(user_id) WHERE used_at IS NULL;

-- Three independent requirements, all off, so no existing deployment changes
-- behaviour. A tenant can require a code in the browser and leave the SSH
-- board on keys alone, or the reverse.
ALTER TABLE tenants ADD COLUMN require_totp_web INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tenants ADD COLUMN require_totp_cli INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tenants ADD COLUMN require_totp_ssh INTEGER NOT NULL DEFAULT 0;
