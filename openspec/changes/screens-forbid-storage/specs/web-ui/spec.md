## ADDED Requirements

### Requirement: Rendered screens forbid storage

Every screen the browser interface renders SHALL carry `Cache-Control: no-store`.
This SHALL apply to successful renders, to the error screen, and to a form
redrawn after a refusal, so that no rendered response reaches a browser cache,
a shared cache, or the back-forward cache.

Static assets are excluded: they carry a strong validator and an explicit
freshness lifetime instead, because their URLs do not change between releases.

#### Scenario: An authenticated screen

- **WHEN** a signed-in reader loads the task list, a board, a project, the
  activity screen or any administration screen
- **THEN** the response carries `Cache-Control: no-store`

#### Scenario: A refused form

- **WHEN** a form is submitted and redrawn with a validation refusal
- **THEN** the redrawn screen carries `Cache-Control: no-store`, so the values
  the user typed are not written to a cache

#### Scenario: The error screen

- **WHEN** a request resolves to the error screen
- **THEN** that response carries `Cache-Control: no-store`

#### Scenario: Assets are not affected

- **WHEN** a stylesheet, script or image under the asset path is served
- **THEN** it carries its strong ETag and freshness lifetime, and does not
  carry `no-store`
