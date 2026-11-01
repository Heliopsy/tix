-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Snoozing a task until a moment in the future.
--
-- A task is snoozed while snoozed_until is after the present, judged at read
-- time, so a task comes back on its own the moment the time passes whether or
-- not any process is running. The sweeper only clears a passed value and
-- announces the wake; it is never what makes a task visible again.
--
-- snoozed_by is who set it. The audit entry records the same thing, this is
-- the cheap read beside the task for the badge. NULL in both is a task that is
-- not snoozed, so every existing row upgrades into exactly what it already was.

ALTER TABLE tasks ADD COLUMN snoozed_until TEXT;

ALTER TABLE tasks ADD COLUMN snoozed_by TEXT REFERENCES actors(id) ON DELETE SET NULL;

-- Serves the wake pass, which asks for the passed values in one tenant. Rows
-- that are not snoozed sort together under the NULL and are never walked.
CREATE INDEX idx_tasks_snoozed ON tasks(tenant_id, snoozed_until);
