-- Monotonic per-project task numbering.
--
-- A task number leaks into commit messages, chat and webhooks, so it must never
-- be reused. The next number therefore comes from a counter that only moves
-- forward, not from MAX(seq) over rows a hard delete can take away.
--
-- Existing projects resume above the highest number already issued, including
-- numbers held by soft deleted tasks, so no reference in the wild is reissued.

ALTER TABLE projects ADD COLUMN seq_counter BIGINT NOT NULL DEFAULT 0;

UPDATE projects SET seq_counter = COALESCE(
  (SELECT MAX(tasks.seq) FROM tasks WHERE tasks.project_id = projects.id), 0);
