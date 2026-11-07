-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Link every user actor to its account through actors.user_id, which the
-- initial schema declared and nothing wrote.
--
-- Until now a user and its actor were written with one identifier, and code
-- found the account by the actor's ID. That coincidence is what tied an
-- account to a single tenant. The backfill keeps every identifier as it is and
-- states the link explicitly, so a person can later hold an actor in another
-- tenant whose identifier differs from the account's.
--
-- The partial unique index keeps at most one actor per account per tenant.
-- The plain index serves finding an account's actors.

UPDATE actors SET user_id = id
WHERE kind = 'user' AND user_id IS NULL AND id IN (SELECT id FROM users);

CREATE UNIQUE INDEX idx_actors_tenant_user ON actors(tenant_id, user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_actors_user ON actors(user_id) WHERE user_id IS NOT NULL;
