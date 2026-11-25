## ADDED Requirements

### Requirement: Tenant session settings only tighten

A tenant administrator SHALL be able to set the tenant's own session lifetime, idle timeout and remember
lifetime, or turn Keep me signed in off. A value longer than the server's, or turning Keep me signed in on where
the server has it off, SHALL be refused with a message naming the server's value. Empty SHALL inherit.

#### Scenario: Longer than the server

- **WHEN** a tenant sets its session lifetime to 24h on a server whose `session.ttl` is 12h
- **THEN** the change is refused with a message naming 12h

#### Scenario: Turned off

- **WHEN** a tenant sets its remember lifetime to off
- **THEN** sign-ins on the tenant are not offered Keep me signed in

### Requirement: Session settings on every surface

The tenant's session settings SHALL be editable from `tix tenant edit --session-ttl --session-idle-timeout
--session-remember-ttl`, Tenant settings in the browser, the terminal tenant screen and `PATCH /v1/tenants`, and
the values in force SHALL be readable from `tix tenant session-policy` and `GET /v1/auth/session-policy`.

#### Scenario: From the command line

- **WHEN** an administrator runs `tix tenant edit acme --session-idle-timeout 30m`
- **THEN** `tix tenant session-policy` reports an idle timeout of 30m
