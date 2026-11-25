## MODIFIED Requirements

### Requirement: Connection rate limiting

The listener SHALL limit connections per source address, allowing a configurable burst and refilling at a
configurable rate, charging one allowance per connection. The memory it uses for this SHALL be bounded. A
connection over the limit SHALL be refused with a message saying so and how many seconds to wait, and a nonzero
exit status, never with an authentication failure, and SHALL reach no key lookup.

#### Scenario: A source exceeds its allowance

- **WHEN** one source address exceeds its burst within the refill window
- **THEN** further connections from that address are refused until the allowance refills

#### Scenario: The client is told why

- **WHEN** a connection is refused by the rate limit
- **THEN** the client sees `tix: too many connections from this address; try again in N seconds` and exit status 1
- **AND** not `Permission denied (publickey)`

#### Scenario: Other sources are unaffected

- **WHEN** one source address is being refused
- **THEN** a different source address connects normally
