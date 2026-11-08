-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A person's name in a tenant is the one on their actor there.
--
-- Until now a rename by an administrator who could reach the account wrote
-- the account's name and left the actor's, so for a person in one tenant the
-- user list showed one name and every other screen, which reads the actor,
-- showed the name they were created with. The user list now reads the actor
-- too, so the actor of a person who belongs to one tenant only takes the
-- account's name, which is the one an administrator last set. A person with
-- actors in several tenants keeps each actor's name, because those may have
-- been set per tenant on purpose.

UPDATE actors SET display_name = (SELECT u.display_name FROM users u WHERE u.id = actors.user_id)
WHERE kind = 'user' AND user_id IS NOT NULL
  AND display_name <> (SELECT u.display_name FROM users u WHERE u.id = actors.user_id)
  AND (SELECT COUNT(*) FROM actors o WHERE o.user_id = actors.user_id) = 1;
