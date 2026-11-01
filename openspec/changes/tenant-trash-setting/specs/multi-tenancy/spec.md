## ADDED Requirements

### Requirement: A tenant chooses whether deleted tasks are kept

Each tenant SHALL carry a trash setting, on by default, that only a tenant admin MAY change. Changing it
SHALL write one audit entry and one event in the same transaction as the change. Turning it off SHALL NOT
remove any task already in the trash, and turning it back on SHALL make those tasks listable and
restorable again.

#### Scenario: An existing tenant keeps its trash

- **GIVEN** a tenant created before the setting existed
- **WHEN** the reader asks whether the trash is on
- **THEN** it is on

#### Scenario: Turning it off keeps what the trash holds

- **GIVEN** a tenant with one task in the trash
- **WHEN** an admin turns the trash off and back on
- **THEN** the task is in the trash and can be restored
