-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Run the actor_user_link and actor_display_name_from_account backfills again.
--
-- On PostgreSQL both ran under the forced row-level security policies with no
-- tenant set, so as a role without BYPASSRLS they saw no actors and changed
-- nothing. Every account created before 0.46.0 kept an actor with no user_id:
-- it could not sign in, was missing from the user list, and its global admin
-- standing did not apply. The runner now gives every migration a policy that
-- admits it (see internal/store/postgres/schema.go), so these reach the rows.
--
-- Both statements are no-ops where the earlier ones worked, as on SQLite, or
-- where the link was repaired by hand. An actor is not linked when another
-- actor of its tenant already holds the account, which the unique index on
-- (tenant_id, user_id) would refuse.

UPDATE actors SET user_id = id
WHERE kind = 'user' AND user_id IS NULL AND id IN (SELECT id FROM users)
  AND NOT EXISTS (SELECT 1 FROM actors o WHERE o.tenant_id = actors.tenant_id AND o.user_id = actors.id);

UPDATE actors SET display_name = (SELECT u.display_name FROM users u WHERE u.id = actors.user_id)
WHERE kind = 'user' AND user_id IS NOT NULL
  AND display_name <> (SELECT u.display_name FROM users u WHERE u.id = actors.user_id)
  AND (SELECT COUNT(*) FROM actors o WHERE o.user_id = actors.user_id) = 1;
