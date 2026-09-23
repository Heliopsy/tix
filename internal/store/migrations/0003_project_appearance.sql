-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Per-project colour and icon.
--
-- Someone working across several projects reads a mixed task list, where the
-- only thing separating one project's rows from another's is the reference at
-- the end of the line. A colour and an icon give the eye something to sort on.
--
-- The colour is a palette token, not a hex value, so every colour scheme can
-- render it with a shade tuned for its own background.
--
-- Existing projects get the empty token, which renders exactly as they render
-- today: no stripe, no icon.

ALTER TABLE projects ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN icon TEXT NOT NULL DEFAULT '';
