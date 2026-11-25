## ADDED Requirements

### Requirement: Links drawn from a request prefer the public address

On a host that serves several tenants, the addresses the interface draws from a request (the Share
menu, the hand-off dialog, calendar feed addresses, invite links and the Connect screen) SHALL use
`server.public_url` when it is set. On a tenant's own host, and when the key is unset, they SHALL use
the request's scheme and host.

#### Scenario: Reached by an internal name

- **GIVEN** `server.public_url: https://tix.example.com` and a reader on `http://tix.lan:8080`
- **WHEN** the reader copies a task's link from the Share menu
- **THEN** the link is `https://tix.example.com/tasks/REF`

### Requirement: Notification channels link under the public address

A notification channel message SHALL link to the task under the tenant's own domain when it has one
and otherwise under `server.public_url`. With neither it SHALL carry no link, as before.

#### Scenario: A tenant without a domain

- **GIVEN** `server.public_url: https://tix.example.com` and a tenant with no domain
- **WHEN** a mention is delivered to its Slack channel
- **THEN** the message links `https://tix.example.com/tasks/REF`
