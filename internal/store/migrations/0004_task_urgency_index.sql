-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Index for the default task ordering: priority first, then due date, with a
-- task that carries no due date treated as the most distant deadline, so at
-- the same priority a dated task always leads an undated one.
--
-- The expression matches the ORDER BY and the keyset predicate exactly, so
-- the planner can walk this index instead of sorting. Without it the default
-- listing degrades into a full scan plus a sort at scale, which keyset
-- pagination exists to avoid.

CREATE INDEX idx_tasks_urgency ON tasks(tenant_id, priority, COALESCE(due_at, '9999-12-31T23:59:59.000000000Z'), id);
