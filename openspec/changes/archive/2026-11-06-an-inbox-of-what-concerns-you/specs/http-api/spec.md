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

### Requirement: Notification channel endpoints

The API SHALL expose `GET /v1/channels`, `POST /v1/channels` (answering 201 with the channel and its
`secret`), `PATCH /v1/channels/{id}`, `DELETE /v1/channels/{id}`, `POST /v1/channels/{id}/test`,
`GET /v1/channels/{id}/deliveries` (keyset paginated, newest first) and
`POST /v1/channel-deliveries/{id}/redeliver`, all over the authenticated actor's own channels.

#### Scenario: The secret is returned once

- **WHEN** a client creates a channel and then lists channels
- **THEN** the creation carries `secret` and the listing does not
