## ADDED Requirements

### Requirement: Completing a task animates only the task that was completed

The web interface SHALL animate the completion mark only on the row that was acted on.

A checkbox that is already complete SHALL NOT replay its completion animation when the page is rendered
for any other reason: a load, a filter change, a column change or a navigation. Motion on a screen means
something just happened, and a page that animates fifteen checkboxes in sequence on load says that fifteen
things happened when nothing did.

Everything static about the completed state SHALL remain outside the animation, so that a page rendered
without JavaScript still paints completion marks correctly.

#### Scenario: Ticking a task

- **WHEN** a task is completed from the list
- **THEN** exactly one completion mark animates, on the row that was completed

#### Scenario: Loading a list that already contains completed tasks

- **WHEN** a list containing completed tasks is rendered
- **THEN** no completion mark animates

#### Scenario: Completion marks render without JavaScript

- **WHEN** the page is rendered and no script runs
- **THEN** completed tasks still show their completion mark
