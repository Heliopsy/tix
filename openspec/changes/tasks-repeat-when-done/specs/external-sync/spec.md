## ADDED Requirements

### Requirement: External sync ignores recurrence

An external sync SHALL NOT create an occurrence when it moves a task into a done state, and SHALL NOT set,
change or clear a task's recurrence rule. A rule set in tix on a synced task SHALL survive later syncs.

#### Scenario: Remote close

- **WHEN** a sync moves a task carrying `every week` to a done state
- **THEN** no new task is created and the rule is unchanged
