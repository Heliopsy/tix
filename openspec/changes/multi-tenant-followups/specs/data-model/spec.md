## ADDED Requirements

### Requirement: Switch tickets are stored hashed and outside tenant scope

A `switch_tickets` table SHALL hold `id`, `user_id`, `target_tenant_id`, `audience`, `token_hash`, `second_factor`, `created_at`, `expires_at` and `used_at`, and SHALL NOT hold the ticket itself. It SHALL be read and written only through the unscoped allow-list, because a ticket is minted in one tenant and redeemed in another.

#### Scenario: Ticket at rest

- **WHEN** a switch ticket has been minted
- **THEN** its row holds a hash of the ticket and no column holds the ticket's value
