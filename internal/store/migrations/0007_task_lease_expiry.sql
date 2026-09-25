-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Evidence that a claim ran out.
--
-- The sweeper clears claimed_by_actor_id, claimed_at, lease_expires_at and
-- lease_token when a lease passes its expiry, which is correct: the task is
-- free again and must read that way. But it also erased the only thing on the
-- row that said a holder had taken this task and then stopped answering. The
-- state was observable for at most one sweep interval, so in practice nobody
-- ever saw it, and the one signal that tells an operator an agent keeps dying
-- disappeared with the lease it belonged to.
--
-- These two columns are written in the same statement that clears the lease,
-- so the evidence cannot disagree with the claim it describes. The audit entry
-- and the task.lease_expired event still carry the full history; this is the
-- cheap read beside the task, because deriving it would mean the most recent
-- audit entry of one action per listed row, and audit_entries is indexed by
-- subject rather than by action and is pruned on a tenant-configurable
-- retention window.
--
-- A fresh claim clears both, because what the reader wants to know is whether
-- the work was dropped and left dropped.

ALTER TABLE tasks ADD COLUMN lease_expired_at TEXT;

ALTER TABLE tasks ADD COLUMN lease_expired_by TEXT REFERENCES actors(id) ON DELETE SET NULL;

-- Serves "which tasks were dropped recently", which is the question these
-- columns exist to answer. Rows that never expired sort together under the
-- NULL, so the scan for a recent window never walks them.
CREATE INDEX idx_tasks_lease_expired ON tasks(tenant_id, lease_expired_at, id);
