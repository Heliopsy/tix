## ADDED Requirements

### Requirement: The Single sign-on screen shows and triggers a metadata refresh

A SAML provider with a metadata URL SHALL show on its row when its metadata was last read and how, the error of a failed read, when it is read next, and how many certificates are retiring, and SHALL carry a Refresh metadata button that rereads it.

#### Scenario: Failed refresh shown

- **WHEN** an admin presses Refresh metadata and the IdP answers an error
- **THEN** the row says the read failed with its reason and still counts the provider's certificates
