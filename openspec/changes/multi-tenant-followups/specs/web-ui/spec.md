## RENAMED Requirements

- FROM: `### Requirement: A tenant at its own address is a link`
- TO: `### Requirement: A tenant at its own address is switched to through a ticket`

## MODIFIED Requirements

### Requirement: A tenant at its own address is switched to through a ticket

The switcher SHALL offer a Switch button for every tenant the reader belongs to. For a tenant served at a domain of its own, Switch SHALL lead to that address through a switch ticket, and the target SHALL show a page naming the account and the tenant with a Continue button that does not submit itself.

#### Scenario: Tenant with its own domain

- **WHEN** a person in HO and in `away`, which has its own domain, opens the switcher on HO's shared host and presses Switch on `away`
- **THEN** they reach `away`'s address on a page asking to continue as their account, and are signed in there only after pressing Continue
