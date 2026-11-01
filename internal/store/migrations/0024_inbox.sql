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

-- A person's own notification channels: URLs their inbox items are posted
-- to, shaped for the service behind them. Separate from webhook_endpoints,
-- which belong to the tenant and carry every event; a channel carries only its
-- owner's items, of the kinds listed in kinds (a JSON array).
CREATE TABLE inbox_channels (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  url        TEXT NOT NULL,
  secret     TEXT NOT NULL,
  format     TEXT NOT NULL,
  kinds      TEXT NOT NULL,
  active     INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  UNIQUE (tenant_id, actor_id, name)
);

-- The delivery queue of the channels, drained by the webhook dispatcher with
-- the same claim, retry schedule and signature as webhook_deliveries. item_id
-- is NULL for a test message, and is not a foreign key: an item gone with its
-- task fails its delivery rather than blocking the purge.
CREATE TABLE inbox_deliveries (
  id               TEXT PRIMARY KEY,
  tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  channel_id       TEXT NOT NULL REFERENCES inbox_channels(id) ON DELETE CASCADE,
  item_id          TEXT,
  attempts         INTEGER NOT NULL DEFAULT 0,
  next_attempt_at  TEXT NOT NULL,
  locked_by        TEXT,
  locked_until     TEXT,
  status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','delivered','failed')),
  last_error       TEXT NOT NULL DEFAULT '',
  last_status_code INTEGER NOT NULL DEFAULT 0,
  created_at       TEXT NOT NULL
);
CREATE INDEX idx_inbox_deliveries_pending ON inbox_deliveries(tenant_id, status, next_attempt_at);
CREATE INDEX idx_inbox_deliveries_channel ON inbox_deliveries(tenant_id, channel_id, created_at, id);
