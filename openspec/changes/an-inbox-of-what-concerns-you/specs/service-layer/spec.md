## ADDED Requirements

### Requirement: Changes that concern somebody create inbox items

The service SHALL create an inbox item, in the same transaction as the change, when an actor is
mentioned as `@handle` in a comment or a task description (`mention`), when a task is assigned to an
actor (`assigned`), when somebody comments on a task an actor is assigned to or has starred
(`comment`), and when a task an actor is assigned to changes status (`status`). It SHALL NOT create
an item for the actor who made the change. One change SHALL give one recipient at most one item per
task, keeping the strongest reason in the order mention, assigned, status, comment. Editing a comment
or a description SHALL mention only handles the earlier text did not. A handle that names no actor
SHALL be ignored. A kind the recipient muted SHALL create no item for them.

#### Scenario: A mention reaches the person named

- **WHEN** ada comments "@bob can you look" on a task
- **THEN** bob's inbox holds one unread `mention` item naming the task, ada and the comment

#### Scenario: Your own actions do not notify you

- **WHEN** ada assigns a task to herself and comments "@ada note to self"
- **THEN** ada's inbox is unchanged

#### Scenario: The strongest reason wins

- **GIVEN** bob is assigned to a task and has starred it
- **WHEN** ada comments on it mentioning @bob
- **THEN** bob receives exactly one item, of kind `mention`

#### Scenario: Watchers hear about comments

- **GIVEN** carol starred a task assigned to bob
- **WHEN** ada comments on it
- **THEN** bob and carol each receive a `comment` item

#### Scenario: A status change reaches the assignee

- **GIVEN** a task assigned to bob
- **WHEN** ada moves it from `todo` to `doing`
- **THEN** bob receives a `status` item carrying `todo` and `doing`

#### Scenario: A muted kind is silent

- **GIVEN** bob muted `comment`
- **WHEN** ada comments on a task bob is assigned to
- **THEN** bob receives nothing

### Requirement: Reading and muting the inbox

The service SHALL let an actor list their own items newest first with keyset pagination, optionally
only unread ones; count their unread items; mark one item read or unread; mark every item read; and
read and replace the set of kinds they muted. An item of another actor SHALL be not found. Each
change SHALL write an audit entry and an `inbox.updated` event in one transaction, and a change that
changes nothing SHALL write nothing. These methods SHALL need `task:read` and SHALL NOT be available
to a project-pinned token.

#### Scenario: Marking everything read

- **GIVEN** bob has three unread items
- **WHEN** bob marks everything read
- **THEN** his unread count is 0 and one audit entry records it

#### Scenario: Somebody else's item

- **WHEN** ada marks bob's item read
- **THEN** the call fails as not found and the item stays unread
