## ADDED Requirements

### Requirement: Calendar feed endpoints

The API SHALL expose, for the authenticated actor: `GET /api/v1/calendar` returning their feed's
metadata; `POST /api/v1/calendar` taking an optional `filter` and answering `201` with the feed, its
secret and its absolute `url`; `PATCH /api/v1/calendar` taking `filter`; `POST
/api/v1/calendar/rotate` answering with the new secret and `url`; and `DELETE /api/v1/calendar`
answering `204`. It SHALL serve `GET /api/v1/calendar/feeds/{token}.ics` without a credential.

#### Scenario: Creating a feed over HTTP returns its URL once

- **WHEN** a client sends `POST /api/v1/calendar` with `{}`
- **THEN** the answer is 201 with a `url` ending `.ics`, and a following `GET /api/v1/calendar`
  carries no secret

#### Scenario: The feed route needs no credential

- **WHEN** a client with no Authorization header and no cookie fetches the `url` it was given
- **THEN** the answer is 200 with `Content-Type: text/calendar; charset=utf-8`
