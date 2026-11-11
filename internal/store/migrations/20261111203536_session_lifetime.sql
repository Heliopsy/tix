-- SPDX-License-Identifier: AGPL-3.0-or-later

-- How long a sign-in lasts, per tenant, and what a session needs to be ended
-- for sitting unused.
--
-- The tenant columns only tighten the server's session.ttl,
-- session.idle_timeout and session.remember_ttl. They are duration text as
-- lease_default is, and '' inherits, so every existing tenant keeps what it
-- had. session_remember_ttl also takes 'off', which stops offering "keep me
-- signed in" on the tenant.
--
-- sessions.last_seen_at is when the session last authenticated a request,
-- written at most once a minute; NULL reads as its created_at, so a session
-- minted before this migration is idle from the moment it signed in.
-- sessions.remembered marks a "keep me signed in" sign-in. A session's
-- expires_at was fixed when it was minted and nothing here moves it.

ALTER TABLE tenants ADD COLUMN session_ttl TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN session_idle_timeout TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN session_remember_ttl TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN last_seen_at TEXT;
ALTER TABLE sessions ADD COLUMN remembered INTEGER NOT NULL DEFAULT 0;
