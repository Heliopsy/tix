-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The access channel of every audit entry and event: how the caller reached
-- tix (web session, API token, local or remote CLI, SSH, MCP, a sync run, an
-- import or a background job), set by the entry point and never by a request
-- body. Rows written before this migration read as 'unknown'.
--
-- via_kind and via_token are columns because the activity feed and
-- `tix audit ls` filter on them; the rest of the detail (client address, user
-- agent, session row id, SSH key fingerprint, host) is JSON text. No column
-- holds a secret: a token is named, a session is its row id.

ALTER TABLE audit_entries ADD COLUMN via_kind TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE audit_entries ADD COLUMN via_token TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_entries ADD COLUMN via_detail TEXT;
ALTER TABLE events ADD COLUMN via_kind TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE events ADD COLUMN via_detail TEXT;
CREATE INDEX idx_audit_via ON audit_entries(tenant_id, via_kind, seq);
