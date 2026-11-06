## ADDED Requirements

### Requirement: Password change and invite commands

`tix user create` and `tix user edit` SHALL take `--require-password-change`. `tix user invite
EMAIL [--role R] [--expires D]` SHALL print the link, and `tix user invite ls`, `revoke ID` and
`reissue ID` SHALL manage pending invites. `tix login` on a flagged account SHALL ask for the new
password twice on a terminal, and without one SHALL fail naming the web sign-in page.

#### Scenario: Login with no terminal

- **GIVEN** a flagged account
- **WHEN** `tix login --password -` runs with the password piped in
- **THEN** it fails saying a password change is required and where to make it, and prints no session
