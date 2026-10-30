-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Tasks that repeat when they are done.
--
-- recurrence holds the canonical rule text core.ParseRecurrence reads, such as
-- 'every 2 weeks on mon,thu'. Empty means the task does not repeat, the same
-- "nobody chose" convention tenants.theme uses, so every existing task
-- upgrades into exactly the behaviour it already had.
--
-- recurrence_next_id names the occurrence a finished task spawned. It is the
-- guard against spawning twice, which is why it is deliberately not a foreign
-- key: ON DELETE SET NULL would let purging the successor clear the guard, and
-- reopening and finishing the predecessor would then spawn a second time. A
-- link to a purged task resolves to nothing, which every reader handles.

ALTER TABLE tasks ADD COLUMN recurrence TEXT NOT NULL DEFAULT '';

ALTER TABLE tasks ADD COLUMN recurrence_next_id TEXT;
