## ADDED Requirements

### Requirement: An invite over SSH prints its absolute link

`tix user invite` and `tix user invite reissue` run over SSH SHALL print the invite's absolute link, under
the tenant's own domain or else `server.public_url`, and give it as `url` in JSON, when the server has
`server.public_url`. Without it the command SHALL print the path as before.

#### Scenario: Server with a public address

- **GIVEN** a server with `server.public_url` `https://tix.z09.org`
- **WHEN** an admin runs `ssh tix user invite bob@example.test --role admin -o json`
- **THEN** `url` is `https://tix.z09.org/invite/TOKEN` and no local database hint is printed
