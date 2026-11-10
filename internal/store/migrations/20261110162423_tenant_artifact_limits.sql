-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A tenant's own artifact limits, which only tighten the server's
-- artifacts.max_bytes and artifacts.allowed_types. Zero and the empty string
-- inherit the server's, so every existing tenant keeps what it had. The types
-- are a comma separated list of media types and patterns such as image/*.
-- Nothing re-judges artifacts already stored.

ALTER TABLE tenants ADD COLUMN artifact_max_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tenants ADD COLUMN artifact_types TEXT NOT NULL DEFAULT '';
