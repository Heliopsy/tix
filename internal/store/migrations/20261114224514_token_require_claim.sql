-- SPDX-License-Identifier: AGPL-3.0-or-later

-- An API token can require a claim: a caller holding it changes a task only
-- while the token's actor holds a live claim on that task. It is chosen when
-- the token is minted and carried over by rotation. Every existing token gets
-- 0, so it behaves exactly as it did.

ALTER TABLE api_tokens ADD COLUMN require_claim INTEGER NOT NULL DEFAULT 0;
