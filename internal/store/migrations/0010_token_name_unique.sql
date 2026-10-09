-- SPDX-License-Identifier: AGPL-3.0-or-later

-- One live token name per tenant.
--
-- A token's name is the only thing on the revocation control that tells one
-- token from another: the identifier beside it is a ULID nobody reads, and the
-- scope list is usually identical across the tokens an operator is choosing
-- between. Two tokens called "ci" therefore made "revoke the one that leaked"
-- a guess, which is a security defect rather than an untidy listing.
--
-- Scoped to the tenant, like every other credential here, and to rows that are
-- not revoked. A revoked token keeps its name so the listing can still explain
-- what stopped working, and the name becomes available again, which is what an
-- operator rotating a credential expects: revoke "ci", mint "ci".
--
-- Existing duplicates are renamed rather than revoked. Revoking would stop an
-- agent that is working right now, to fix a listing; appending the identifier
-- keeps every token authenticating, cannot collide (the identifier is the
-- primary key), and leaves the operator a name that says which row to look at.
-- The row kept unchanged is the lowest identifier, which for a ULID is the
-- oldest, so the long-standing token keeps the name people know it by.
UPDATE api_tokens SET name = name || ' (' || id || ')'
WHERE revoked_at IS NULL
  AND id NOT IN (
    SELECT MIN(id) FROM api_tokens WHERE revoked_at IS NULL GROUP BY tenant_id, name
  );

CREATE UNIQUE INDEX idx_tokens_name_live ON api_tokens(tenant_id, name) WHERE revoked_at IS NULL;
