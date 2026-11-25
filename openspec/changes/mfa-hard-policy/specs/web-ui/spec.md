## ADDED Requirements

### Requirement: A gated browser session can only enrol

Under the tenant second factor policy a password sign-in by an unenrolled person SHALL mint a
session. A page that reaches a gated operation SHALL redirect a plain GET to `/settings/totp`,
the enrolment screen, and SHALL answer an htmx request or a POST with 403 and an `HX-Redirect` to
it. Enrolling, sign-out and the account log SHALL work in that session.

#### Scenario: Sign in, enrol, work

- **WHEN** an unenrolled person signs in under the policy, opens the task list, enrols from the screen they are sent to and opens the task list again
- **THEN** the first opening redirects to `/settings/totp` and the second shows the tasks

### Requirement: The grace is announced

During the policy grace every page SHALL show an unenrolled reader a banner naming the deadline
and linking to the enrolment screen.

#### Scenario: Banner

- **WHEN** an unenrolled person opens the task list during a grace
- **THEN** the page carries the banner with a link to `/settings/totp`

### Requirement: The Tenant screen sets the policy

The Tenant screen SHALL have a Second factor section with the policy and the grace, saved through
the tenant update.

#### Scenario: Saving the policy

- **WHEN** an enrolled administrator saves the section with Required and 7d
- **THEN** the tenant reads back `required` and `7d`

### Requirement: The Users screen filters people without a factor

The Users screen SHALL show each person's second factor and SHALL offer a filter that lists only
people without one.

#### Scenario: Filter

- **WHEN** an administrator opens `/admin/users?without_totp=1`
- **THEN** only people without a factor are listed
