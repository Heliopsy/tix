-- SPDX-License-Identifier: AGPL-3.0-or-later

-- The server processes running against this database.
--
-- This is the first table since the initial schema with no tenant_id, and the
-- omission is the point rather than an oversight. One `tix serve` answers
-- requests for every tenant the database holds: the router resolves the tenant
-- per request, from the Host header or from the token, and the next request a
-- millisecond later belongs to somebody else. There is no value a tenant_id
-- column could hold that would not be a falsehood, and the first thing anybody
-- would do with it is filter by it, which would hide servers from the tenants
-- they are serving.
--
-- So it is reached through store.UnscopedTx, which is where installation facts
-- already live, and a confinement test names the files that may do so.
--
-- Nothing here identifies a tenant. Not an identifier, not a key, not a
-- hostname that resolves to one, not an actor. That is what makes the row
-- showable to a tenant administrator at all, and it is asserted rather than
-- reviewed, because a column added later is exactly what the isolation suite
-- cannot see: that suite is written method by method and a new table is
-- invisible to it.
--
-- last_seen_at is a heartbeat and it is authoritative the moment it goes stale,
-- the way a lease expiry already is. A reader compares it to its own clock and
-- calls the server gone; no sweeper has to have run for that to be right. A
-- graceful shutdown deletes the row, and a crash deliberately does not, because
-- a dead server an operator can see is the whole reason this table exists.
--
-- surfaces is a comma-joined list of the four words api, ws, web and ssh. A
-- closed set of four needs neither a document type nor a second scanner per
-- engine to hold it.
CREATE TABLE servers (
  id           TEXT PRIMARY KEY,
  address      TEXT NOT NULL,
  version      TEXT NOT NULL,
  surfaces     TEXT NOT NULL DEFAULT '',
  started_at   TEXT NOT NULL,
  last_seen_at TEXT NOT NULL
);

-- Serves the one question asked of this table: which servers were seen
-- recently. The identifier is in the index because the listing pages by keyset
-- and every keyset ordering ends on the identifier.
CREATE INDEX idx_servers_last_seen ON servers(last_seen_at, id);
