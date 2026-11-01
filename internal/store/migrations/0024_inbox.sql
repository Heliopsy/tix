-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Each actor's inbox: one row per thing that concerned them, a mention, an
-- assignment, a comment on a task they follow or a status change.
--
-- Rows are written in the transaction of the change that caused them. They
-- cascade from the recipient and the task, so a purged task or a removed
-- actor leaves nothing behind; a soft-deleted task keeps its items, so a
-- restore loses nothing. actor_id is who caused the item and is not a foreign
-- key, so removing that actor does not empty somebody else's inbox.
CREATE TABLE inbox_items (
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id           TEXT NOT NULL,
  recipient_id TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL,
  task_id      TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  actor_id     TEXT NOT NULL,
  comment_id   TEXT,
  from_status  TEXT,
  to_status    TEXT,
  excerpt      TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  read_at      TEXT,
  PRIMARY KEY (tenant_id, id)
);
CREATE INDEX idx_inbox_items_recipient ON inbox_items(tenant_id, recipient_id, created_at, id);
CREATE INDEX idx_inbox_items_unread ON inbox_items(tenant_id, recipient_id) WHERE read_at IS NULL;
CREATE INDEX idx_inbox_items_task ON inbox_items(tenant_id, task_id);

-- The kinds an actor chose not to receive. A row exists only for a muted kind.
CREATE TABLE inbox_mutes (
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id  TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  kind      TEXT NOT NULL,
  PRIMARY KEY (tenant_id, actor_id, kind)
);
