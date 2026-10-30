# agent-skill Specification

## Purpose

Teaches an agent, through the shipped tix skill, to record its own plan as tasks in the store before working it, and states which writes the lease covers so the agent does not have to infer it.

## Requirements

### Requirement: The skill teaches an agent to record its plan before working it

The agent skill SHALL instruct an agent holding an assignment to decompose
that assignment into tix tasks before it begins work, and to claim and work
those tasks one at a time.

It SHALL state why: a plan held only in an agent's context is lost in full
when the process dies, where a plan recorded as tasks costs at most the one
task in flight, because every other task remains in the store for the same or
another agent to claim.

It SHALL distinguish containment from ordering, naming `--parent` for the
first and `--depends-on` for the second, and SHALL say that only the second is
enforced by the queue.

#### Scenario: An agent is given the skill and an assignment

- **WHEN** an agent reads the skill with an assignment it has not started
- **THEN** the skill directs it to record the assignment and its steps as tasks first
- **AND** names the flag that records ordering separately from the flag that records containment

#### Scenario: The durability argument is stated, not assumed

- **WHEN** a reader asks why the plan should be written down
- **THEN** the skill states the unit of loss with and without recorded tasks

### Requirement: The skill states the boundary of the lease

The skill SHALL say that the lease guards transitions and SHALL NOT imply that
it guards every write to a claimed task.

It SHALL state that comment and artifact writes carry no lease token and are
not checked against one, and that an agent whose lease has lapsed can
therefore record a result over the live holder's work.

It SHALL give the defences that exist: confirming the lease is still held
before recording a result, and closing the task through a release, whose
comment and results travel with the guarded transition.

#### Scenario: A reader asks what the lease protects

- **WHEN** a reader reads the skill's lease material
- **THEN** it names transitions as the guarded operations
- **AND** names comments and artifacts as unguarded, with the consequence for a queue of agents

### Requirement: The skill states that lazy expiry frees the task but does not move it

The skill SHALL say that an expired lease makes a task claimable again with no
sweeper, cron job or server running, because expiry is evaluated when the row
is read.

It SHALL also say that the task's status is unchanged until a sweep runs, and
SHALL warn that a worker claiming with a status filter will not see a crashed
task again until then.

It SHALL name the command that reports whether anything is sweeping.

#### Scenario: An agent plans to drain a queue by status

- **WHEN** an agent reads the skill's claiming material
- **THEN** it is warned that a status-scoped claim misses a crashed task
- **AND** is told both remedies: run the sweep, or do not scope by status

### Requirement: The skill's hot path is separable from its reference material

The skill SHALL be organised so that the material an agent runs on every task
is in the file an agent loads, and material needed less often is in files
beside it.

Every file the skill ships SHALL be reachable by a link from the file an agent
loads, because a reference nothing points at is never opened and goes stale
unread while the skill's guards continue to report it sound.

Exactly one version marker SHALL exist across the whole skill, and it SHALL be
in the file an agent loads, since a reference file cannot date the skill.

#### Scenario: A reference file is added without a link

- **WHEN** the skill directory holds a Markdown file that the loaded file does not link to
- **THEN** the skill's guards fail and name the unreachable file

#### Scenario: A version marker moves out of the loaded file

- **WHEN** the one version marker is in a reference file rather than the loaded file
- **THEN** the skill's guards fail

### Requirement: Every guard reads every file the skill ships

The guards over the skill SHALL read the whole skill directory, not one file
within it.

A command or flag named in any file of the skill SHALL be checked against the
real command tree; a failure quoted in any file SHALL be checked against the
messages the binary can print; a default stated in any file SHALL be checked
against the flag's real default; and a command the skill is required to cover
SHALL be satisfied by a mention in any file of it.

Files SHALL be parsed separately rather than joined, so that a fenced block
opened in one file cannot absorb another file's content and hide the commands
in it.

A positional word the skill passes to a command SHALL be checked against that
command's own argument validator, so that a subcommand which does not exist
fails rather than being read as an argument to its parent.

#### Scenario: A reference file names a flag that does not exist

- **WHEN** a file beside the loaded file passes a flag no command registers
- **THEN** the guard fails and names the file it came from

#### Scenario: A reference file states a default that has drifted

- **WHEN** a file beside the loaded file states a default that is not the flag's
- **THEN** the guard fails and reports the real default

#### Scenario: The skill names a subcommand that does not exist

- **WHEN** the skill writes a command whose last word names no subcommand of a command that takes no arguments
- **THEN** the guard fails, rather than resolving to the parent and passing
