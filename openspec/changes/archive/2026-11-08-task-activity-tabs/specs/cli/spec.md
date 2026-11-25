## ADDED Requirements

### Requirement: A tenant's users list as a table

`tix tenant user ls KEY` SHALL render each user of the tenant as one table row with the columns ID,
EMAIL, NAME, ROLE, GLOBAL ADMIN, CREATED and DISABLED, never as a printed structure.

#### Scenario: Listing another tenant's users

- **GIVEN** a tenant with one viewer named Carol
- **WHEN** a global admin runs `tix tenant user ls acme`
- **THEN** the table's row reads Carol's id, address, name, `viewer`, `no` and when she was created
