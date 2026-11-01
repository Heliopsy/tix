## ADDED Requirements

### Requirement: A personal calendar feed of due dates

The system SHALL let each actor hold at most one live calendar feed. A feed SHALL be addressed by a
URL carrying a secret that begins `tix_cal_`, stored only as its hash and returned only by the call
that created or rotated it. Fetching the URL SHALL need no session and SHALL answer an RFC 5545
`VCALENDAR` with one all-day `VEVENT` per selected task that has a due date.

#### Scenario: A feed lists the owner's open dated tasks

- **GIVEN** an actor with a feed and no filter, assigned one open task due 2026-11-05, one finished
  task with a due date and one open task with no due date
- **WHEN** a calendar app fetches the feed URL with no credential
- **THEN** the answer is `text/calendar` holding exactly one `VEVENT`, with `DTSTART;VALUE=DATE:20261105`

#### Scenario: A malformed secret is not found

- **WHEN** a client fetches a feed URL whose secret is not a `tix_cal_` secret
- **THEN** the answer is 404

### Requirement: Feed events are stable and well formed

Each event SHALL carry a `UID` derived from the task's identifier alone, a `SUMMARY` of the task's
reference and title, a `DESCRIPTION` and `URL` linking to the task's page, `DTSTAMP` and
`LAST-MODIFIED` from the task's update time, `SEQUENCE` from its version, and a `STATUS` of
`CANCELLED` for a task in a cancelled state and `CONFIRMED` otherwise, with a finished task's summary
marked with a check. Text SHALL be escaped and every line folded at 75 octets without splitting a
character, with CRLF line endings.

#### Scenario: The same task keeps its UID

- **WHEN** a task's title and due date change between two fetches
- **THEN** both documents carry the same `UID` for it and the later one a higher `SEQUENCE`

#### Scenario: A long title round-trips

- **WHEN** a task's title holds commas, semicolons, a backslash, a newline and more than 75 octets of
  multi-byte text
- **THEN** unfolding and unescaping the `SUMMARY` gives back the reference and the title exactly

### Requirement: A feed never reads more than its owner may

A feed SHALL read as its owner with no more than the owner's current `task:read`, `project:read`
and `workflow:read`, resolved from the owner's membership on every fetch. A revoked or rotated-away
secret, an owner no longer a member, and a disabled account SHALL all answer 404.

#### Scenario: A revoked feed is not found

- **GIVEN** a feed that answered 200
- **WHEN** its owner revokes it and the app fetches it again
- **THEN** the answer is 404

#### Scenario: A removed member's feed stops

- **WHEN** the owner of a feed is removed from the tenant
- **THEN** fetching the feed answers 404

### Requirement: Feed filters

A feed SHALL select with a filter expression in the task filter language, validated when it is
stored. Without one it SHALL select the tasks assigned to its owner whose state is not terminal.
Changing the filter SHALL keep the URL.

#### Scenario: A filter narrows the feed

- **GIVEN** a feed whose filter is `project:infra`
- **WHEN** it is fetched
- **THEN** it lists only dated tasks of `infra`, finished ones included

#### Scenario: An unknown project in a filter is refused

- **WHEN** a feed is created with the filter `project:nosuch`
- **THEN** the call fails as not found and no feed is created

### Requirement: Feeds answer conditional requests

A feed answer SHALL carry a strong `ETag` computed from its body and SHALL answer `304 Not Modified`
with no body to a request whose `If-None-Match` matches it.

#### Scenario: An unchanged feed is not sent twice

- **WHEN** a client fetches a feed, then fetches it again with `If-None-Match` set to the ETag it got
- **THEN** the second answer is 304
