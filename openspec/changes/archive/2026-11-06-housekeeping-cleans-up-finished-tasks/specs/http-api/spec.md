## ADDED Requirements

### Requirement: The API exposes housekeeping

`GET /api/v1/housekeeping` SHALL answer the policy, with `?project=KEY` the override and effective rules, and
the preview counts. `PUT /api/v1/housekeeping` SHALL set either level. `POST /api/v1/housekeeping/run` SHALL
run one pass, or preview with `dry_run`.

#### Scenario: Refused while the trash is off

- **GIVEN** a tenant whose trash is off
- **WHEN** a client puts `{"done": "30"}`
- **THEN** it answers 422 with `details.reason` `trash_off`
