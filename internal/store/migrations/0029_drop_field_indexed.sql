-- SPDX-License-Identifier: AGPL-3.0-or-later

-- field_defs.indexed was stored and displayed and nothing read it: no index
-- was built and no query planned differently. The flag is gone from the API,
-- so the column goes too.
--
-- A plain DROP COLUMN rather than a table rebuild: SQLite has supported it
-- since 3.35, and indexed sits in no index, constraint, trigger or view,
-- which are the cases where SQLite refuses the statement.

ALTER TABLE field_defs DROP COLUMN indexed;
