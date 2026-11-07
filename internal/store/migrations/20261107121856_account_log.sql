-- SPDX-License-Identifier: AGPL-3.0-or-later

-- One list per account of every change made to it: its password, second
-- factor, disabled state, global admin standing and memberships. Per-tenant
-- audit cannot answer "what happened to my account", because those changes
-- are recorded in whichever tenant they were made from, and the person may
-- not read the audit of every one. The person and global admins read this;
-- tenant admins never do, because it names the person's other tenants.
--
-- It has no tenant column, like users: an account is global. user_id carries
-- no foreign key, so a deleted account's history stays for global admins.
-- by_email is copied rather than joined for the same reason. detail is JSON
-- and never holds a secret: no password, hash, code or token.

CREATE TABLE account_log (
  seq             INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id         TEXT NOT NULL,
  action          TEXT NOT NULL,
  tenant_key      TEXT NOT NULL DEFAULT '',
  by_user_id      TEXT NOT NULL DEFAULT '',
  by_email        TEXT NOT NULL DEFAULT '',
  by_global_admin INTEGER NOT NULL DEFAULT 0,
  via_kind        TEXT NOT NULL DEFAULT 'unknown',
  detail          TEXT,
  occurred_at     TEXT NOT NULL
);

CREATE INDEX idx_account_log_user ON account_log(user_id, seq);
