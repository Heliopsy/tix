-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Whether accepting an invite is what grants its membership.
--
-- An invite to an address that already has an account in another tenant
-- creates that person's actor here when it is issued, so the invite row has an
-- actor to point at, but no membership: an administrator of this tenant must
-- not be able to add somebody else's account to it without that person's
-- consent. The membership is written when the person accepts with their own
-- password and second factor. Every invite issued before this reads 0 and
-- keeps behaving as it did.
--
-- On PostgreSQL this migration also brings the read-only policies that let
-- the store list an account's memberships across tenants (see
-- internal/store/postgres/schema.go).

ALTER TABLE user_invites ADD COLUMN grants_membership INTEGER NOT NULL DEFAULT 0;
