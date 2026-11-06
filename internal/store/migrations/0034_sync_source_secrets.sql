-- SPDX-License-Identifier: AGPL-3.0-or-later

-- A sync source can carry its mapping document and its credentials, so a
-- source can be configured from wherever its owner is rather than only from
-- the server's filesystem and environment, which is where every run executes.
--
-- The credentials are sealed with the server's key (auth.totp_key), the same
-- AES-GCM sealing second factors use, and bound to the tenant, system and name
-- of the source as additional data, so a ciphertext copied onto another row
-- does not open. The key lives in configuration and never here.
-- credential_hints holds which credentials are stored and, for a long one, its
-- last four characters: what a screen may show, and nothing more.
ALTER TABLE sync_sources ADD COLUMN mapping TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_sources ADD COLUMN credentials_sealed TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_sources ADD COLUMN credentials_nonce TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_sources ADD COLUMN credential_hints TEXT NOT NULL DEFAULT '[]';
