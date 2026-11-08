## ADDED Requirements

### Requirement: Token rotation reaches every surface

The capability registry SHALL bind `token.rotate` to the command line as `tix token rotate`, to HTTP as `POST /api/v1/tokens/{id}/rotate`, to the browser's Tokens screen and to the terminal account view. It SHALL declare no single scope, because the service needs `token:admin` only for another actor's token.

#### Scenario: Every binding is declared

- **WHEN** `token.rotate` is read from the registry
- **THEN** it declares command line, HTTP, web and terminal bindings and no exemption
