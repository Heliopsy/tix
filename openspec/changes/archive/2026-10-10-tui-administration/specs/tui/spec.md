## ADDED Requirements

### Requirement: The terminal interface states what the tenant is

The terminal interface SHALL offer a screen that states, for the tenant the session is pinned to, the
tenant's own attributes, the tenants this session can see, the hostnames that resolve to it and the actors
who belong to it.

The screen SHALL read what it states rather than rendering it from what the session already held, so that
what it says is true after an action it performed.

A section the reader's authority does not reach SHALL say why it is not shown, and SHALL NOT cost the reader
the rest of the screen.

A member SHALL be named by the handle a reader recognises wherever the handle can be resolved, and by the
identifier the service stores otherwise, so that a row is never blank.

#### Scenario: The screen names the tenant it read

- **WHEN** a reader opens the tenant screen
- **THEN** it states that tenant's key, name and theme
- **AND** the tenant was read rather than taken from the session's configuration

#### Scenario: The domains and the members are listed

- **WHEN** the tenant screen is open
- **THEN** it lists each hostname that resolves to the tenant
- **AND** lists each member with the role that member holds

#### Scenario: A refused section costs nothing else

- **WHEN** the reader may reach the screen but not read its domains
- **THEN** the domains section says why it is not shown
- **AND** the screen still states the tenant's own attributes

#### Scenario: The switch the screen already offered still works

- **WHEN** a reader with a way to dial another tenant opens the screen
- **THEN** it still takes a tenant key and switches the session to it
- **AND** a session with no way to dial still says so and names the command that can

### Requirement: The tenant screen selects the row an action acts on

The terminal interface SHALL put one cursor over the tenant screen's domains and members together, and each
selected row SHALL carry which of the two it is.

A screen holding no domain and no member SHALL have no selection, and the removal key SHALL say there is
nothing to remove rather than opening a question over nothing.

#### Scenario: The cursor moves through both listings

- **WHEN** a reader moves the selection down past the last domain
- **THEN** the selection lands on the first member

#### Scenario: A screen with nothing to remove refuses the key

- **WHEN** the tenant has no domains and no members
- **THEN** pressing the removal key opens no confirmation
- **AND** says there is nothing on this screen to remove

### Requirement: The terminal interface edits the tenant's attributes

The terminal interface SHALL offer, from the tenant screen, an edit of the tenant's name and of the theme it
presents itself with.

The theme SHALL be answered from the palettes the build carries, together with a value standing for having
none, so that a theme can be cleared by choosing rather than by typing nothing.

The name SHALL be gathered by the single-line input, seeded with the name it would replace.

#### Scenario: The theme is chosen from what the build has

- **WHEN** a reader opens the tenant edit and selects the theme attribute
- **THEN** the values offered are the palettes the build carries and a value meaning none

#### Scenario: The name edit starts from the current name

- **WHEN** a reader opens the tenant edit and selects the name attribute
- **THEN** the single-line input opens carrying the tenant's current name

### Requirement: The terminal interface adds a domain and a member

The terminal interface SHALL offer, from the tenant screen, one key that asks whether a domain or a member is
being added.

A domain SHALL gather its hostname through the single-line input, and SHALL be added carrying no certificate
of its own, because a certificate is a pair of paths on the server that no control here can gather.

A member SHALL gather both of its answers from fixed lists: the actor from the tenant's own directory, read at
the keystroke rather than held for the session, and the role from the roles the service accepts.

A tenant whose directory holds nobody SHALL say so rather than opening a picker with nothing in it.

#### Scenario: A domain is added by hostname

- **WHEN** a reader chooses to add a domain and types a hostname
- **THEN** the domain is added with the hostname that was typed
- **AND** it carries no certificate of its own

#### Scenario: A member is picked from the directory

- **WHEN** a reader chooses to add a member
- **THEN** the actors offered are the ones the directory was read for at that keystroke
- **AND** the role offered is one the service accepts

#### Scenario: An empty directory opens no picker

- **WHEN** the tenant's directory holds nobody
- **THEN** no member form opens
- **AND** the screen says the tenant has nobody to add

### Requirement: Removing a domain or a member names its subject

The terminal interface SHALL hold a removal of a domain or of a membership behind a confirmation that names
the hostname or the handle it will remove.

The confirmation SHALL be answered by the agreement key rather than by the key every other input is accepted
with.

#### Scenario: The question names the hostname

- **WHEN** a reader presses the removal key on a selected domain
- **THEN** the confirmation names that hostname

#### Scenario: The question names the member

- **WHEN** a reader presses the removal key on a selected member
- **THEN** the confirmation names that member's handle

#### Scenario: Declining removes nothing

- **WHEN** a reader dismisses the confirmation
- **THEN** no removal is sent to the service

### Requirement: The tenant screen offers only what the reader's authority reaches

The terminal interface SHALL gate each of the tenant screen's actions on the operation it calls, in the
footer, in the help overlay and at the keystroke, so that none of the three can offer what the other two
refuse.

#### Scenario: A reader without tenant administration is offered none of the actions

- **WHEN** a reader who may not administer the tenant opens the screen
- **THEN** the footer offers none of the edit, add or remove keys
- **AND** pressing one of them opens nothing and sends nothing
