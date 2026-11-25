## ADDED Requirements

### Requirement: CalDAV commands

The CLI SHALL provide `tix dav show`, printing whether CalDAV is enabled, its address and the username to use; `tix dav password list`; `tix dav password create NAME [--collections assigned|projects|all] [--read-only] [--expires DURATION|never]`, printing the password once with the address and username; and `tix dav password revoke ID`.

#### Scenario: Creating an app password from the command line

- **WHEN** a person connected to a server runs `tix dav password create iPhone`
- **THEN** the command prints a password, the CalDAV address and their sign-in email as the username, and `tix dav password list` shows `iPhone` without the password
