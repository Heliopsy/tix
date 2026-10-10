## ADDED Requirements

### Requirement: A workflow key is addressable

A workflow key SHALL start with a letter, SHALL end with a letter or a digit, SHALL contain only letters,
digits, hyphens and underscores, and SHALL be at most 64 characters. This is the shape a project key already
holds, and it is held for the same reason: the key is spent as a path segment in a URL.

A key failing the shape SHALL be refused as invalid input, and the refusal SHALL name the workflow key so an
operator is not left guessing which field was wrong.

The rule SHALL be enforced by the contract rather than by one surface, so the command line, the HTTP API, the
browser editor and a snapshot import all refuse the same keys.

A refused key SHALL leave no workflow stored and SHALL produce no redirect.

#### Scenario: A key carrying a path separator

- **WHEN** a workflow is saved under a key containing a slash, or one made of relative path segments such as `../admin`
- **THEN** the save is refused as invalid input naming the workflow key
- **AND** no workflow is stored and the response carries no redirect location

#### Scenario: A key carrying a query or a fragment

- **WHEN** a workflow is saved under a key containing `?` or `#`
- **THEN** the save is refused as invalid input naming the workflow key

#### Scenario: A usable key is still usable

- **WHEN** a workflow is saved under a key of letters, digits, hyphens or underscores beginning with a letter
- **THEN** the save succeeds and the response redirects to that workflow's own route under the workflows prefix

#### Scenario: Every surface refuses it

- **WHEN** a workflow carrying such a key arrives through a snapshot import rather than through the editor
- **THEN** it is refused for the same reason
