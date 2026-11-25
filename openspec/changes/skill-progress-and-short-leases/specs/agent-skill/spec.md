## ADDED Requirements

### Requirement: The skill has an agent post progress on a bigger task

The skill SHALL give a rule of thumb for a bigger task and SHALL direct an agent working one to comment on the
task with its plan when it starts, at each meaningful milestone and whenever it is blocked, and with a closing
summary of results and links. It SHALL name the command that adds a comment and SHALL say a quick task needs
no progress comments.

#### Scenario: An agent starts a multi-step task

- **WHEN** an agent reads the skill before working a task it has split into subtasks
- **THEN** the skill directs it to post the plan as a comment, then milestones, then a final summary
- **AND** it names `tix comment add` and `claim release --comment`

### Requirement: The skill keeps leases short and renewed

The skill SHALL direct an agent to claim with the default lease or a ttl close to the next stretch of work
rather than the whole job, to renew while it works, and to release as soon as it stops. It SHALL state that a
long lease is how long the task stays locked after its holder dies.

#### Scenario: An agent sizes a claim

- **WHEN** an agent reads the skill's lease material before claiming
- **THEN** it is told not to front-load a long lease and is shown `tix claim renew`
