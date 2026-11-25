## ADDED Requirements

### Requirement: Idempotency key window is configurable

The configuration key `retention.idempotency_keys`, environment variable `TIX_RETENTION_IDEMPOTENCY_KEYS`, SHALL set how long a kept idempotency key answers repeats, default `24h`. A negative value SHALL be refused by validation. A change SHALL apply to keys kept after it.

#### Scenario: A shorter window

- **WHEN** an operator sets retention.idempotency_keys to 1h and a key is kept
- **THEN** a repeat two hours later runs as a new request

#### Scenario: A negative window

- **WHEN** an operator sets TIX_RETENTION_IDEMPOTENCY_KEYS to -1h
- **THEN** configuration validation refuses it naming retention.idempotency_keys
