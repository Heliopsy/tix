# task-management Specification

## Purpose

Creates, edits, relates and lists tasks: the work items the rest of the system exists to move.

## Requirements

### Requirement: Task creation

A task SHALL be creatable with a title, and optionally a body, project, priority, assignee, due date, tags, and custom field values. Only a title SHALL be required; every other attribute SHALL take a documented default.

#### Scenario: Create with only a title

- **WHEN** a task is created supplying only a title
- **THEN** the task is created in the default project, at the default priority, in its workflow's initial state, unassigned, with no due date and no tags

#### Scenario: Create with full attributes

- **WHEN** a task is created supplying a title, body, project, priority, assignee, due date, tags, and custom field values
- **THEN** all supplied attributes are stored and returned on the created task

#### Scenario: Empty title

- **WHEN** a task is created with an empty or whitespace-only title
- **THEN** the request is rejected with a validation error and no task is created

#### Scenario: Unknown project

- **WHEN** a task is created naming a project that does not exist in the tenant
- **THEN** the request is rejected with a not-found error and no task is created

### Requirement: Task identifiers and references

Every task SHALL have a stable opaque identifier and a human-readable reference composed of its project key and a per-project sequence number, such as `infra-42`. Both forms SHALL be accepted wherever a task reference is taken, and the reference SHALL be unique within its tenant.

#### Scenario: Reference assigned on create

- **WHEN** a task is created in the project with key `infra`
- **THEN** the task is returned with a stable identifier and a reference of the form `infra-<n>` where `<n>` is the next number in that project's sequence

#### Scenario: Operate by reference

- **WHEN** an operation is addressed to `infra-42`
- **THEN** it acts on the same task as the equivalent operation addressed to that task's stable identifier

#### Scenario: References are not reused

- **WHEN** a task is deleted and a new task is created in the same project
- **THEN** the new task receives a reference that has not previously been used in that project

#### Scenario: Unknown reference

- **WHEN** an operation is addressed to a reference that matches no task
- **THEN** the request is rejected with a not-found error

### Requirement: Task update

A task's attributes SHALL be updatable individually, and an update SHALL change only the attributes explicitly supplied.

#### Scenario: Partial update

- **WHEN** an update supplies a new assignee only
- **THEN** the assignee changes and the title, body, priority, due date, tags, and custom fields are unchanged

#### Scenario: Clearing an optional attribute

- **WHEN** an update explicitly clears the due date
- **THEN** the task is returned with no due date

#### Scenario: Invalid value in an update

- **WHEN** an update supplies a value that fails validation for its field
- **THEN** the whole update is rejected with a validation error and no attribute of the task is changed

### Requirement: Task deletion

A task SHALL support soft deletion, after which it is excluded from ordinary reads and lists but remains recoverable, and hard deletion, which removes it permanently.

#### Scenario: Soft delete

- **WHEN** a task is soft deleted
- **THEN** it no longer appears in list results and reading it without requesting deleted records reports it as not found

#### Scenario: Restore a soft-deleted task

- **WHEN** a soft-deleted task is restored
- **THEN** it appears in list results again with its previous attributes intact

#### Scenario: Hard delete

- **WHEN** a task is hard deleted
- **THEN** it is removed permanently and cannot be restored

#### Scenario: Delete a task with subtasks

- **WHEN** deletion is requested for a task that has child tasks
- **THEN** the request is rejected unless it explicitly requests that the children be deleted or detached, and the outcome is reported

### Requirement: Subtasks

A task MAY declare a parent task, expressing containment. A task SHALL have at most one parent, and a parent relationship SHALL NOT form a cycle.

#### Scenario: Attach a subtask

- **WHEN** a task is created or updated with a parent reference
- **THEN** the task is reported as a child of that parent and the parent lists it among its children

#### Scenario: Parent cycle

- **WHEN** a parent relationship is set that would make a task its own ancestor
- **THEN** the request is rejected with a validation error and no relationship is created

#### Scenario: Detach a subtask

- **WHEN** a task's parent reference is cleared
- **THEN** the task becomes a root task and no longer appears among the former parent's children

#### Scenario: Containment does not order work

- **WHEN** a parent task has children that are not in terminal states
- **THEN** the parent is not reported as blocked solely because of those children

### Requirement: Dependencies

A task MAY declare dependencies on other tasks, expressing ordering. Dependencies SHALL be distinct from the parent relationship, and a dependency graph SHALL NOT contain a cycle.

#### Scenario: Add a dependency

- **WHEN** task B is declared to depend on task A
- **THEN** A is listed among B's dependencies and B is listed among A's dependents

#### Scenario: Direct cycle

- **WHEN** a dependency is added that would make a task depend on itself, directly or through other tasks
- **THEN** the request is rejected with a validation error naming the cycle and no dependency is created

#### Scenario: Remove a dependency

- **WHEN** a dependency is removed
- **THEN** it no longer appears on either task and any blocked state caused solely by it is cleared

#### Scenario: Dependency on an unknown task

- **WHEN** a dependency is added naming a task that does not exist
- **THEN** the request is rejected with a not-found error

### Requirement: Blocked reporting

A task whose dependencies are not all in terminal states SHALL be reported as blocked. A task with no dependencies, or whose dependencies are all in terminal states, SHALL be reported as unblocked.

#### Scenario: Blocked by an open dependency

- **WHEN** a task depends on another task that is not in a terminal state
- **THEN** the task is reported as blocked and the unmet dependencies are reported with it

#### Scenario: Unblocked when dependencies complete

- **WHEN** every dependency of a blocked task reaches a terminal state
- **THEN** the task is reported as unblocked without any further action on it

#### Scenario: No dependencies

- **WHEN** a task has no dependencies
- **THEN** the task is reported as unblocked

### Requirement: Tags

Tags SHALL be attachable to and detachable from a task, a task SHALL carry any number of tags, and attaching a tag already present SHALL be idempotent.

#### Scenario: Attach a tag

- **WHEN** a tag is attached to a task
- **THEN** the tag appears on the task and the task appears when filtering by that tag

#### Scenario: Attach a tag twice

- **WHEN** a tag already on a task is attached again
- **THEN** the operation succeeds and the tag appears exactly once on the task

#### Scenario: Detach a tag

- **WHEN** a tag is detached from a task
- **THEN** the tag no longer appears on the task and the task no longer matches a filter on that tag

### Requirement: Comments

A task SHALL support comments, which can be created, edited, and soft deleted. A comment SHALL record its author and its creation time, and an edited comment SHALL record that it was edited.

#### Scenario: Add a comment

- **WHEN** a comment is added to a task
- **THEN** the comment is returned with the task's comment thread, attributed to its author, with its creation time

#### Scenario: Edit a comment

- **WHEN** a comment's body is edited
- **THEN** the new body is returned and the comment is marked as edited

#### Scenario: Soft delete a comment

- **WHEN** a comment is deleted
- **THEN** it is excluded from the task's comment thread and the remaining comments retain their order

#### Scenario: Comment on an unknown task

- **WHEN** a comment is added to a task reference that matches no task
- **THEN** the request is rejected with a not-found error

### Requirement: Artifacts

A task SHALL accept structured artifacts written by workers. An artifact SHALL carry a kind, a name, a JSON payload, and a content type, and MAY carry an inline blob. Artifacts on a task SHALL be listable and individually retrievable.

#### Scenario: Write an artifact

- **WHEN** an artifact is written to a task with a kind, name, JSON payload, and content type
- **THEN** the artifact is stored and appears when the task's artifacts are listed

#### Scenario: Retrieve an artifact payload

- **WHEN** a stored artifact is retrieved
- **THEN** its JSON payload and any inline blob are returned unchanged

#### Scenario: Invalid payload

- **WHEN** an artifact is written whose payload is not valid JSON
- **THEN** the request is rejected with a validation error and no artifact is stored

#### Scenario: Multiple artifacts of the same kind

- **WHEN** several artifacts of the same kind are written to one task
- **THEN** all of them are retained and listed, distinguished by name and creation time

### Requirement: Task filtering

Task lists SHALL be filterable by project, status, assignee, tag, priority, due date, claimed state, blocked state, parent, free-text query, and custom field values. Multiple filters SHALL combine conjunctively.

#### Scenario: Single filter

- **WHEN** tasks are listed filtered by status
- **THEN** only tasks in that status are returned

#### Scenario: Combined filters

- **WHEN** tasks are listed filtered by project, tag, and unblocked state together
- **THEN** only tasks satisfying all three conditions are returned

#### Scenario: Filter by custom field

- **WHEN** tasks are listed filtered by a custom field value
- **THEN** only tasks whose value for that field matches are returned

#### Scenario: Text query

- **WHEN** tasks are listed with a free-text query
- **THEN** tasks whose title or body matches the query are returned

#### Scenario: Text query holding a wildcard character

- **WHEN** tasks are listed with a free-text query containing `%` or `_`
- **THEN** that character is matched as itself rather than as a pattern operator
- **AND** tasks matched only by reading it as a wildcard are not returned

#### Scenario: Unknown filter field

- **WHEN** tasks are listed with a filter naming a field that does not exist
- **THEN** the request is rejected with a validation error naming the unknown field

### Requirement: Negated filter terms

Task lists SHALL be filterable by the absence of a value as well as its presence, for project, status, tag, assignee, creator, lease holder and priority. A negated term SHALL combine conjunctively with every other term, and SHALL take precedence over a selecting term naming the same value.

#### Scenario: Exclude a tag

- **WHEN** tasks are listed excluding the tag `ops`
- **THEN** every returned task lacks that tag, and a task carrying no tags at all is returned

#### Scenario: Exclusion beats selection

- **WHEN** tasks are listed selecting status `todo` and excluding status `todo`
- **THEN** no tasks are returned

#### Scenario: Selection and exclusion combine

- **WHEN** tasks are listed selecting status `todo` and excluding tag `docs`
- **THEN** only tasks in status `todo` that lack that tag are returned

#### Scenario: A null column is not an excluded value

- **WHEN** tasks are listed excluding an assignee
- **THEN** tasks with no assignee are returned, because having no assignee is not being assigned to the excluded actor

#### Scenario: Negating a term that shapes the listing

- **WHEN** tasks are listed with a negated sort, limit, parent or due bound
- **THEN** the request is rejected with a validation error, because those terms select a shape of the listing rather than a set of tasks

### Requirement: Weak text matching

Task lists SHALL support a weak text match, distinct from exact match, on title and body. A weak match SHALL succeed when the value appears anywhere inside the field; an exact match SHALL succeed only when the value is the whole field. Both SHALL be case-insensitive, and both SHALL be answerable on every storage engine with the same result for the same corpus. A weak or exact term SHALL be negatable.

#### Scenario: Weak match is a substring

- **WHEN** tasks are listed with a weak title match on `api`
- **THEN** every task whose title contains `api` anywhere is returned

#### Scenario: Exact match is the whole field

- **WHEN** tasks are listed with an exact title match on `api`
- **THEN** no task whose title merely contains `api` is returned

#### Scenario: Case is ignored

- **WHEN** tasks are listed with a weak title match on `API` and again on `api`
- **THEN** both return the same tasks

#### Scenario: Wildcards in the value are literal

- **WHEN** tasks are listed with a weak title match on a value holding `%` or `_`
- **THEN** those characters match themselves and do not act as pattern wildcards

#### Scenario: Engines agree

- **WHEN** the same weak, exact or negated text term is answered by each supported storage engine over the same corpus
- **THEN** both engines return the same tasks

#### Scenario: Unknown text field

- **WHEN** tasks are listed with a text term naming a field other than title, body or the pair of them
- **THEN** the request is rejected with a validation error

### Requirement: Task sorting

Task lists SHALL be sortable by documented fields including creation time, update time, priority, due date, and status, in ascending or descending order, with a deterministic total order.

#### Scenario: Sort by priority

- **WHEN** tasks are listed sorted by priority descending
- **THEN** tasks are returned in descending priority order

#### Scenario: Default sort is urgency

- **WHEN** tasks are listed with no sort field given
- **THEN** they are returned by priority first, most urgent priority leading, and by due date within a shared priority, soonest deadline leading, with a task carrying no due date sorted after every dated task at that priority as if its deadline were the most distant

#### Scenario: Sort by due date includes undated tasks

- **WHEN** tasks are listed sorted by due date, ascending or descending, over a mix of dated and undated tasks and across every page
- **THEN** each task is returned exactly once, with a task carrying no due date ordered as if its deadline were the most distant, identically on every storage engine

#### Scenario: Deterministic tie-breaking

- **WHEN** several tasks share the same value for the requested sort field
- **THEN** they are returned in a stable, deterministic order that is identical across repeated identical requests

#### Scenario: Unsupported sort field

- **WHEN** a sort is requested on a field that is not sortable
- **THEN** the request is rejected with a validation error

### Requirement: Keyset pagination

Every list result SHALL be paginated by an opaque cursor derived from the sort key and the task identifier. Offset-based pagination SHALL NOT be offered by any list operation.

#### Scenario: First page

- **WHEN** a list is requested with a page size and no cursor
- **THEN** at most that many results are returned together with a cursor for the next page when more results exist

#### Scenario: Subsequent page

- **WHEN** a list is requested with the cursor returned by the previous page
- **THEN** the results continue immediately after the last result of the previous page, with no duplicates and no gaps among unchanged rows

#### Scenario: Last page

- **WHEN** the final page of a list is returned
- **THEN** no next-page cursor is returned

#### Scenario: Offset is not accepted

- **WHEN** a list is requested with an offset parameter
- **THEN** the request is rejected rather than being served by skipping rows

#### Scenario: Cursor from a different query

- **WHEN** a cursor is supplied with filter or sort parameters that differ from those it was produced under
- **THEN** the request is rejected with a validation error

#### Scenario: Paging a compound ordering

- **WHEN** a list is sorted by more than one key, such as the default urgency ordering
- **THEN** the cursor carries every key the ordering uses, plus the task identifier, so paging never skips or repeats a row

### Requirement: Optimistic concurrency

Every task SHALL carry a version that changes on each mutation. An update that supplies a version other than the task's current version SHALL be rejected as a conflict and SHALL NOT overwrite the stored task.

#### Scenario: Stale update rejected

- **WHEN** two updates are prepared from the same version and the first is applied
- **THEN** the second is rejected with a conflict error reporting the current version, and the task retains the result of the first update

#### Scenario: Current version accepted

- **WHEN** an update supplies the task's current version
- **THEN** the update is applied and the task's version changes

#### Scenario: Version omitted

- **WHEN** an update omits the version
- **THEN** the update is applied last-write-wins and the behaviour is documented as an explicit opt-out of conflict detection

### Requirement: Task tree view

A task and its descendants SHALL be retrievable as a tree, reporting each node's depth and its parent, so that containment can be rendered without repeated queries.

#### Scenario: Retrieve a tree

- **WHEN** the tree of a task with nested subtasks is requested
- **THEN** the task and all of its descendants are returned with their parent relationship and depth

#### Scenario: Leaf task

- **WHEN** the tree of a task with no children is requested
- **THEN** only that task is returned

#### Scenario: Depth limit

- **WHEN** a tree is requested with a maximum depth
- **THEN** only nodes within that depth are returned and nodes with omitted children are marked as having more

#### Scenario: Deleted descendants

- **WHEN** the tree of a task with soft-deleted descendants is requested without asking for deleted records
- **THEN** the soft-deleted nodes and their subtrees are omitted

### Requirement: A reader can star a task

The system SHALL let any actor who may read a task star it and unstar it. A star SHALL belong to
the actor who set it, within one tenant, and SHALL NOT be visible in another actor's listings.

Starring a task the actor has already starred, and unstarring a task the actor has not starred,
SHALL succeed and SHALL change nothing.

Starring a task the actor cannot read SHALL fail exactly as reading it would.

#### Scenario: Starring marks the task for the reader

- **GIVEN** a task an actor can read
- **WHEN** the actor stars it and then reads it
- **THEN** the task is reported as starred

#### Scenario: A star is personal

- **GIVEN** two actors of one tenant who can both read a task
- **WHEN** the first actor stars it
- **THEN** the task is reported as starred to the first actor and as not starred to the second

#### Scenario: Starring twice is a no-op

- **GIVEN** a task the actor has starred
- **WHEN** the actor stars it again
- **THEN** the call succeeds, the task stays starred, and no further audit entry or event is written

#### Scenario: Unstarring an unstarred task is a no-op

- **WHEN** an actor unstars a task they have not starred
- **THEN** the call succeeds and no audit entry or event is written

#### Scenario: A viewer can star

- **GIVEN** an actor holding only the task read scope
- **WHEN** the actor stars a task
- **THEN** the star is recorded

#### Scenario: Starring outside the caller's reach

- **GIVEN** a token pinned to one project
- **WHEN** it stars a task in another project
- **THEN** the call is refused as reading that task would be

### Requirement: A star is recorded like every other change

Starring and unstarring SHALL each write the star row, an audit entry and an outbox event in one
transaction. The audit actions SHALL be `task.star` and `task.unstar`.

#### Scenario: A star leaves an audit entry

- **WHEN** an actor stars a task
- **THEN** an audit entry with action `task.star` naming the task and the actor is written in the same transaction as the star

### Requirement: A reader's starred tasks lead every task listing

Every task listing SHALL return the reading actor's starred tasks before their other tasks, and
SHALL keep the requested order, including the default urgency order and an explicit sort in
either direction, inside each of the two groups.

A listing read without a reader, such as an export, SHALL be ordered exactly as if nothing were
starred.

#### Scenario: Starred first under the default order

- **GIVEN** three tasks of descending urgency, of which the reader has starred the least urgent
- **WHEN** the reader lists tasks
- **THEN** the starred task is first and the other two follow in urgency order

#### Scenario: Starred first under an explicit sort

- **GIVEN** starred and unstarred tasks with different deadlines
- **WHEN** the reader lists them sorted by due date, ascending or descending
- **THEN** every starred task precedes every unstarred task, and each group is in the requested due-date order

#### Scenario: Another reader's stars do not reorder the listing

- **GIVEN** a task starred by one actor
- **WHEN** a second actor lists tasks
- **THEN** the task appears where the second actor's order puts it, not first

### Requirement: The starred group pages without gaps or repeats

The starred grouping SHALL be the leading key of the keyset ordering and SHALL be carried in the
cursor, so that walking a listing page by page returns every unchanged task exactly once,
including across the page boundary where the starred group ends.

A cursor issued before stars existed SHALL remain valid and SHALL resume among the unstarred
tasks.

#### Scenario: Walking across the boundary

- **GIVEN** five starred and seven unstarred tasks
- **WHEN** they are listed two at a time, in either direction and under every sort, following each next cursor
- **THEN** each of the twelve tasks is returned exactly once, the five starred ones first

#### Scenario: An old cursor still pages

- **WHEN** a cursor carrying no starred state is presented
- **THEN** the listing resumes among the unstarred tasks after the position the cursor names

### Requirement: A task starred during a walk moves between the groups

A task starred or unstarred while a listing is being walked SHALL move between the groups as a
task whose sort key changed does: the cursor SHALL remain valid, the walk SHALL continue, the
moved task MAY be returned twice or not at all, and no other task SHALL be skipped or repeated.

#### Scenario: Starring during a walk

- **GIVEN** a reader part of the way through the unstarred group
- **WHEN** they star a task they have not reached yet and then request the next page
- **THEN** the next page continues from the cursor, the remaining unchanged tasks are each returned once, and no error is raised

### Requirement: Deleting a task removes its stars

Hard deleting a task SHALL remove every actor's star on it, as SHALL anything that removes the
task with it. Soft deleting a task SHALL keep its stars, so that restoring it restores them.
Removing an actor SHALL remove that actor's stars.

#### Scenario: Hard delete clears stars

- **GIVEN** a task two actors have starred
- **WHEN** the task is hard deleted
- **THEN** no star row for it remains

#### Scenario: Restore brings the star back

- **GIVEN** a starred task that is soft deleted and then restored
- **WHEN** the actor who starred it lists tasks
- **THEN** the task is starred and listed first

### Requirement: An assignee is named by handle or by identifier, never silently ignored

Every operation that takes an assignee reference SHALL accept an actor handle as readily as an actor
identifier, and SHALL resolve the reference to an identifier in the service layer, so that the command
line, the HTTP API and the browser all obey the same rule rather than each transport obeying its own.

#### Scenario: A handle assigns work

- **WHEN** a task is created or edited with an assignee given as a handle
- **THEN** the stored assignee is that actor's identifier, and the task is assigned to the actor the handle
  names

### Requirement: An assignee reference is classified by its shape alone

The reference SHALL be classified by shape alone: a value of exactly the length and alphabet of a
generated identifier is an identifier, and every other value is a handle. Deciding by shape rather than by
asking the directory keeps the decision free of a query, which is what allows a listing to resolve its
filter in a number of lookups proportional to the handles in it rather than to the values in it.

#### Scenario: Shape alone decides between identifier and handle

- **WHEN** an assignee reference is classified
- **THEN** a value of exactly the length and alphabet of a generated identifier, in either case, is an
  identifier, and a value one character short or long, or carrying a character outside that alphabet, is
  a handle

### Requirement: An identifier-shaped assignee is accepted without a lookup

An identifier-shaped reference SHALL be accepted without a lookup, because actor identifiers are globally
unique and an actor of another tenant is deliberately assignable while never resolving locally.

#### Scenario: An identifier assigns work unchanged

- **WHEN** a task is created or edited with an assignee given as an identifier
- **THEN** the identifier is stored as given, without a directory lookup, so an actor of another tenant
  stays assignable

### Requirement: An assignee that resolves to no actor is refused as not found

A reference that is neither identifier-shaped nor a handle of this tenant SHALL be refused as not found,
naming the reference that failed to resolve. It SHALL NOT be passed to storage, where it previously failed
a foreign key and surfaced the constraint text verbatim.

#### Scenario: An unknown assignee is refused rather than stored

- **WHEN** a task is created or edited with an assignee that resolves to no actor and is not
  identifier-shaped
- **THEN** the operation fails as not found, names the unresolved reference, writes no task, and reports no
  storage constraint

### Requirement: Filtering by an unknown assignee fails instead of answering empty

A task listing SHALL resolve every assignee term of its filter, on the inclusion side and on the exclusion
side alike, before the listing is queried.

Resolution SHALL reuse the actor directory rather than introduce a second lookup path, SHALL run inside the
tenant-scoped transaction the listing already opens, and SHALL look up each distinct handle at most once
per listing.

#### Scenario: A handle filters a listing

- **WHEN** a listing is filtered by an assignee given as a handle
- **THEN** it returns that actor's tasks, the same tasks the actor's identifier would have returned

#### Scenario: An excluded assignee is resolved too

- **WHEN** a listing excludes an assignee given as a handle
- **THEN** that actor's tasks are removed from the result, and an unresolvable excluded reference fails the
  listing in the same way as an included one

### Requirement: An unresolvable assignee filter term is not an empty listing

An assignee term that resolves to no actor SHALL fail the listing as not found, naming the term. It SHALL
NOT produce an empty listing with a successful status, because an empty result is indistinguishable from a
correct answer and reads as "this actor has no work" rather than "this is not a reference this filter
understands". A published example used a handle here, so every reader following it received a confidently
wrong answer.

#### Scenario: An unknown assignee fails the listing

- **WHEN** a listing is filtered by an assignee that names no actor
- **THEN** the listing fails as not found and names the unresolved reference, and the caller receives no
  empty success

### Requirement: A filter term that names nothing fails the listing

A task listing SHALL resolve every reference-shaped term of its filter, on the inclusion side and on the
exclusion side alike, before the listing is queried, and a term that names nothing SHALL fail the listing
as not found rather than select no rows.

#### Scenario: An unknown term on the exclusion side fails the listing

- **WHEN** a listing excludes a project key that names no project
- **THEN** the listing fails as not found, as it does when the same key is included

### Requirement: A failed filtered listing writes no document

An empty listing with a successful status is the one answer that cannot be told apart from a correct one:
a reader, or an agent, reads `tix task ls -p nosuchproject` returning `[]` as "that project has no work"
rather than "there is no such project", and acts on it. The assignee term was corrected first; this
requirement extends the rule to the rest of the filter.

A failed listing SHALL write no document: the error is the whole of the answer.

#### Scenario: An unknown project fails the listing

- **WHEN** a listing is filtered by a project key or identifier that names no project
- **THEN** the listing fails as not found, names the unresolved reference, and writes no listing document

### Requirement: Project and parent filter terms that resolve to nothing fail

Which terms qualify is decided per term, because the terms do not name the same kind of thing and a
refusal that takes away a legitimate query is worse than the silence it replaces. The decisions are:

- A **project**, named by key or by identifier, names a row that exists or does not, so an unresolvable one
  SHALL fail the listing.
- A **parent** names one task, so an unresolvable one SHALL fail the listing.

#### Scenario: A parent named by reference

- **WHEN** a listing is filtered by a parent given in the project reference form the interfaces display
- **THEN** it returns that task's children, and a parent reference that names no task fails the listing

### Requirement: Creator and claimant filter terms obey the assignee rule

- An **assignee**, **creator** and **claimant** each name an actor, and SHALL obey the assignee rule above:
  resolved by shape, an identifier passed through, an unresolvable handle refused.
- A **priority** is already refused as invalid when it is outside the range, by the filter's own
  validation, and needs no lookup.

#### Scenario: A creator or claimant named by handle

- **WHEN** a listing is filtered by a creator or a claimant given as a handle
- **THEN** it resolves as an assignee does, and a handle that names no actor fails the listing

### Requirement: Status and tag filter terms are judged by their own requirements

- A **status** SHALL be checked against the union of the states of every workflow the tenant defines, and
  SHALL fail the listing only when no workflow declares it. See the requirement below.
- A **tag** SHALL NOT fail a listing. See the requirement below.

#### Scenario: A declared status and an unapplied tag do not fail a listing

- **WHEN** a listing is filtered by a status only one workflow of the tenant declares, or by a tag no task
  has been given
- **THEN** the listing succeeds rather than failing as not found

### Requirement: Filter references are resolved once per listing in its transaction

Resolution SHALL run inside the tenant-scoped transaction the listing already opens, and SHALL look up
each distinct reference at most once per listing, sharing one cache between the inclusion and the exclusion
lists. The cost of a listing therefore stays proportional to the distinct references its filter names
rather than to the values in it or the rows it returns.

#### Scenario: A reference named on both sides costs one lookup

- **WHEN** a listing names the same project, or the same handle, in its inclusion and its exclusion list
- **THEN** the reference is resolved once for the listing

### Requirement: Project keys and statuses in a filter match in any case

A project key and a status SHALL also be matched without regard to case, because the store matches both
exactly and an upper-case spelling of a name that plainly exists is the same silent empty answer by
another route.

#### Scenario: A project named in any case

- **WHEN** a listing is filtered by a project key that differs from the stored key only in case
- **THEN** it returns that project's tasks, rather than nothing

### Requirement: A status is checked against the tenant's workflows, not the listing's scope

A status term SHALL be accepted when any workflow of the tenant declares it as a state, and SHALL fail the
listing as not found only when no workflow does.

A status is not a row of its own: it is a state a workflow declares, and a tenant's workflows need not
agree, so a status valid in one project may be undefined in another. A status some workflow declares is
therefore a real thing that a given listing may simply not reach, and an empty page there is the truthful
answer.

#### Scenario: A status only one workflow declares

- **WHEN** a listing is filtered by a status that one workflow defines and another does not
- **THEN** the listing succeeds, returning the tasks in that state, and returns an empty page when the
  listing is scoped to a project whose workflow lacks the state

#### Scenario: A status named in any case

- **WHEN** a listing is filtered by a status that differs from the declared state only in case
- **THEN** it returns the tasks in that state, rather than nothing

### Requirement: A status no workflow declares fails naming the vocabulary it was checked against

The scope of the check is the point of the requirement. Checking a term against only the workflow of the
project in scope would refuse a query somebody legitimately meant, because a listing spanning several
projects, or the whole tenant, may reasonably name a status only one workflow defines and should answer
with that workflow's tasks.

A status no workflow declares can describe no task anywhere and is a typo. The failure SHALL name the term
and the vocabulary it was checked against.

#### Scenario: A status no workflow declares

- **WHEN** a listing is filtered by a status that no workflow of the tenant defines
- **THEN** the listing fails as not found, naming the term and the statuses that are defined

### Requirement: An unknown tag is a tag with no tasks, not an error

A listing filtered by a tag nothing carries SHALL return an empty page with a successful status.

A tag is free-form and comes into being by being applied, so there is no declaration against which a tag
could be called unknown: the only evidence that a tag exists is that some task carries it, which makes "no
such tag" and "a tag with no tasks" the same state.

#### Scenario: Filtering by a tag nothing carries

- **WHEN** a listing is filtered by a tag no task has been given
- **THEN** it returns an empty page and a successful status, rather than failing

### Requirement: Neither including nor excluding an absent tag fails a listing

A tag term SHALL NOT fail a listing, on either the inclusion or the exclusion side.

Refusing "no such tag" would refuse "a tag with no tasks", so removing the last task from a tag would turn
a working filter into a failure, a filter written before the tag is first applied would fail rather than wait, and
excluding a tag, whose whole purpose is that the result carries none of it, would fail for naming a tag
that is absent.

#### Scenario: Excluding a tag nothing carries

- **WHEN** a listing excludes a tag no task has been given
- **THEN** it returns every task the other terms selected, rather than failing

### Requirement: The tag term is the deliberate exception to failing on an unknown term

The tag term SHALL be the one term of the family where an empty page is the honest answer rather than the
defect, and it is recorded here so that a later reading of the rule does not make the vocabulary closed by
tidiness.

#### Scenario: Only the tag term answers an unknown value with an empty page

- **WHEN** a listing is filtered by a tag no task has been given, and another listing by a project that
  names no project
- **THEN** the first returns an empty page with a successful status and the second fails as not found

### Requirement: Transitions honour an expected version

A transition or a route that names a version SHALL be refused as a conflict when the task is at
another version, and SHALL NOT move the task. A route checks the version before its first hop. A
transition that names no version SHALL apply to whatever version is current.

#### Scenario: Stale version on a transition

- **WHEN** a transition names version 3 and the task is at version 4
- **THEN** the transition is refused with a conflict and the task keeps its status

#### Scenario: Stale version on a route

- **WHEN** a route of two hops names a version the task is no longer at
- **THEN** no hop is applied and the route is refused with a conflict

### Requirement: Deleting a project decides what happens to its tasks

Deleting a project that holds tasks, live or soft deleted, SHALL require the caller to choose between moving the
tasks to another project and deleting them with the project. A deletion of such a project that makes neither choice
SHALL be refused as a failed precondition naming the number of tasks, and SHALL write nothing. A project holding
no tasks SHALL delete without a choice.

#### Scenario: A project with tasks and no choice is refused

- **GIVEN** a project holding three tasks
- **WHEN** it is deleted without a choice
- **THEN** the deletion fails as a precondition naming three tasks
- **AND** the project and its tasks are unchanged

#### Scenario: An empty project needs no choice

- **WHEN** a project holding no tasks is deleted without a choice
- **THEN** the project is removed with one audit entry and one event

### Requirement: A project deletion refuses both choices at once and an unusable target

Making both choices, or naming the project itself as the target, SHALL be refused as invalid input. Naming an
archived project as the target SHALL be refused as a failed precondition.

#### Scenario: Both choices at once are invalid

- **WHEN** a deletion names a target project and also asks to delete the tasks
- **THEN** it is refused as invalid input and nothing is written

### Requirement: Deleting a project with its tasks removes and records every task permanently

Deleting a project SHALL remain permanent. When the tasks are deleted with it, every one of them, soft deleted
tasks included, SHALL be removed permanently together with its comments, attachments, tags, dependencies and stars,
and each SHALL be recorded with its own `task.delete` audit entry and `task.deleted` event marked hard, in the same
transaction as the project's own audit entry and event.

#### Scenario: Deleting the tasks with the project records each one

- **GIVEN** a project holding two live tasks and one soft deleted task
- **WHEN** it is deleted with its tasks
- **THEN** the project and all three tasks are gone
- **AND** three `task.deleted` events marked hard and one `project.deleted` event were written in one transaction

### Requirement: Moving or deleting a project's tasks needs the project deletion authority

Moving and deleting with the project SHALL require the authority project deletion requires. Moving SHALL in
addition require the authority to update tasks in the target project.

#### Scenario: Moving needs task authority on the target

- **GIVEN** a caller holding project write but not task write on the target project
- **WHEN** the caller deletes a project moving its tasks to that target
- **THEN** the deletion is refused as permission denied and nothing is written

### Requirement: Moving a deleted project's tasks keeps everything that belongs to them

Moving the tasks of a deleted project SHALL give each task, in its original sequence order, the next number the
target project's own counter issues, and SHALL keep the task's identifier, so that its history, comments,
attachments, tags, dependencies, stars, parent, repeat rule, lease and soft deletion are kept unchanged. A tag scoped
to the deleted project that a moved task carries SHALL be re-created under the target project with the same name
and colour and stay on the task.

#### Scenario: Tasks are renumbered in the target

- **GIVEN** a target project whose counter has issued 4
- **WHEN** a project holding tasks numbered 1 and 2 is deleted moving them to the target
- **THEN** they are numbered 5 and 6 in the target, in that order

#### Scenario: A moved task keeps its comments, tags, stars and dependencies

- **GIVEN** a task with a comment, a tag, a star and a dependency on a task in a third project
- **WHEN** its project is deleted moving it elsewhere
- **THEN** the task under its new reference still has the comment, the tag, the star and the dependency

#### Scenario: A soft deleted task stays restorable

- **WHEN** a project holding a soft deleted task is deleted moving its tasks
- **THEN** the task is soft deleted in the target and restoring it under its new reference succeeds

### Requirement: A moved task's status is mapped onto the target project's workflow

Each moved task's status SHALL be mapped onto the target project's workflow: to the same state when the target
workflow defines it, otherwise to the first state of the target workflow, in declared order, of the same category as
the task's state, otherwise to the target workflow's initial state.

#### Scenario: A state the target lacks maps by category

- **GIVEN** a task in `review`, a state of category in progress, and a target workflow without `review` whose first
  in progress state is `doing`
- **WHEN** its project is deleted moving it to that target
- **THEN** the task is in `doing`

### Requirement: Each task moved off a deleted project is audited with its old and new place

Each moved task SHALL be recorded with a `task.move` audit entry and a `task.updated` event naming its previous and
new reference, project and status, in the same transaction as the project's deletion.

#### Scenario: Each move is recorded

- **WHEN** a project holding two tasks is deleted moving them to another project
- **THEN** two `task.updated` events name the deleted project as the previous project and the target as the new one

### Requirement: A project deletion can be planned before it is made

The service SHALL report, without writing, what deleting a project would do: the number of live and of soft deleted
tasks it holds, the target named, and for a move the status mapping with the number of tasks in each mapped status.
The plan SHALL apply the same validation of the choice and the target as the deletion and SHALL require the authority
to read projects, and the authority to update tasks in the target when one is named.

#### Scenario: The plan states the mapping

- **GIVEN** a project with two tasks in `todo` and one in `review`, and a target workflow without `review`
- **WHEN** the deletion is planned moving to that target
- **THEN** the plan states `todo` to `todo` for two tasks and `review` to the mapped state for one task
- **AND** nothing is written

### Requirement: Copying a task

The service SHALL copy a task into any project of the same tenant the caller may create tasks
in, the source's own project included. The copy SHALL be a new task with a fresh reference in
the target project. It SHALL carry the source's title unless the caller supplies another, and
the source's description, priority, tags, assignee, due date and repeat rule. The copy
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

#### Scenario: Nothing personal travels

- **WHEN** OPS-3 is starred and claimed by the caller and is copied
- **THEN** the copy is neither starred nor claimed and has no artifacts

### Requirement: A copy carries only the custom fields its target project accepts

A copy SHALL carry only the custom field values the target project defines and accepts, then
apply the target's defaults, and SHALL refuse the copy when a field the target requires is still
missing.

#### Scenario: Fields the target does not define

- **WHEN** OPS-3 carries the custom field `env` and project WEB defines no `env`
- **THEN** the copy in WEB carries no `env` value and the copy succeeds

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
when asked, recursively, each subtask copied under the copy of its parent. A copy that stays in
the source's project SHALL keep the source's parent; a copy into another project SHALL be a root
task.

#### Scenario: With comments

- **WHEN** OPS-3 has a comment "looks good" by `ana` and is copied with comments
- **THEN** the copy has one comment by the caller whose body names `ana`, the original time and "looks good"

#### Scenario: Without comments

- **WHEN** OPS-3 has comments and is copied without asking for them
- **THEN** the copy has no comments

#### Scenario: With subtasks

- **WHEN** OPS-3 has subtask OPS-4, which has subtask OPS-5, and OPS-3 is copied to WEB with subtasks
- **THEN** three tasks are created in WEB and the copy of OPS-5 is a subtask of the copy of OPS-4, which is a subtask of the copy of OPS-3

### Requirement: A copy keeps dependencies and records its source only when asked

A copy SHALL keep the source's dependencies only when asked, pointing an edge whose far end was
copied in the same operation at that copy and skipping an edge the caller may not create. When
asked to link, each copy SHALL record the task it was copied from.

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

### Requirement: Bulk task actions

The service SHALL offer `BulkTasks` taking an action, a list of task references and the
action's argument, and SHALL apply the action to each referenced task through the same rules as
the single-task operation: move to a status (along the shortest route of the task's own
workflow), set the priority, add a tag, remove a tag, set or clear the assignee, star, unstar,
copy to a project, soft delete and restore.

#### Scenario: Adding a tag to several tasks

- **WHEN** a caller adds the tag `ops` to OPS-1 and OPS-2 in one bulk action
- **THEN** both tasks carry the tag `ops`

### Requirement: Each task in a bulk action is changed and reported on its own

Each task SHALL be changed in its own transaction with its own audit entry and event. The answer
SHALL carry one outcome per distinct reference, in the order given, saying whether it was applied
and, when refused, the error code and message; a copy's outcome SHALL name the new task. A refusal
for one task SHALL NOT undo or prevent the change to another.

#### Scenario: Partial refusal

- **WHEN** a caller moves OPS-1 and OPS-2 to `done` and OPS-2's workflow has no route to `done`
- **THEN** OPS-1 is `done`, OPS-2 is unchanged, and the outcomes report OPS-1 applied and OPS-2 refused with the reason

#### Scenario: One audit entry per task

- **WHEN** a caller sets the priority of three tasks in one bulk action
- **THEN** the audit log holds a `task.update` entry for each of the three tasks and three `task.updated` events are emitted

### Requirement: Bulk action limits and authority

A bulk action SHALL name a known action, between 1 and 500 references and the argument the
action needs, and SHALL be refused as invalid otherwise. A reference named twice SHALL be acted
on once. Before acting on any task, the service SHALL check the caller holds the scope the
action needs and SHALL refuse the whole request when it does not.

#### Scenario: Too many references

- **WHEN** a caller sends a bulk action naming 501 references
- **THEN** the request is refused as invalid and no task changes

#### Scenario: Missing scope

- **WHEN** a caller holding only `task:read` asks for a bulk delete
- **THEN** the request is refused as forbidden and no task changes

### Requirement: Quick-add syntax

The system SHALL provide one parser, shared by every surface that offers it, that reads a
single line into a task title and the attributes it names: priority (`!highest`, `!high`,
`!normal`, `!low`, `!lowest`, `!1` to `!5`, `priority:X`), tags (`#tag`, `tag:X`), assignee
(`@handle`, `assignee:X`), due date (`due:X`), project (`+key`, `project:key`), parent
(`^ref`, `parent:ref`) and initial status (`status:X`).

#### Scenario: A line with every attribute

- **WHEN** the line `Fix login !high #auth @raj due:2026-11-20 +web ^infra-3 status:doing` is parsed
- **THEN** the title is `Fix login`, the priority is high, the tags are `auth`, the assignee is `raj`, the due date is 2026-11-20, the project is `web`, the parent is `infra-3` and the status is `doing`

### Requirement: Quick-add keeps the words it does not recognise as the title

Recognised tokens SHALL be removed from the title and the remaining words SHALL form the title in
their original order.

#### Scenario: Unicode titles

- **WHEN** the line `Поправи входа #сигурност !high` is parsed
- **THEN** the title is `Поправи входа` and the tag is `сигурност`

### Requirement: Quick-add tokens only count as whole valid words

A sigil SHALL only start a token at the beginning of a whitespace-separated word, and only
when the rest of the word is a valid value for it. A word that does not qualify SHALL stay in
the title unchanged. A keyed form (`due:`, `project:`, `status:`, `priority:`, `tag:`,
`assignee:`, `parent:`) with an invalid value SHALL be refused.

#### Scenario: Look-alikes stay in the title

- **WHEN** the line `Mail a@b.c about C# and #42 see https://x.io/#top +1 !important` is parsed
- **THEN** the title is the whole line and no attribute is set

#### Scenario: A bad keyed value is refused

- **WHEN** the line `Ship it due:someday` is parsed
- **THEN** the parse is refused with a message naming `someday`

### Requirement: Quick-add escapes

A backslash before a token SHALL keep the token as literal title text without the backslash.
Text inside a pair of quotes (`"…"`, `„…“`, `“…”` or `«…»`) SHALL be kept in the title as
written, quotes included. A backslash before anything that is not a token SHALL be left as it
is.

#### Scenario: Backslash

- **WHEN** the line `Tag posts \#news` is parsed
- **THEN** the title is `Tag posts #news` and no tag is set

#### Scenario: Quotes

- **WHEN** the line `Rename "#main @ops" to trunk` is parsed
- **THEN** the title is `Rename "#main @ops" to trunk` and no tag or assignee is set

### Requirement: Quick-add due dates

`due:` SHALL accept `today`, `tomorrow`, a weekday name in full or as three letters (the next
such day after today), `+Nd` and `+Nw` counted from today, and the absolute forms the filter
language accepts. Calendar words SHALL be resolved against a present the caller supplies, in
that present's time zone, and a date without a time SHALL mean midnight UTC of that day.

#### Scenario: Next Friday

- **WHEN** `due:fri` is parsed with a present of Friday 2026-11-06
- **THEN** the due date is 2026-11-13

#### Scenario: Relative days

- **WHEN** `due:+3d` is parsed with a present of 2026-11-01
- **THEN** the due date is 2026-11-04

### Requirement: Quick-add contradictions are refused

A line that names the same single-valued attribute twice with different values, or that
contradicts a value the caller set explicitly alongside it, SHALL be refused. Tags SHALL be
combined. A line with nothing left for the title SHALL be refused.

#### Scenario: Two priorities

- **WHEN** the line `Fix it !high !low` is parsed
- **THEN** the parse is refused naming both values

#### Scenario: Only tokens

- **WHEN** the line `#auth !high` is parsed
- **THEN** the parse is refused because the title is empty

### Requirement: A task can be snoozed until a moment in the future

The service SHALL let an actor with task write snooze a live task until an instant after the present.
Snoozing SHALL record the instant and the snoozing actor on the task, write a `task.snooze` audit entry
naming that actor with the previous and new wake times, and emit `task.snoozed`, all in one
transaction. Snoozing an already snoozed task SHALL replace its wake time. A snooze SHALL be visible to
every reader of the task, not only to the actor who set it.

#### Scenario: Snoozing hides the task from everyone

- **GIVEN** alice and bob both read project OPS
- **WHEN** alice snoozes OPS-3 until tomorrow
- **THEN** OPS-3 is absent from bob's default listing and the audit entry for it names alice

### Requirement: A snooze needs a future wake time and a live task

A wake time that is not after the present SHALL be refused as invalid, and a deleted task SHALL be not
found.

#### Scenario: A wake time in the past is refused

- **WHEN** an actor snoozes OPS-3 until an hour ago
- **THEN** the call fails as invalid and nothing is written

### Requirement: A snoozed task can be unsnoozed

The service SHALL let an actor with task write unsnooze a task, clearing its wake time and
snoozing actor, writing `task.unsnooze` and emitting `task.unsnoozed` in one transaction.
Unsnoozing a task that is not snoozed SHALL change nothing and write nothing.

#### Scenario: Unsnoozing brings the task back at once

- **GIVEN** OPS-3 is snoozed until next week
- **WHEN** an actor unsnoozes it
- **THEN** OPS-3 is in the default listing again

### Requirement: A snoozed task wakes by itself

A task SHALL count as snoozed only while its wake time is after the present, judged at the moment
of each read, so it returns to every listing when the time passes without any process running. A
wake pass SHALL clear every wake time that has passed, recording `task.wake` and emitting
`task.woke` for each in one transaction, and SHALL skip a task whose wake time changed since the
pass read it. A running server SHALL run a wake pass on the same interval as its lease sweep.

#### Scenario: Time passing is enough

- **GIVEN** OPS-3 is snoozed until 10:00 and no server is running
- **WHEN** a listing is read at 10:01
- **THEN** OPS-3 is in it

#### Scenario: The wake pass announces the task

- **GIVEN** OPS-3 was snoozed until 10:00
- **WHEN** a wake pass runs at 10:01
- **THEN** OPS-3 no longer carries a wake time and `task.woke` was emitted for it

### Requirement: Reading one task shows its snooze

Reading a task by reference SHALL return it whether or not it is snoozed, with its wake time and
the actor who snoozed it while the snooze is in force.

#### Scenario: Show a snoozed task

- **GIVEN** OPS-3 is snoozed
- **WHEN** an actor shows OPS-3
- **THEN** the task is returned with `snoozed_until` and `snoozed_by_actor_id`

### Requirement: Save a task as a template

The service SHALL save a live task as a named template holding its title, description,
priority, tags, custom fields and repeat rule and, when asked, its live subtasks recursively.
The template SHALL belong to the task's project unless the caller asks for a tenant-wide
template. A template name SHALL be unique in the tenant regardless of case.

#### Scenario: Save with subtasks

- **WHEN** a member saves OPS-3, which has two subtasks, as "release" with subtasks
- **THEN** template "release" belongs to OPS and holds OPS-3's title, tags and fields and two subtasks

#### Scenario: Duplicate name

- **WHEN** a template "Release" exists and a member saves another task as "release"
- **THEN** the save is refused as a conflict and nothing is written

### Requirement: Templates are not tasks

Templates SHALL NOT appear in task listings, task trees, claims, filters, statistics or
snapshots.

#### Scenario: Listing after save

- **WHEN** a project holds three tasks and one of them is saved as a template
- **THEN** listing the project's tasks returns three tasks

### Requirement: Create a task from a template

The service SHALL create a task, and a subtask for every subtask the template holds, from a template in
one transaction. The caller MAY replace the title and give a due offset that sets the due date relative
to now; otherwise the template's own offset applies. Status, custom fields and tags SHALL follow the
rules a copy into that project follows. The template's use counter SHALL go up by one.

#### Scenario: Use with an offset

- **WHEN** a member uses template "release" with a due offset of 3d
- **THEN** a new OPS task with the template's subtasks exists, due three days from now, in the initial state

### Requirement: A task made from a template goes to the template's project or the one named

The task SHALL go to the template's project or, for a tenant-wide template, the project the caller
names.

#### Scenario: Tenant-wide template needs a project

- **WHEN** a tenant whose projects were all named by hand holds two and a member uses a tenant-wide template naming no project
- **THEN** the use is refused naming the project field

### Requirement: Title placeholders

Every title created from a template SHALL have its placeholders replaced, regardless of case:
DATE in braces by today's date as YYYY-MM-DD, N in braces by the use number and PROJECT in
braces by the target project's key. Any other braced word SHALL stay as written.

#### Scenario: Numbered standup

- **WHEN** template "standup" with title "Standup {N} on {DATE}" has been used four times and is used again on 2026-11-01
- **THEN** the new task's title is "Standup 5 on 2026-11-01"

### Requirement: Manage templates

The service SHALL list the templates a caller can read, keyset-paginated by name, optionally
narrowed to one project and the tenant-wide ones; show one by name or identifier; rename it,
change its scope, due offset, title, description, priority, tags, custom fields and repeat rule;
and delete it.

#### Scenario: Rename

- **WHEN** a member renames template "release" to "ship"
- **THEN** showing "ship" returns the template and showing "release" is not found

#### Scenario: Pinned token and tenant-wide templates

- **WHEN** a token pinned to OPS tries to delete a tenant-wide template
- **THEN** the call is refused as forbidden

### Requirement: Deletes obey the tenant's trash

With the tenant's trash off, deleting a task SHALL remove it permanently whatever the caller asked,
restoring a task SHALL be refused, and listing only deleted tasks SHALL be refused, each refusal a
precondition failure whose detail `reason` is `trash_off`. Listing that merely includes deleted tasks
SHALL still be answered. Any reader SHALL be able to ask whether the trash is on and how many tasks it
holds.

#### Scenario: A delete with the trash off is permanent

- **GIVEN** a tenant whose trash is off
- **WHEN** a task is deleted without asking for a hard delete
- **THEN** the task cannot be listed even with deleted tasks included, and cannot be restored

#### Scenario: Restore and the trash listing are refused

- **GIVEN** a task in the trash and then the trash turned off
- **WHEN** the task is restored, or tasks are listed with `is:trashed`
- **THEN** both are refused as a failed precondition with reason `trash_off`
