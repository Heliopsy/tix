## ADDED Requirements

### Requirement: Copying a task

The service SHALL copy a task into any project of the same tenant the caller may create tasks
in, the source's own project included. The copy SHALL be a new task with a fresh reference in
the target project. It SHALL carry the source's title unless the caller supplies another, and
the source's description, priority, tags, assignee, due date and repeat rule. It SHALL carry
only the custom field values the target project defines and accepts, then apply the target's
defaults, and SHALL refuse the copy when a field the target requires is still missing. The copy
SHALL NOT carry stars, claims, leases, history, artifacts or the recurrence successor link.

#### Scenario: Copy to another project

- **WHEN** a caller copies OPS-3, titled "Rotate keys" with tags `sec` and priority high, to project WEB
- **THEN** a new task WEB-n exists with the title "Rotate keys", tags `sec` and priority high, and OPS-3 is unchanged

#### Scenario: Copy within the same project

- **WHEN** a caller copies OPS-3 without naming a project
- **THEN** a new task OPS-m with m greater than every existing sequence number is created in OPS

#### Scenario: Replacement title

- **WHEN** a caller copies OPS-3 with the title "Rotate keys again"
- **THEN** the copy is titled "Rotate keys again"

#### Scenario: Fields the target does not define

- **WHEN** OPS-3 carries the custom field `env` and project WEB defines no `env`
- **THEN** the copy in WEB carries no `env` value and the copy succeeds

#### Scenario: Nothing personal travels

- **WHEN** OPS-3 is starred and claimed by the caller and is copied
- **THEN** the copy is neither starred nor claimed and has no artifacts

### Requirement: Status of a copy

A copy SHALL start in the target workflow's initial state. When the caller asks to keep the
status and the target workflow has a state of the same name, the copy SHALL start in that state
instead, and a copy that starts in a terminal state SHALL carry a completion time.

#### Scenario: Default status

- **WHEN** OPS-3 in state `doing` is copied to WEB whose initial state is `todo`
- **THEN** the copy is in `todo`

#### Scenario: Keep status

- **WHEN** OPS-3 in state `doing` is copied to WEB with keep status and WEB's workflow has `doing`
- **THEN** the copy is in `doing`

#### Scenario: Keep status without a matching state

- **WHEN** OPS-3 in state `review` is copied with keep status to a project whose workflow has no `review`
- **THEN** the copy is in that workflow's initial state

### Requirement: Copy options

A copy SHALL include the source's live comments only when asked, each as a new comment written
by the caller that names the original author and time. It SHALL include the live subtree only
when asked, recursively, each subtask copied under the copy of its parent. It SHALL keep the
source's dependencies only when asked, pointing an edge whose far end was copied in the same
operation at that copy and skipping an edge the caller may not create. When asked to link, each
copy SHALL record the task it was copied from. A copy that stays in the source's project SHALL
keep the source's parent; a copy into another project SHALL be a root task.

#### Scenario: With comments

- **WHEN** OPS-3 has a comment "looks good" by `ana` and is copied with comments
- **THEN** the copy has one comment by the caller whose body names `ana`, the original time and "looks good"

#### Scenario: Without comments

- **WHEN** OPS-3 has comments and is copied without asking for them
- **THEN** the copy has no comments

#### Scenario: With subtasks

- **WHEN** OPS-3 has subtask OPS-4, which has subtask OPS-5, and OPS-3 is copied to WEB with subtasks
- **THEN** three tasks are created in WEB and the copy of OPS-5 is a subtask of the copy of OPS-4, which is a subtask of the copy of OPS-3

#### Scenario: With dependencies

- **WHEN** OPS-3 depends on OPS-1 and is copied with dependencies
- **THEN** the copy depends on OPS-1

#### Scenario: Linked copy

- **WHEN** OPS-3 is copied with link
- **THEN** the copy's `copied_from_id` is the identifier of OPS-3

### Requirement: A copy is one transaction

Copying SHALL write every new task, tag, comment and dependency, a `task.copy` audit entry and
a `task.created` event per new task, and a `comment.add` audit entry and `comment.added` event
per copied comment, in one transaction. A refused copy SHALL leave no new row behind.

#### Scenario: Audit and event

- **WHEN** OPS-3 is copied to WEB
- **THEN** the audit log holds one `task.copy` entry for the copy and the event stream one `task.created` event whose payload carries `copied_from: OPS-3`

#### Scenario: Refused copy writes nothing

- **WHEN** a copy with subtasks is refused because the target requires a custom field the second subtask lacks
- **THEN** no task is created in the target and no audit entry or event is written

### Requirement: Authority to copy

Copying SHALL require read authority on the source task and create authority on the target
project, comment authority on the target when comments are copied, and update authority over
both ends of each copied dependency edge. A caller confined to one project SHALL NOT copy into
another.

#### Scenario: Project-confined caller

- **WHEN** a token confined to OPS copies OPS-3 to WEB
- **THEN** the copy is refused as forbidden and nothing is written
