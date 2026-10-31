-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Indexes that let a page of tasks find its own tags and dependencies.
--
-- Both tables are keyed (task_id, ...) without the tenant, and every query
-- the tenant-scoped builder writes leads with tenant_id. Given a tenant_id
-- equality and a task_id that arrives through a correlated subquery or an IN
-- list, SQLite picked the (tenant_id, tag_id) and (tenant_id, depends_on)
-- indexes instead, which walk every tag or dependency row in the tenant for
-- each task asked about. A tag: filter on a tenant of 200 tasks spent 20ms in
-- that walk, and it grows with the tenant's tags times the tasks considered.
-- Leading with the tenant and then the task makes each lookup a seek.

CREATE INDEX idx_task_tags_task ON task_tags(tenant_id, task_id, tag_id);
CREATE INDEX idx_task_deps_task ON task_deps(tenant_id, task_id, depends_on);
