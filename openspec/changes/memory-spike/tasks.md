# Tasks

- [x] Seed a thousand long tasks and measure the screens' allocations to find the top allocators
- [x] List the overdue badge's tasks without descriptions (`TaskFilter.OmitBody`, both engines)
- [x] Count checklists with a block-only parse, a checkbox pre-check and a bounded cache
- [x] Prove the checklist parse finds the renderer's items, by corpus and by fuzzing
- [x] Guard the badge's allocation and the checklist parse's
- [x] Set the soft memory limit from the cgroup limit in `tix serve`, leaving a set `GOMEMLIMIT` alone
- [x] Document it in `docs/deployment.md`
