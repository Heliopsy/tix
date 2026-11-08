# Tasks

- [x] Set `tix.migrating` on every migration transaction and admit it with a policy on every forced table
- [x] Migration `repair_actor_links` running both backfills again, idempotent
- [x] Tests: an upgrade from before 0.46.0 as a NOBYPASSRLS owner signs in, lists users and keeps global admin; every forced table carries the policy; a probe data step reaches every tenant; the flag widens no runtime transaction
- [x] Docs: the engine README and an upgrading note in `docs/migrating.md`
