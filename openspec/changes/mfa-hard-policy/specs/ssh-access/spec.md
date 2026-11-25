## ADDED Requirements

### Requirement: The SSH session of a gated person offers only enrolment

Under the tenant second factor policy, once its grace has ended, an SSH session of a person
holding no factor SHALL be taken through enrolment in the terminal before anything of the tenant
is drawn, and SHALL end if enrolment is not completed. A command session without a terminal
SHALL be refused naming `ssh -t`. Inside the grace the session SHALL open as before.

#### Scenario: Interactive key session

- **WHEN** an unenrolled person connects with an enrolled key and a terminal under the policy
- **THEN** they are shown the enrolment and asked for a code before the board opens

#### Scenario: Command without a terminal

- **WHEN** the same person runs a hosted command without a terminal
- **THEN** the session is refused naming `ssh -t`
