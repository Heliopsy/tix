-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A reader's named task filter: the expression, the view and the sort the
-- task screen opens with.
--
-- Owned by an actor, like a star or a group preference, so each reader keeps
-- their own list. shared lists it for the whole tenant; only its owner or an
-- administrator changes it. sort_order is the owner's own order, from 1.
-- Names are unique per owner without regard to case, which the service
-- enforces, since an index on lower(name) is not portable to the builder.
CREATE TABLE saved_filters (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  id         TEXT NOT NULL,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  query      TEXT NOT NULL DEFAULT '',
  view_name  TEXT NOT NULL DEFAULT '',
  sort_field TEXT NOT NULL DEFAULT '',
  shared     INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, id)
);
CREATE INDEX idx_saved_filters_actor ON saved_filters(tenant_id, actor_id);
CREATE INDEX idx_saved_filters_shared ON saved_filters(tenant_id, shared);
