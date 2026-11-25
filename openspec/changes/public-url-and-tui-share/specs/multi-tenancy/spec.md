## ADDED Requirements

### Requirement: A tenant on its own domain links under it

A link built for a tenant served on a domain bound to it, not marked shared, SHALL use that domain
with the scheme, port and base path of `server.public_url`, wherever the link is built: a request, a
hosted command, the terminal interface or a channel message.

#### Scenario: Over SSH

- **GIVEN** `server.public_url: https://tix.example.com` and tenant acme bound to
  `tix.acme.example`
- **WHEN** a member of acme runs `ssh HOST task link acme-3`
- **THEN** it prints `https://tix.acme.example/tasks/acme-3`
