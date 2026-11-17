## ADDED Requirements

### Requirement: SCIM administration commands

The CLI SHALL offer `tix tenant scim show`, `enable`, `disable`, `token`, `map-group NAME --role ROLE` and `unmap-group NAME`, each needing `tenant:admin`. `show` SHALL report whether SCIM is on, the endpoint, the provisioning token's metadata, the provisioned user and group counts, the last activity and the group mappings.

#### Scenario: Enable and read the endpoint

- **WHEN** an admin runs `tix tenant scim enable` against a server
- **THEN** it prints the endpoint URL ending in `/scim/v2` and the token, once

#### Scenario: Map a group

- **WHEN** an admin runs `tix tenant scim map-group Engineering --role admin`
- **THEN** `tix tenant scim show` lists `Engineering` mapped to `admin`
