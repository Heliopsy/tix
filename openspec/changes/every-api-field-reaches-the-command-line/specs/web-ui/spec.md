## ADDED Requirements

### Requirement: The field form carries the whole definition

The browser's field definition form SHALL offer every part of a definition the service takes: key,
label, type, enum options, required, default, indexed and position. A default SHALL be converted to
the field's declared type before it is saved, and one that does not fit SHALL be refused with the
reason.

#### Scenario: A default saved from the browser

- **WHEN** a field `points` of type `int` is saved from the form with default 3, indexed and position 2
- **THEN** the stored definition carries default 3, is indexed and sits at position 2

#### Scenario: A default that does not fit

- **WHEN** a field of type `int` is saved with default `big`
- **THEN** the form answers 400 and nothing is stored
