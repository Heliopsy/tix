## ADDED Requirements

### Requirement: A claim-requiring token changes only tasks its actor has claimed

A caller authenticated with a token whose `require_claim` is set SHALL be refused a transition, a
route, a field edit, a checklist tick, a tag change, a move to another project or a delete of a task
unless the token's actor holds a live claim on it. Each task of a cascading delete SHALL be judged.

#### Scenario: A move without a claim is refused

- **WHEN** a claim-requiring token moves an unclaimed task `ABC-12` to `done`
- **THEN** it is refused as permission denied with a message naming `tix claim task ABC-12 --start`, and the task is unchanged

#### Scenario: A move under the token's own claim is applied

- **WHEN** a claim-requiring token claims `ABC-12` and moves it to `done` with its lease token
- **THEN** the move is applied and the lease is released

#### Scenario: Another actor's claim does not count

- **WHEN** a claim-requiring token edits a task held under another actor's live claim
- **THEN** the edit is refused as permission denied

### Requirement: The claim requirement leaves claiming and discussion open

A claim-requiring token SHALL be able to claim, claim and start in one step, renew and release its
own claim, create tasks, comment, react and read, without holding a claim first.

#### Scenario: Claim and start in one step

- **WHEN** a claim-requiring token runs `tix claim task ABC-12 --start`
- **THEN** the task is claimed by the token's actor and moved to its in-progress state

#### Scenario: Comment on an unclaimed task

- **WHEN** a claim-requiring token comments on a task nobody holds
- **THEN** the comment is recorded

### Requirement: The claim requirement is judged per task in a bulk action and in a dry run

A bulk action by a claim-requiring token SHALL refuse each unclaimed task as its own outcome and
apply the rest. A dry run of a transition or route SHALL refuse exactly as the real one does.

#### Scenario: A bulk action mixes claimed and unclaimed tasks

- **WHEN** a claim-requiring token raises the priority of one task it holds and one it does not
- **THEN** the held task is changed and the other's outcome is refused as permission denied

#### Scenario: A dry-run move without a claim is refused

- **WHEN** a claim-requiring token runs `tix task mv ABC-12 done --dry-run` on an unclaimed task
- **THEN** it exits 5 with the same message as the real move

### Requirement: Sessions and other tokens are unaffected by the claim requirement

A session, an SSH sign-in, a shell holding the database and a token without `require_claim` SHALL
change tasks exactly as they did before the setting existed.

#### Scenario: A person moves an unclaimed task

- **WHEN** a person signed in to the browser moves an unclaimed task to `done`
- **THEN** the move is applied
