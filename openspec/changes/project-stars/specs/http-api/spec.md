## ADDED Requirements

### Requirement: The API stars projects for the caller

`PUT /api/v1/projects/{ref}/star` SHALL star a project for the caller and `DELETE` on the same path
SHALL clear it, each answering 204. `GET /api/v1/me/starred-projects` SHALL list the caller's starred
projects. A project returned to a known caller SHALL carry `starred: true` when that caller starred it.

#### Scenario: Another caller's stars stay out

- **GIVEN** Ada starred INFRA through the API
- **WHEN** Bob reads `GET /api/v1/me/starred-projects`
- **THEN** the list is empty and `GET /api/v1/projects/INFRA` carries no `starred`
