## ADDED Requirements

### Requirement: Settings states who the reader is

The settings screen SHALL open with a Profile section naming the signed-in actor's handle, email, display
name, identifier, kind, role, tenant, memberships, scopes, whether it may act as a global admin, and whether
the request authenticated with a browser session or an API token. It SHALL read them from `WhoAmI` and
SHALL link to the reader's own actor screen.

#### Scenario: An administrator reads their profile

- **GIVEN** alice, an admin of the tenant acme
- **WHEN** she opens Settings
- **THEN** the Profile section names `alice`, her email, her actor identifier, the role admin and the tenant acme
- **AND** it lists acme among her memberships as this tenant
- **AND** it links to `/actors/` followed by her identifier

### Requirement: An actor has a screen of its own

`GET /actors/{id}` SHALL show the actor a handle or an identifier names: its handle, display name, kind and
identifier, a link to the tasks assigned to it, and for a reader holding `token:admin` links to its tokens
and SSH keys. It SHALL show no scope or role of that actor. An identifier this tenant does not hold SHALL be
not found.

#### Scenario: Opening an agent by handle

- **GIVEN** an agent with the handle `ci-bot`
- **WHEN** a member opens `/actors/ci-bot`
- **THEN** the screen names `ci-bot`, the kind agent and its identifier

### Requirement: The actor screen shows recent activity to an audit reader

The actor screen SHALL list the actor's most recent audit entries, grouped and worded as the activity feed
words them, with a link to the activity feed narrowed to that actor. A reader without `audit:read` SHALL be
told the activity needs that scope, and SHALL still be shown the rest of the screen.

#### Scenario: A viewer without the audit scope

- **GIVEN** a reader without `audit:read`
- **WHEN** they open an actor screen
- **THEN** the screen shows the actor and says recent activity needs `audit:read`

### Requirement: An actor named on a screen links to its actor screen

The directory's handles, a task's assignee and creator, a comment's author and a token's owner SHALL link to
that actor's screen.

#### Scenario: Following a comment's author

- **GIVEN** a comment written by alice
- **WHEN** the reader activates the author's name on the task screen
- **THEN** alice's actor screen opens

### Requirement: The task screen shows the whole subtask tree

The task screen SHALL draw the task's subtasks as a tree to any depth. It SHALL draw three levels on load,
and a node at the third level that has children SHALL offer to load them in place, three levels further,
from `GET /tasks/{ref}/tree`. Without scripting that offer SHALL be a link to the subtask's own screen.

#### Scenario: A grandchild is drawn

- **GIVEN** infra-1 with a subtask infra-2 that has a subtask infra-3
- **WHEN** the reader opens infra-1
- **THEN** infra-3 is drawn under infra-2 in the Subtasks section

#### Scenario: A deep subtree loads on request

- **GIVEN** a chain of subtasks five levels deep under infra-1
- **WHEN** the reader opens infra-1
- **THEN** the fourth level is not drawn and its parent offers to show its subtasks

### Requirement: A subtask tree node collapses

Each tree node with drawn children SHALL carry a control with `aria-expanded` that hides and shows them.
Without scripting the control SHALL NOT be drawn and the tree SHALL stay open. A subtree too large to walk
SHALL fall back to direct subtasks rather than failing the screen.

#### Scenario: Collapsing a node

- **WHEN** the reader activates the collapse control of infra-2
- **THEN** infra-2's subtasks are hidden and the control reads `aria-expanded="false"`

### Requirement: The token and SSH key screens take an actor filter

The token screen and the SSH key screen SHALL take an `actor` query parameter, a handle or an identifier,
and SHALL then list that actor's credentials only, naming whose they are. The service SHALL decide whether
the reader may; the filter SHALL be a GET form offering the directory's handles.

#### Scenario: A token keeper reads another actor's keys

- **GIVEN** a reader holding `token:admin` and not `tenant:admin`
- **AND** another actor with an enrolled key
- **WHEN** they open `/admin/ssh-keys?actor=` with that actor's handle
- **THEN** that key is listed and the screen names whose keys it lists

## MODIFIED Requirements

### Requirement: A tenant administrator can see and revoke another actor's token in the browser

The API token screen SHALL list every token of the tenant to a reader holding the tenant administration
scope, and SHALL list only the reader's own tokens to any other reader unless the reader names another actor
in the screen's actor filter.

Where the listing spans more than the reader's own tokens, each row SHALL say which actor holds it, by that
actor's handle where the directory holds one. A row that is the reader's own SHALL be distinguishable from
a row that is not.

#### Scenario: A reader without the administration scope sees only their own

- **GIVEN** a reader holding the token administration scope and not the tenant administration scope
- **AND** a token held by another actor of the same tenant
- **WHEN** the token screen is read with no actor filter
- **THEN** the other actor's token is not listed
- **AND** no revocation control on the screen names that token
- **AND** the listing carries no owner column

#### Scenario: An administrator sees the tenant's tokens with the owner named

- **GIVEN** a reader holding the tenant administration scope
- **AND** a token held by another actor of the same tenant
- **WHEN** the token screen is read
- **THEN** that token is listed
- **AND** its row names the actor holding it
- **AND** a revocation control is offered for it
