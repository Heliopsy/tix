## ADDED Requirements

### Requirement: Saved filter routes

The API SHALL serve `GET /v1/filters`, `POST /v1/filters` (answering `201`), `GET /v1/filters/{filter}`,
`PATCH /v1/filters/{filter}`, `PUT /v1/filters/order` and `DELETE /v1/filters/{filter}` (answering `204`)
over the saved filter operations, with the filter addressed by identifier or name.

#### Scenario: Save and read back

- **WHEN** a client posts `{"name": "ops", "query": "tag:ops"}` to `/v1/filters`
- **THEN** the answer is `201` with the filter, and `GET /v1/filters/ops` answers the same filter
