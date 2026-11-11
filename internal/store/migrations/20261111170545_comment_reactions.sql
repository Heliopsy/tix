-- SPDX-License-Identifier: AGPL-3.0-or-later

-- One actor's reaction on a comment, from the fixed set core.Reactions names.
--
-- An actor holds at most one row per reaction per comment, so the key is the
-- three of them inside the tenant. Every foreign key cascades: a hard-deleted
-- comment, and a removed actor, leave no reaction behind. A soft-deleted
-- comment keeps its rows, which no read returns while it stays deleted.

CREATE TABLE comment_reactions (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  reaction   TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, comment_id, actor_id, reaction)
);
