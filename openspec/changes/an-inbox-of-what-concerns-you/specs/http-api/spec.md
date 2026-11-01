## ADDED Requirements

### Requirement: Inbox endpoints

The API SHALL expose, for the authenticated actor: `GET /v1/inbox` listing items newest first with
`limit`, `cursor` and `unread`, and a `next_cursor`; `GET /v1/inbox/count` returning `unread`;
`PUT /v1/inbox/{id}/read` and `DELETE /v1/inbox/{id}/read` marking one item read or unread and
returning it; `POST /v1/inbox/read-all` returning how many it marked; and `GET /v1/inbox/prefs` and
`PUT /v1/inbox/prefs` reading and replacing the muted kinds. An unknown kind SHALL be refused as
invalid.

#### Scenario: Paging the inbox

- **GIVEN** an actor with three items
- **WHEN** a client sends `GET /v1/inbox?limit=2` and then follows `next_cursor`
- **THEN** it sees the three items newest first, each once

#### Scenario: Muting an unknown kind

- **WHEN** a client sends `PUT /v1/inbox/prefs` with `{"muted": ["everything"]}`
- **THEN** the response is 400
