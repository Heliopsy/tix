## ADDED Requirements

### Requirement: The terminal interface edits the second factor policy

The tenant editor SHALL offer the second factor policy and its grace as attributes.

#### Scenario: Turning the policy on

- **WHEN** an enrolled administrator picks the second factor attribute and Required in the tenant editor
- **THEN** the tenant's `mfa_policy` becomes `required`

### Requirement: The people view filters people without a factor

The people view SHALL toggle a filter that lists only people without a second factor.

#### Scenario: Toggling

- **WHEN** an administrator presses the filter key in the people view
- **THEN** only people without a factor are listed, and pressing it again lists everybody
