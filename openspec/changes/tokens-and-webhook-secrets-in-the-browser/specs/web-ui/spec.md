## ADDED Requirements

### Requirement: A tenant administrator can see and revoke another actor's token in the browser

The API token screen SHALL list every token of the tenant to a reader holding the tenant administration
scope, and SHALL list only the reader's own tokens to any other reader.

Where the listing spans more than the reader's own tokens, each row SHALL say which actor holds it, by that
actor's handle where the directory holds one. A row that is the reader's own SHALL be distinguishable from
a row that is not.

A revocation control SHALL be offered only for a token the reader is shown, and the control for a token
held by another actor SHALL say whose it is before it is used. No control SHALL be offered for a token the
reader may not see.

#### Scenario: A reader without the administration scope sees only their own

- **GIVEN** a reader holding the token administration scope and not the tenant administration scope
- **AND** a token held by another actor of the same tenant
- **WHEN** the token screen is read
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

#### Scenario: The control for somebody else's token says whose it is

- **GIVEN** an administrator reading a token held by another actor
- **WHEN** the revocation disclosure for that row is opened
- **THEN** it says the token is not the reader's own
- **AND** the button naming the token also names its owner

#### Scenario: An administrator revokes another actor's token from the browser

- **GIVEN** an administrator and a live token held by another actor
- **WHEN** the administrator submits that row's revocation
- **THEN** the token is revoked
- **AND** the row comes back marked revoked and offering no revocation control

### Requirement: A generated webhook signing secret is shown once

Where tix generates a signing secret for a delivery endpoint, because the operator registered one without
supplying a secret, the webhook screen SHALL present that value once, immediately after the registration,
and SHALL NOT present it on any later render.

It SHALL be presented in the same one-time secret region the token screen uses for a freshly issued token:
a region of its own rather than the interface's transient confirmation message, readable in full without
being cropped, selectable, and accompanied by a copy control. The value SHALL remain readable in full when
no scripting is available.

A secret the operator supplied SHALL NOT be presented back to them.

The field's own help SHALL state what happens in each case: when tix generates a secret, when the operator
supplies one, and when an existing endpoint is saved with the field left blank.

#### Scenario: A generated secret appears once

- **WHEN** an endpoint is registered with the signing secret field left blank
- **THEN** the webhook screen presents the generated secret in its one-time secret region
- **AND** a copy control points at it

#### Scenario: A generated secret is gone on the next render

- **GIVEN** a generated secret that has been presented once
- **WHEN** the webhook screen is read again
- **THEN** no secret region is rendered
- **AND** the value does not appear on the page

#### Scenario: A supplied secret is not echoed back

- **WHEN** an endpoint is registered with a signing secret the operator supplied
- **THEN** no secret region is rendered
- **AND** the supplied value does not appear on the page
- **AND** the endpoint is listed

#### Scenario: The field's help is true in both cases

- **WHEN** the signing secret field's help is read
- **THEN** it says a generated secret is shown once
- **AND** it says a secret the operator supplies is not shown

## MODIFIED Requirements

### Requirement: One presentation for a value the server will not disclose again

A value the server will never disclose again SHALL be presented through one shared region, used by every
screen that has such a value, rather than through markup copied per screen.

The region SHALL be distinct from the interface's transient confirmation message, SHALL be readable in full
without being cropped, SHALL be selectable, and SHALL carry a copy control driven by the interface's one
generic copy affordance. It SHALL remain readable in full when no scripting is available.

A freshly issued API token and a generated webhook signing secret SHALL both be presented through it.

#### Scenario: The issued value is not presented as a confirmation message

- **WHEN** a token has just been issued
- **THEN** its value is in its own region rather than in the transient confirmation message
- **AND** a copy control points at it

#### Scenario: Both screens use one region

- **WHEN** the token screen and the webhook screen each present a one-time secret
- **THEN** both are rendered by the same shared region
