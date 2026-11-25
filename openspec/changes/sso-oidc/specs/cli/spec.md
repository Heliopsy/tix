## ADDED Requirements

### Requirement: Providers are managed under tix tenant sso

`tix tenant sso add KEY`, `ls`, `show KEY`, `edit KEY` and `rm KEY` SHALL manage the tenant's providers. The client secret SHALL be read from standard input or a prompt, never printed back.

#### Scenario: Add reads the secret from stdin

- **WHEN** `tix tenant sso add kc --kind oidc --issuer URL --client-id ID --client-secret -` reads the secret from standard input
- **THEN** the provider is created and the output shows the secret only as set
