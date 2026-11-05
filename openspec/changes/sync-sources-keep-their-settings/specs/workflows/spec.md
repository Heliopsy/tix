## REMOVED Requirements

### Requirement: Indexed custom fields

**Reason**: The flag was stored and displayed, and nothing built an index or planned a query from it.
Filtering on a custom field is unchanged: always allowed, never an error.

**Migration**: Stop sending `indexed`. Input that still carries it is accepted and the key ignored.

## ADDED Requirements

### Requirement: A field definition ignores a legacy indexed key

A field definition SHALL NOT carry an indexed flag. An API body, bundle or snapshot that sends `indexed`
SHALL be accepted with the key ignored.

#### Scenario: An old bundle

- **WHEN** a bundle exported before the flag was removed, with `"indexed": true` on a field, is imported
- **THEN** the field is created and the import succeeds
