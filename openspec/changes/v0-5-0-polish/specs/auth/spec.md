## ADDED Requirements

### Requirement: The directory of actors can be listed

The service SHALL offer a listing of the actors a tenant holds, so that a screen or a command can offer a
choice of who to assign work to rather than asking for an identifier to be typed from memory.

The listing SHALL be confined to the caller's tenant, SHALL be keyset paginated like every other listing,
and SHALL return only the fields that name an actor: its identifier, kind, handle and display name. It
SHALL NOT report what an actor may do, because the question it answers is who is here.

It SHALL include agents as well as people. An agent holding work is ordinary in this product, so a
directory built from the user accounts alone would omit exactly the actors an operator most often needs to
look up.

#### Scenario: Listing the actors of a tenant

- **WHEN** a signed-in caller lists actors
- **THEN** the actors of that caller's tenant are returned, agents among them

#### Scenario: Another tenant's actors are never listed

- **WHEN** a caller lists actors and another tenant holds actors of its own
- **THEN** none of the other tenant's actors appear

#### Scenario: A caller who is not signed in

- **WHEN** an unauthenticated caller lists actors
- **THEN** the request is refused

#### Scenario: A sort the listing cannot serve

- **WHEN** a sort is requested that the stored rows cannot support
- **THEN** the request is refused rather than answered with an order that is not the one asked for

### Requirement: Assigning work offers the actors that exist

A screen that assigns work SHALL offer the actors of the tenant as suggestions, and SHALL still accept an
identifier that is not among them.

The suggestion is a convenience, not a constraint: an actor from another tenant is deliberately assignable
by identifier, and a control that only permitted what it listed would remove that without saying so.

#### Scenario: The assignee field suggests actors

- **WHEN** the task screen is rendered
- **THEN** it offers the handles of this tenant's actors, people and agents alike

#### Scenario: An identifier that is not suggested is still accepted

- **WHEN** an identifier outside the suggestions is submitted
- **THEN** it is accepted as before
