## MODIFIED Requirements

### Requirement: A binding that reaches less far than the operation records the shortfall

An operation whose binding on one surface is present but narrower than the same operation elsewhere SHALL
record that shortfall against that surface with its reason. A shortfall SHALL name a surface the operation
binds, and SHALL NOT be recorded against a surface the operation is exempted from.

A recorded shortfall SHALL describe the narrowing the code actually performs. Where a narrowing is lifted
or replaced, the record SHALL be rewritten rather than left describing the former behaviour, and any spec
stating the former narrowing as deliberate SHALL be corrected, including an archived one.

#### Scenario: The browser's subtask depth is recorded

- **WHEN** the registry is read
- **THEN** the task tree operation records that the browser asks for one level of depth

#### Scenario: The browser's key list is recorded as self-scoped

- **WHEN** the registry is read
- **THEN** the ssh key list records that the browser lists the signed-in actor's own only

#### Scenario: The browser's token list records the scope that widens it

- **WHEN** the registry is read
- **THEN** the token list records that the browser reaches another actor's tokens only for a reader holding
  the tenant administration scope, where the command line and the API reach them on the token
  administration scope alone

#### Scenario: A lifted narrowing is not left recorded as it was

- **GIVEN** a spec stating that the browser deliberately lists the signed-in actor's own tokens only
- **WHEN** the browser begins listing the tenant's tokens for an administrator
- **THEN** that spec is corrected rather than left contradicting the code
