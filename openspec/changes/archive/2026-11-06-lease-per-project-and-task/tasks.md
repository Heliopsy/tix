## 1. Storage and resolution

- [x] 1.1 Migration 0028: `projects.lease_default`, `projects.lease_max`, `tasks.lease`
- [x] 1.2 Both stores read and write the columns
- [x] 1.3 `lease.Policy.Grant` and `Cap` with sources; claim, claim next and renew resolve through it

## 2. Service

- [x] 2.1 `UpdateTask` lease and `UpdateProject` lease bounds, refused above the ceiling
- [x] 2.2 `TaskLease` and the capability entry
- [x] 2.3 Copy, template, next occurrence, export and import carry the task's lease

## 3. Surfaces

- [x] 3.1 CLI flags, `task show` line, claim output
- [x] 3.2 HTTP route and docs/api.md
- [x] 3.3 Web: task Lease row, board Claims panel, Bulgarian strings
- [x] 3.4 TUI detail line
- [x] 3.5 docs/agents.md, docs/web-ui.md, skills/tix
