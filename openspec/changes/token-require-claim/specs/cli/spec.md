## ADDED Requirements

### Requirement: Token commands carry the claim requirement

`tix token create` SHALL take `--require-claim`, and the token table shown by `tix token create`
and `tix token ls` SHALL carry a CLAIM column reading `required` for a token that requires a claim.

#### Scenario: List a claim-requiring token

- **WHEN** a token minted with `--require-claim` is listed with `tix token ls`
- **THEN** its CLAIM cell reads `required`, and `-o json` carries `"require_claim": true`
