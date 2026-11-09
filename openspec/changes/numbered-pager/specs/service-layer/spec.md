## ADDED Requirements

### Requirement: A task listing counts its selection

`ListTasks` SHALL answer the number of tasks the filter selects, counted under the same filter, tenant
scope and authorization as the page. The count SHALL stop at 10,000 and say when it did.

#### Scenario: A filtered count

- **GIVEN** a project of 30 tasks, 12 of them tagged ops
- **WHEN** the tasks tagged ops are listed 5 to a page
- **THEN** the page holds 5 tasks and the total is 12

#### Scenario: A project-scoped token counts only its project

- **WHEN** a token scoped to one project lists every task
- **THEN** the total counts only that project's tasks

### Requirement: A task listing starts at a page number

`ListTasks` SHALL accept a page number from 1 and answer that page of the walk, starting it from a
cursor found by a keyset seek over the same ordering, without `OFFSET`. A page number and a cursor
SHALL NOT both be given. A number past the last page SHALL answer the last page and say which.

#### Scenario: Page 3 is the third slice of the walk

- **GIVEN** 12 tasks listed 5 to a page
- **WHEN** page 3 is asked for
- **THEN** it holds the last 2 tasks of the walk, in the walk's order, and its page number is 3

#### Scenario: A page number with a cursor

- **WHEN** a listing names both a page number and a cursor
- **THEN** it is refused as invalid
