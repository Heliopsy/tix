## ADDED Requirements

### Requirement: A SCIM screen under Manage

The browser SHALL offer a SCIM screen under Manage for a tenant admin, showing whether SCIM is on with a control to switch it, the endpoint URL, the provisioning token's metadata with a control to mint or rotate it, the group mappings with controls to add and remove one, and recent SCIM activity.

#### Scenario: Turn SCIM on in the browser

- **WHEN** an admin turns SCIM on from the SCIM screen
- **THEN** the screen shows the endpoint URL and the new token once
