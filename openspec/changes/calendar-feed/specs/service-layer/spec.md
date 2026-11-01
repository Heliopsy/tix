## ADDED Requirements

### Requirement: Calendar feed operations

The service SHALL offer `GetCalendarFeed`, `CreateCalendarFeed`, `RotateCalendarFeed`,
`SetCalendarFeedFilter` and `RevokeCalendarFeed` on the caller's own feed behind the `calendar.feed`
action, which needs `task:read` and is not confinable to a project, and `ReadCalendarFeed`, which
takes the secret and no actor. Creating a feed while one is live SHALL be refused as a conflict, and
an actor with no membership in the tenant SHALL be refused. Each change SHALL write its row, an audit
entry and a `calendar_feed.*` event in one transaction, and no event or audit entry SHALL carry the
secret or its hash.

#### Scenario: A second feed is a conflict

- **GIVEN** an actor with a live feed
- **WHEN** they create another
- **THEN** the call fails as a conflict and the first feed's URL still answers

#### Scenario: Rotation keeps the filter

- **GIVEN** a feed with the filter `tag:ops`
- **WHEN** its owner rotates it
- **THEN** the new URL answers with the same filter and the old URL answers 404
