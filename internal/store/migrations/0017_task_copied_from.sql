-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The task a copy was copied from.
--
-- Not a foreign key, for the reason recurrence_next_id is not one: purging the
-- source must neither be blocked by its copies nor reach into them, and a link
-- to a purged task resolves to nothing, which every reader handles. NULL is a
-- task that was created rather than copied, so every existing row upgrades
-- into exactly what it already was.

ALTER TABLE tasks ADD COLUMN copied_from_id TEXT;
