## ADDED Requirements

### Requirement: A tenant can turn password sign-in off

A tenant SHALL hold `password_login`, `allowed` by default or `disabled`. While disabled, the password step of a sign-in SHALL be refused for every account except a global admin. Disabling SHALL be refused while the tenant has no enabled provider, and disabling or removing the last enabled provider SHALL be refused while password sign-in is disabled.

#### Scenario: Refused without a provider

- **WHEN** an admin disables password sign-in on a tenant with no enabled provider
- **THEN** the change is refused naming the need for a provider

#### Scenario: Member refused, global admin allowed

- **WHEN** password sign-in is disabled and a member and a global admin each sign in with a correct password
- **THEN** the member is refused and the global admin is signed in

### Requirement: Tokens and keys survive the password policy

Disabling password sign-in SHALL NOT affect API tokens, SSH key sign-in or the SSH listener.

#### Scenario: Token still works

- **WHEN** password sign-in is disabled
- **THEN** an existing API token keeps working
