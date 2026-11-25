## ADDED Requirements

### Requirement: The people view leaves out removed people

The people view and the global admin view's users of a tenant SHALL NOT list a person removed from that
tenant.

#### Scenario: Opening the view after a removal

- **GIVEN** a person removed from the tenant
- **WHEN** an administrator opens the people view
- **THEN** the person is not listed
