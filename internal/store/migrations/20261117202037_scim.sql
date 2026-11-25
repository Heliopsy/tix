-- SPDX-License-Identifier: AGPL-3.0-or-later

-- SCIM 2.0 provisioning. Everything an identity provider pushes lives here,
-- keyed to the tenant's actors, so the users and actors tables are not
-- changed: a person provisioned over SCIM is an ordinary member with a row in
-- scim_users beside them.
--
-- scim_config is the tenant's switch, the provisioning agent the enable step
-- creates, and when a provider last changed anything. scim_users carries the
-- provider's userName, externalId, address and name parts; user_name_key is
-- the userName lower-cased, unique per tenant. A SCIM Group is a tenant group
-- of its own, scim_groups with scim_group_members, and scim_group_roles maps
-- a group's display name, lower-cased, to the role its members are given.

CREATE TABLE scim_config (
  tenant_id        TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  enabled          INTEGER NOT NULL DEFAULT 0,
  agent_id         TEXT NOT NULL DEFAULT '',
  last_activity_at TEXT,
  updated_at       TEXT NOT NULL
);

CREATE TABLE scim_users (
  tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id      TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  user_name     TEXT NOT NULL,
  user_name_key TEXT NOT NULL,
  external_id   TEXT NOT NULL DEFAULT '',
  email         TEXT NOT NULL DEFAULT '',
  given_name    TEXT NOT NULL DEFAULT '',
  family_name   TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL,
  PRIMARY KEY (tenant_id, actor_id)
);
CREATE UNIQUE INDEX idx_scim_users_name ON scim_users(tenant_id, user_name_key);
CREATE INDEX idx_scim_users_external ON scim_users(tenant_id, external_id);
CREATE INDEX idx_scim_users_email ON scim_users(tenant_id, email);

CREATE TABLE scim_groups (
  id           TEXT PRIMARY KEY,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  display_name TEXT NOT NULL,
  name_key     TEXT NOT NULL,
  external_id  TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_scim_groups_name ON scim_groups(tenant_id, name_key);
CREATE INDEX idx_scim_groups_external ON scim_groups(tenant_id, external_id);

CREATE TABLE scim_group_members (
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  group_id  TEXT NOT NULL REFERENCES scim_groups(id) ON DELETE CASCADE,
  actor_id  TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  PRIMARY KEY (tenant_id, group_id, actor_id)
);
CREATE INDEX idx_scim_group_members_actor ON scim_group_members(tenant_id, actor_id);

CREATE TABLE scim_group_roles (
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name_key     TEXT NOT NULL,
  display_name TEXT NOT NULL,
  role         TEXT NOT NULL,
  created_at   TEXT NOT NULL,
  PRIMARY KEY (tenant_id, name_key)
);
