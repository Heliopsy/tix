## ADDED Requirements

### Requirement: The token form offers the claim requirement

The browser's issue-a-token form SHALL offer a checkbox that mints the token with `require_claim`,
unticked by default, and the token listing SHALL mark a token that requires a claim.

#### Scenario: Issue a claim-requiring token in the browser

- **WHEN** an admin ticks "Require a claim before changing a task" and issues a token
- **THEN** the token is minted with `require_claim` and its row is marked as requiring a claim
