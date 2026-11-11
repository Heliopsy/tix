-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Emoji reactions on comments. A row is one actor's one emoji on one comment,
-- so an actor can leave several different emoji but each only once. Deleting a
-- comment is a soft delete, so the service clears its reactions itself; the
-- cascades cover a hard-deleted comment, task or actor.
CREATE TABLE comment_reactions (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  emoji      TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, comment_id, actor_id, emoji)
);
CREATE INDEX idx_comment_reactions_comment ON comment_reactions(tenant_id, comment_id, created_at);
