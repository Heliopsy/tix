-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Raise every project's task number counter to the highest number its tasks
-- hold, trashed ones included.
--
-- A snapshot import wrote each task under its original number and left the
-- counter where it was, so after importing into a fresh tenant the next task
-- created took a number an imported task already held and was refused as a
-- conflict. Imports now raise the counter themselves; this repairs the
-- projects they left behind. A counter already above its tasks is untouched,
-- since a hard deleted task's number must never be issued again.

UPDATE projects SET seq_counter = (
  SELECT MAX(tasks.seq) FROM tasks
  WHERE tasks.tenant_id = projects.tenant_id AND tasks.project_id = projects.id)
WHERE seq_counter < (
  SELECT MAX(tasks.seq) FROM tasks
  WHERE tasks.tenant_id = projects.tenant_id AND tasks.project_id = projects.id);
