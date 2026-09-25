## ADDED Requirements

### Requirement: An assignee is named by handle or by identifier, never silently ignored

Every operation that takes an assignee reference SHALL accept an actor handle as readily as an actor
identifier, and SHALL resolve the reference to an identifier in the service layer, so that the command
line, the HTTP API and the browser all obey the same rule rather than each transport obeying its own.

The reference SHALL be classified by shape alone: a value of exactly the length and alphabet of a
generated identifier is an identifier, and every other value is a handle. Deciding by shape rather than by
asking the directory keeps the decision free of a query, which is what allows a listing to resolve its
filter in a number of lookups proportional to the handles in it rather than to the values in it.

An identifier-shaped reference SHALL be accepted without a lookup, because actor identifiers are globally
unique and an actor of another tenant is deliberately assignable while never resolving locally.

A reference that is neither identifier-shaped nor a handle of this tenant SHALL be refused as not found,
naming the reference that failed to resolve. It SHALL NOT be passed to storage, where it previously failed
a foreign key and surfaced the constraint text verbatim.

#### Scenario: A handle assigns work

- **WHEN** a task is created or edited with an assignee given as a handle
- **THEN** the stored assignee is that actor's identifier, and the task is assigned to the actor the handle
  names

#### Scenario: An identifier assigns work unchanged

- **WHEN** a task is created or edited with an assignee given as an identifier
- **THEN** the identifier is stored as given, without a directory lookup, so an actor of another tenant
  stays assignable

#### Scenario: An unknown assignee is refused rather than stored

- **WHEN** a task is created or edited with an assignee that resolves to no actor and is not
  identifier-shaped
- **THEN** the operation fails as not found, names the unresolved reference, writes no task, and reports no
  storage constraint

### Requirement: Filtering by an unknown assignee fails instead of answering empty

A task listing SHALL resolve every assignee term of its filter, on the inclusion side and on the exclusion
side alike, before the listing is queried.

An assignee term that resolves to no actor SHALL fail the listing as not found, naming the term. It SHALL
NOT produce an empty listing with a successful status, because an empty result is indistinguishable from a
correct answer and reads as "this actor has no work" rather than "this is not a reference this filter
understands". A published example used a handle here, so every reader following it received a confidently
wrong answer.

Resolution SHALL reuse the actor directory rather than introduce a second lookup path, SHALL run inside the
tenant-scoped transaction the listing already opens, and SHALL look up each distinct handle at most once
per listing.

#### Scenario: A handle filters a listing

- **WHEN** a listing is filtered by an assignee given as a handle
- **THEN** it returns that actor's tasks, the same tasks the actor's identifier would have returned

#### Scenario: An unknown assignee fails the listing

- **WHEN** a listing is filtered by an assignee that names no actor
- **THEN** the listing fails as not found and names the unresolved reference, and the caller receives no
  empty success

#### Scenario: An excluded assignee is resolved too

- **WHEN** a listing excludes an assignee given as a handle
- **THEN** that actor's tasks are removed from the result, and an unresolvable excluded reference fails the
  listing in the same way as an included one

### Requirement: A filter term that names nothing fails the listing

A task listing SHALL resolve every reference-shaped term of its filter, on the inclusion side and on the
exclusion side alike, before the listing is queried, and a term that names nothing SHALL fail the listing
as not found rather than select no rows.

An empty listing with a successful status is the one answer that cannot be told apart from a correct one:
a reader, or an agent, reads `tix task ls -p nosuchproject` returning `[]` as "that project has no work"
rather than "there is no such project", and acts on it. The assignee term was corrected first; this
requirement extends the rule to the rest of the filter.

Which terms qualify is decided per term, because the terms do not name the same kind of thing and a
refusal that takes away a legitimate query is worse than the silence it replaces. The decisions are:

- A **project**, named by key or by identifier, names a row that exists or does not, so an unresolvable one
  SHALL fail the listing.
- A **status** SHALL be checked against the union of the states of every workflow the tenant defines, and
  SHALL fail the listing only when no workflow declares it. See the requirement below.
- A **parent** names one task, so an unresolvable one SHALL fail the listing.
- An **assignee**, **creator** and **claimant** each name an actor, and SHALL obey the assignee rule above:
  resolved by shape, an identifier passed through, an unresolvable handle refused.
- A **priority** is already refused as invalid when it is outside the range, by the filter's own
  validation, and needs no lookup.
- A **tag** SHALL NOT fail a listing. See the requirement below.

Resolution SHALL run inside the tenant-scoped transaction the listing already opens, and SHALL look up
each distinct reference at most once per listing, sharing one cache between the inclusion and the exclusion
lists. The cost of a listing therefore stays proportional to the distinct references its filter names
rather than to the values in it or the rows it returns.

A project key and a status SHALL also be matched without regard to case, because the store matches both
exactly and an upper-case spelling of a name that plainly exists is the same silent empty answer by
another route.

A failed listing SHALL write no document: the error is the whole of the answer.

#### Scenario: An unknown project fails the listing

- **WHEN** a listing is filtered by a project key or identifier that names no project
- **THEN** the listing fails as not found, names the unresolved reference, and writes no listing document

#### Scenario: A project named in any case

- **WHEN** a listing is filtered by a project key that differs from the stored key only in case
- **THEN** it returns that project's tasks, rather than nothing

#### Scenario: A parent named by reference

- **WHEN** a listing is filtered by a parent given in the project reference form the interfaces display
- **THEN** it returns that task's children, and a parent reference that names no task fails the listing

#### Scenario: A creator or claimant named by handle

- **WHEN** a listing is filtered by a creator or a claimant given as a handle
- **THEN** it resolves as an assignee does, and a handle that names no actor fails the listing

#### Scenario: A reference named on both sides costs one lookup

- **WHEN** a listing names the same project, or the same handle, in its inclusion and its exclusion list
- **THEN** the reference is resolved once for the listing

### Requirement: A status is checked against the tenant's workflows, not the listing's scope

A status term SHALL be accepted when any workflow of the tenant declares it as a state, and SHALL fail the
listing as not found only when no workflow does. The failure SHALL name the term and the vocabulary it was
checked against.

The scope of the check is the point of the requirement. A status is not a row of its own: it is a state a
workflow declares, and a tenant's workflows need not agree, so a status valid in one project may be
undefined in another. Checking a term against only the workflow of the project in scope would refuse a
query somebody legitimately meant, because a listing spanning several projects, or the whole tenant, may
reasonably name a status only one workflow defines and should answer with that workflow's tasks.

A status some workflow declares is therefore a real thing that a given listing may simply not reach, and an
empty page there is the truthful answer. A status no workflow declares can describe no task anywhere and is
a typo.

#### Scenario: A status no workflow declares

- **WHEN** a listing is filtered by a status that no workflow of the tenant defines
- **THEN** the listing fails as not found, naming the term and the statuses that are defined

#### Scenario: A status only one workflow declares

- **WHEN** a listing is filtered by a status that one workflow defines and another does not
- **THEN** the listing succeeds, returning the tasks in that state, and returns an empty page when the
  listing is scoped to a project whose workflow lacks the state

#### Scenario: A status named in any case

- **WHEN** a listing is filtered by a status that differs from the declared state only in case
- **THEN** it returns the tasks in that state, rather than nothing

### Requirement: An unknown tag is a tag with no tasks, not an error

A tag term SHALL NOT fail a listing, on either the inclusion or the exclusion side. A listing filtered by a
tag nothing carries SHALL return an empty page with a successful status.

A tag is free-form and comes into being by being applied, so there is no declaration against which a tag
could be called unknown: the only evidence that a tag exists is that some task carries it, which makes "no
such tag" and "a tag with no tasks" the same state. Refusing the first would refuse the second, so removing
the last task from a tag would turn a working filter into a failure, a filter written before the tag is
first applied would fail rather than wait, and excluding a tag, whose whole purpose is that the result
carries none of it, would fail for naming a tag that is absent.

This is the one term of the family where an empty page is the honest answer rather than the defect, and it
is recorded here so that a later reading of the rule does not make the vocabulary closed by tidiness.

#### Scenario: Filtering by a tag nothing carries

- **WHEN** a listing is filtered by a tag no task has been given
- **THEN** it returns an empty page and a successful status, rather than failing

#### Scenario: Excluding a tag nothing carries

- **WHEN** a listing excludes a tag no task has been given
- **THEN** it returns every task the other terms selected, rather than failing
