## MODIFIED Requirements

### Requirement: Single service contract

A single `Service` interface SHALL describe the complete product surface, and every access path, including the CLI, TUI, HTTP API, WebSocket stream, and web UI, SHALL perform its operations exclusively through that interface. No access path SHALL reach the storage layer directly.

A method of that interface declared as returning a pointer and an error SHALL return a non-nil
pointer whenever it returns a nil error. There is no third answer: an absent record is an error, and
a read that succeeded has a record to return. A caller MAY therefore branch on the error alone.

An access path SHALL NOT be required to defend that guarantee at each of its own call sites. Where a
path consumes such a read, a violation SHALL be reported as an internal fault naming the record that
was missing, rather than reaching a dereference.

#### Scenario: Operation available on every path

- **WHEN** a new operation is added to the service contract
- **THEN** it becomes available to every access path through the same method, with the same arguments and the same result shape

#### Scenario: No bypass of the contract

- **WHEN** any access path attempts an operation not present on the service contract
- **THEN** that operation does not exist in the product and the code does not build

#### Scenario: Contract is transport-neutral

- **WHEN** the service contract is inspected
- **THEN** no method signature refers to HTTP, CLI, terminal, or any other transport concept

#### Scenario: A read that succeeded returns a record

- **WHEN** a service method declared to return a pointer and an error returns a nil error
- **THEN** the pointer it returns is not nil

#### Scenario: An implementation that breaks the guarantee is reported, not dereferenced

- **GIVEN** an implementation of the service contract that returns a nil pointer together with a nil
  error
- **WHEN** a screen that reads a single record through it is served
- **THEN** the request fails with an internal fault that names the record that was missing
- **AND** no dereference of the absent record is attempted
