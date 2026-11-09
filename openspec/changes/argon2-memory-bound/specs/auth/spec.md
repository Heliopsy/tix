## ADDED Requirements

### Requirement: Password hashing runs under a memory bound

The system SHALL run every argon2id derivation, for minting a hash and for checking one, under one process-wide bound on how many run at once. The bound SHALL be the configured `auth.max_concurrent_hashes` when it is positive, and otherwise a quarter of the runtime's soft memory limit divided by the per-hash memory, at least 1 and at most the core count, or 2 when there is no soft limit.

#### Scenario: More sign-ins than slots

- **WHEN** more password checks start than the bound allows
- **THEN** at most the bound derive at once and the rest wait for a slot

#### Scenario: Operator sets the bound

- **WHEN** `auth.max_concurrent_hashes` is 3
- **THEN** three derivations may run at once whatever the memory limit

### Requirement: A saturated hasher refuses rather than admits

A caller that gets no hashing slot before its context ends, or within ten seconds, SHALL receive a `precondition_failed` error with reason `password_hashing_busy`, without a derivation. Sign-in and invite acceptance SHALL return that error rather than an authentication failure, SHALL answer it identically for known and unknown accounts, and SHALL NOT admit the caller.

#### Scenario: Sign-in while busy

- **WHEN** every slot is held and a person signs in, with an address that exists or one that does not
- **THEN** both receive the busy refusal and no session is minted

### Requirement: Unknown addresses cost one derivation

The system SHALL check a sign-in for an address no account holds against a dummy hash at the current parameters, minted without a derivation, so the check costs one derivation, as a wrong password does, and the first sign-in after a start derives once.

#### Scenario: Unknown address

- **WHEN** a person signs in with an address nobody holds
- **THEN** exactly one derivation runs at the current parameters and the answer is invalid credentials

### Requirement: Password hash cost is configurable and only upgrades

The default argon2id cost SHALL be m=64 MiB, t=3, p=4. `auth.argon2_memory_kib`, `auth.argon2_time` and `auth.argon2_threads` SHALL change it for new hashes, and a setting below 7 MiB or below 35 MiB of memory times passes SHALL be refused at start-up. A stored hash weaker than the setting SHALL be rehashed on the next successful sign-in, and a stronger one SHALL be kept.

#### Scenario: Weaker hash upgrades

- **WHEN** an account whose hash is weaker than the setting signs in successfully
- **THEN** the stored hash is replaced by one at the current setting

#### Scenario: Lowered setting keeps stored hashes

- **WHEN** the operator lowers the cost and an account with a 64 MiB hash signs in
- **THEN** the sign-in succeeds and the stored hash is unchanged

#### Scenario: Setting below the floor

- **WHEN** `auth.argon2_memory_kib` is 4096
- **THEN** configuration is refused naming that key
