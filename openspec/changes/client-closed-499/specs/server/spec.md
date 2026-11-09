## ADDED Requirements

### Requirement: Abandoned requests are not server errors

When a request's client disconnects before it is answered, the server SHALL record the request with status 499 and a note that the client closed it, at info level, and SHALL NOT log it as a server error. A request that exceeds the server's own request timeout SHALL still be answered and recorded as 504, and a failure while the client is still connected SHALL still be 500.

#### Scenario: Client leaves a long poll

- **WHEN** a client cancels an event poll while the server is waiting for an event
- **THEN** the request record carries status 499 and a client closed note, and no error-level record is written

#### Scenario: Server timeout

- **WHEN** a request exceeds the configured request timeout while its client is still connected
- **THEN** the client receives 504 and the request record carries 504

#### Scenario: Service failure

- **WHEN** a service call fails while the client is still connected
- **THEN** the client receives 500 and the request record carries 500
