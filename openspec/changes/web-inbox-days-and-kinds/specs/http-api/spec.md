## ADDED Requirements

### Requirement: Inbox listing by kind

`GET /api/v1/inbox` SHALL accept `kind`, repeatable, keeping only items of the kinds named. An unknown kind
SHALL be answered with 400.

#### Scenario: Two kinds

- **WHEN** a client sends `GET /api/v1/inbox?kind=mention&kind=status` for an inbox holding only comments
- **THEN** the page holds no items
