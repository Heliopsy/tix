## ADDED Requirements

### Requirement: The API sets and explains lease length

`PATCH /api/v1/tasks/REF` SHALL accept `lease` and `PATCH /api/v1/projects/KEY` SHALL accept `lease_default` and
`lease_max`. `GET /api/v1/tasks/REF/lease` SHALL answer with the lease a claim naming no ttl would be granted,
with `ttl`, `source`, `max`, `max_source` and `clamped`. Claim and renew responses SHALL carry the same as `lease`.

#### Scenario: Reading a task's lease

- **WHEN** a task's lease is 4h and a client reads `/api/v1/tasks/REF/lease`
- **THEN** the answer has `source` `task` and `ttl` of four hours
