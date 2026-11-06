## ADDED Requirements

### Requirement: Task templates table

Migration 0025 SHALL create `task_templates` with tenant, identifier, name, nullable project,
JSON body, due offset in seconds, use counter, creator and timestamps, on both engines, with the
tenant-scoped row-level security every scoped table has on PostgreSQL. Deleting a project SHALL
delete its templates.

#### Scenario: Upgrade

- **WHEN** a database at migration 0020 is opened by this build
- **THEN** `task_templates` exists and no task row changed
