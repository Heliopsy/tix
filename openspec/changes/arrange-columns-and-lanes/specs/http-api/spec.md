## ADDED Requirements

### Requirement: View arrangement endpoints

The API SHALL expose, for the authenticated actor: `GET /v1/views/{view}/groups` returning the
actor's preferences on the view; `PATCH /v1/views/{view}/groups/{group}` taking any of `hidden`,
`collapsed` and `starred` and returning the resulting preference; `PUT /v1/views/{view}/order`
taking `groups` and returning the preferences it wrote; and `DELETE /v1/views/{view}/groups`
forgetting every preference on the view, or only the one named by the `group` query parameter, and
answering `204 No Content`. A view is `board`, `lanes` or `project:KEY`; anything else SHALL be
refused as invalid, and a group the view cannot hold SHALL be refused as invalid or not found.

#### Scenario: Hiding a column over HTTP

- **WHEN** a client sends `PATCH /v1/views/board/groups/done` with `{"hidden": true}`
- **THEN** the response carries `"hidden": true` and a following `GET /v1/views/board/groups` lists it

#### Scenario: An unknown view is refused

- **WHEN** a client sends `GET /v1/views/calendar/groups`
- **THEN** the response is 400
