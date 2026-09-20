-- Initial schema.
--
-- Portable subset: TEXT timestamps in RFC3339 UTC so lexicographic comparison
-- equals chronological comparison on SQLite, INTEGER booleans, JSON in TEXT.
-- Postgres-specific types, partitioning and row-level security are applied by
-- internal/store/postgres on top of this.
--
-- tenant_id leads every composite index. Isolation is enforced by the scoped
-- query builder, by lint, and on Postgres by row-level security.

CREATE TABLE tenants (
  id         TEXT PRIMARY KEY,
  key        TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);

CREATE TABLE tenant_domains (
  id          TEXT PRIMARY KEY,
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  hostname    TEXT NOT NULL UNIQUE,
  verified_at TEXT,
  cert_mode   TEXT NOT NULL DEFAULT 'none',
  cert_path   TEXT NOT NULL DEFAULT '',
  key_path    TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL
);
CREATE INDEX idx_domains_tenant ON tenant_domains(tenant_id);

CREATE TABLE users (
  id            TEXT PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL DEFAULT '',
  display_name  TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL,
  disabled_at   TEXT,
  sso_provider  TEXT NOT NULL DEFAULT '',
  sso_subject   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE actors (
  id           TEXT PRIMARY KEY,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id      TEXT REFERENCES users(id) ON DELETE SET NULL,
  kind         TEXT NOT NULL CHECK (kind IN ('user','agent','system')),
  handle       TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  disabled_at  TEXT,
  UNIQUE (tenant_id, handle)
);

CREATE TABLE tenant_members (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  role       TEXT NOT NULL CHECK (role IN ('viewer','member','admin')),
  created_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, actor_id)
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id   TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_sessions_expiry ON sessions(expires_at);

CREATE TABLE workflows (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key        TEXT NOT NULL,
  name       TEXT NOT NULL,
  definition TEXT NOT NULL,
  builtin    INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (tenant_id, key)
);

CREATE TABLE projects (
  id          TEXT PRIMARY KEY,
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key         TEXT NOT NULL,
  name        TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  workflow_id TEXT NOT NULL REFERENCES workflows(id),
  archived_at TEXT,
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL,
  UNIQUE (tenant_id, key)
);

CREATE TABLE api_tokens (
  id           TEXT PRIMARY KEY,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id     TEXT NOT NULL REFERENCES actors(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  token_hash   TEXT NOT NULL UNIQUE,
  scopes       TEXT NOT NULL DEFAULT '[]',
  project_id   TEXT REFERENCES projects(id) ON DELETE CASCADE,
  created_at   TEXT NOT NULL,
  expires_at   TEXT,
  last_used_at TEXT,
  revoked_at   TEXT
);
CREATE INDEX idx_tokens_actor ON api_tokens(tenant_id, actor_id);

CREATE TABLE field_defs (
  id            TEXT PRIMARY KEY,
  tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key           TEXT NOT NULL,
  label         TEXT NOT NULL,
  type          TEXT NOT NULL CHECK (type IN
                  ('string','text','int','float','bool','date','datetime','enum','actor','json')),
  required      INTEGER NOT NULL DEFAULT 0,
  enum_options  TEXT,
  default_value TEXT,
  indexed       INTEGER NOT NULL DEFAULT 0,
  position      INTEGER NOT NULL DEFAULT 0,
  UNIQUE (project_id, key)
);

CREATE TABLE tasks (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  seq        INTEGER NOT NULL,
  parent_id  TEXT REFERENCES tasks(id) ON DELETE SET NULL,

  title    TEXT NOT NULL,
  body     TEXT NOT NULL DEFAULT '',
  status   TEXT NOT NULL,
  priority INTEGER NOT NULL DEFAULT 3 CHECK (priority BETWEEN 1 AND 5),

  assignee_actor_id TEXT REFERENCES actors(id) ON DELETE SET NULL,
  creator_actor_id  TEXT NOT NULL REFERENCES actors(id),

  due_at       TEXT,
  started_at   TEXT,
  completed_at TEXT,

  -- A lease whose expiry has passed reads as unclaimed everywhere, so
  -- correctness never depends on the sweeper having run.
  claimed_by_actor_id TEXT REFERENCES actors(id) ON DELETE SET NULL,
  claimed_at          TEXT,
  lease_expires_at    TEXT,
  lease_token         TEXT,
  claim_count         INTEGER NOT NULL DEFAULT 0,

  custom_fields TEXT NOT NULL DEFAULT '{}',
  version       INTEGER NOT NULL DEFAULT 1,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL,
  deleted_at    TEXT,

  UNIQUE (project_id, seq)
);
CREATE INDEX idx_tasks_project_status ON tasks(tenant_id, project_id, status, deleted_at);
CREATE INDEX idx_tasks_assignee       ON tasks(tenant_id, assignee_actor_id, deleted_at);
CREATE INDEX idx_tasks_lease          ON tasks(tenant_id, lease_expires_at);
CREATE INDEX idx_tasks_parent         ON tasks(tenant_id, parent_id);
CREATE INDEX idx_tasks_created        ON tasks(tenant_id, created_at, id);
CREATE INDEX idx_tasks_priority       ON tasks(tenant_id, priority, id);
CREATE INDEX idx_tasks_due            ON tasks(tenant_id, due_at, id);

CREATE TABLE task_deps (
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  depends_on TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY (task_id, depends_on),
  CHECK (task_id <> depends_on)
);
CREATE INDEX idx_deps_depends_on ON task_deps(tenant_id, depends_on);

CREATE TABLE labels (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT '',
  UNIQUE (tenant_id, project_id, name)
);

CREATE TABLE task_labels (
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  task_id   TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  label_id  TEXT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
  PRIMARY KEY (task_id, label_id)
);
CREATE INDEX idx_task_labels_label ON task_labels(tenant_id, label_id);

CREATE TABLE comments (
  id              TEXT PRIMARY KEY,
  tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  author_actor_id TEXT NOT NULL REFERENCES actors(id),
  body            TEXT NOT NULL,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  deleted_at      TEXT
);
CREATE INDEX idx_comments_task ON comments(tenant_id, task_id, created_at);

CREATE TABLE artifacts (
  id           TEXT PRIMARY KEY,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  task_id      TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  actor_id     TEXT NOT NULL REFERENCES actors(id),
  kind         TEXT NOT NULL,
  name         TEXT NOT NULL DEFAULT '',
  payload      TEXT NOT NULL DEFAULT '{}',
  content_type TEXT NOT NULL DEFAULT 'application/json',
  blob         BLOB,
  created_at   TEXT NOT NULL
);
CREATE INDEX idx_artifacts_task ON artifacts(tenant_id, task_id, created_at);

-- The outbox. Written in the same transaction as the rows it describes, which
-- is what lets a command-line write with no server running still reach every
-- connected subscriber.
CREATE TABLE events (
  seq          INTEGER PRIMARY KEY AUTOINCREMENT,
  id           TEXT NOT NULL UNIQUE,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  type         TEXT NOT NULL,
  project_id   TEXT,
  subject_type TEXT NOT NULL,
  subject_id   TEXT NOT NULL,
  actor_id     TEXT,
  payload      TEXT NOT NULL DEFAULT '{}',
  occurred_at  TEXT NOT NULL
);
CREATE INDEX idx_events_tenant_seq ON events(tenant_id, seq);
CREATE INDEX idx_events_project    ON events(tenant_id, project_id, seq);
CREATE INDEX idx_events_occurred   ON events(occurred_at);

CREATE TABLE audit_entries (
  seq          INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  actor_id     TEXT,
  action       TEXT NOT NULL,
  subject_type TEXT NOT NULL,
  subject_id   TEXT NOT NULL,
  before_state TEXT,
  after_state  TEXT,
  source       TEXT NOT NULL,
  occurred_at  TEXT NOT NULL
);
CREATE INDEX idx_audit_subject  ON audit_entries(tenant_id, subject_type, subject_id, seq);
CREATE INDEX idx_audit_actor    ON audit_entries(tenant_id, actor_id, seq);
CREATE INDEX idx_audit_occurred ON audit_entries(occurred_at);

CREATE TABLE webhook_endpoints (
  id          TEXT PRIMARY KEY,
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  url         TEXT NOT NULL,
  secret      TEXT NOT NULL,
  event_types TEXT NOT NULL DEFAULT '["*"]',
  active      INTEGER NOT NULL DEFAULT 1,
  created_at  TEXT NOT NULL
);
CREATE INDEX idx_webhooks_tenant ON webhook_endpoints(tenant_id, active);

CREATE TABLE webhook_deliveries (
  id               TEXT PRIMARY KEY,
  tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  endpoint_id      TEXT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
  event_seq        INTEGER NOT NULL,
  attempts         INTEGER NOT NULL DEFAULT 0,
  next_attempt_at  TEXT NOT NULL,
  locked_by        TEXT,
  locked_until     TEXT,
  status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','delivered','failed')),
  last_error       TEXT NOT NULL DEFAULT '',
  last_status_code INTEGER NOT NULL DEFAULT 0,
  created_at       TEXT NOT NULL,
  UNIQUE (endpoint_id, event_seq)
);
CREATE INDEX idx_deliveries_pending ON webhook_deliveries(status, next_attempt_at);

CREATE TABLE retention_policies (
  tenant_id          TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  events             TEXT NOT NULL,
  audit_entries      TEXT NOT NULL,
  webhook_deliveries TEXT NOT NULL
);

-- Ties a tix entity to its counterpart in an external system. This is what
-- makes a re-import an update rather than a duplicate.
CREATE TABLE external_refs (
  tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  entity_type      TEXT NOT NULL,
  entity_id        TEXT NOT NULL,
  system           TEXT NOT NULL,
  external_id      TEXT NOT NULL,
  external_url     TEXT NOT NULL DEFAULT '',
  external_version TEXT NOT NULL DEFAULT '',
  last_synced_at   TEXT NOT NULL,
  PRIMARY KEY (tenant_id, system, entity_type, external_id)
);
CREATE INDEX idx_external_entity ON external_refs(tenant_id, entity_type, entity_id);

CREATE TABLE sync_sources (
  id          TEXT PRIMARY KEY,
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  system      TEXT NOT NULL,
  name        TEXT NOT NULL,
  config      TEXT NOT NULL DEFAULT '{}',
  mapping_path TEXT NOT NULL DEFAULT '',
  cursor      TEXT NOT NULL DEFAULT '',
  last_run_at TEXT,
  last_status TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL,
  UNIQUE (tenant_id, system, name)
);
