## ADDED Requirements

### Requirement: The browser sets housekeeping

The Tenant screen SHALL carry a Housekeeping section setting the tenant defaults, and each project's board SHALL
carry a Housekeeping panel setting the overrides as inherit, off or a number of days. Both SHALL say how many
tasks each rule would act on now, and that the rules are paused while the trash is off.

#### Scenario: The count is shown

- **GIVEN** a done rule of 30 days and two tasks done 40 days ago
- **WHEN** an admin opens the Tenant screen
- **THEN** the Housekeeping section says 2 tasks would be moved to the trash now
