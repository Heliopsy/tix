-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The CalDAV sync tokens a server has answered with. A token stands for a
-- snapshot: the set of member names and entity tags a collection held when it
-- was issued, so a later sync-collection report answers the difference
-- whatever made a task enter or leave. set_hash names the set and is the
-- token; identical sets share one row per app password and collection.
-- event_seq, event_mark, config and valid_until are what the "nothing
-- changed" short cut compares: valid_until is the first moment the set could
-- change with no event, when a finished task ages out or a snooze passes.
-- A row is a cache of what was answered, not domain state: writing one
-- records no audit entry and no event.
CREATE TABLE dav_sync_states (
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  token_id    TEXT NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
  actor_id    TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  collection  TEXT NOT NULL,
  set_hash    TEXT NOT NULL,
  members     TEXT NOT NULL DEFAULT '[]',
  event_seq   INTEGER NOT NULL DEFAULT 0,
  event_mark  INTEGER NOT NULL DEFAULT 0,
  config      TEXT NOT NULL DEFAULT '',
  valid_until TEXT NOT NULL,
  seen_at     TEXT NOT NULL,
  PRIMARY KEY (tenant_id, token_id, collection, set_hash)
);
