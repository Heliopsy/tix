## ADDED Requirements

### Requirement: The terminal interface changes a project's key

The project edit form SHALL offer the key as an attribute, answered by the single-line prompt seeded
with the current key, and SHALL send only the key. After the change the project screen SHALL address
the project by its new key.

#### Scenario: A key is typed into the prompt it opens

- **WHEN** a reader chooses the key attribute
- **THEN** the prompt opens seeded with the project's current key
- **AND** accepting it sends only the key
