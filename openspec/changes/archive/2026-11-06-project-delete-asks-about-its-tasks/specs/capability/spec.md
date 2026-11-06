## ADDED Requirements

### Requirement: The registry records the project deletion plan on every surface

The capability registry SHALL declare `project.delete-plan` for `PlanProjectDeletion` with scope `project:read`,
bound to `tix project rm` on the command line, `GET /api/v1/projects/{ref}/deletion` over HTTP, the deletion
confirmation screen in the browser, and the project view in the terminal, with no gap recorded against it.

#### Scenario: The plan binds every surface

- **WHEN** the registry entry for `PlanProjectDeletion` is read
- **THEN** it names a binding on the command line, HTTP, the browser and the terminal, and records no gap
