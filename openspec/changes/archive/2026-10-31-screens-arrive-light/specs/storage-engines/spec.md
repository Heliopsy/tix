## ADDED Requirements

### Requirement: A page of tasks reaches its tags and dependencies by task

The schema SHALL index `task_tags` and `task_deps` by tenant and then task, and the queries that
load a page's tags and dependencies, the tag filter and the blocked filter SHALL reach those tables
through a seek on the task rather than through a walk of every tag or dependency row in the tenant.

#### Scenario: A tag filter on a tenant with many tagged tasks

- **GIVEN** a tenant whose tasks carry many tags
- **WHEN** a listing is filtered with `tag:ops`
- **THEN** each task considered is checked against its own tag rows only

#### Scenario: Loading the dependencies of a page

- **WHEN** a listing loads the dependencies of the tasks on one page
- **THEN** the dependency rows are found by each task on the page, in task and then dependency order
