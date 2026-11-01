-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Whether a tenant keeps deleted tasks in a trash.
--
-- Stored as trash_off rather than trash_on so the column default is the
-- behaviour every tenant already had: deleting a task soft deletes it. An
-- upgrade that defaulted the other way would start destroying tasks on the
-- next delete without anybody having chosen it.
--
-- A column rather than a retention_policies row, following require_totp_*:
-- it is one flag on a tenant, read inside the delete that is already in a
-- transaction, not something the pruner reads on a schedule.

ALTER TABLE tenants ADD COLUMN trash_off INTEGER NOT NULL DEFAULT 0;
