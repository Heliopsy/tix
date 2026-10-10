# statistics Specification

## Purpose

Reports counts and durations over a window for a tenant, and optionally one project, in a form a human reads rather than a machine dump.

## Requirements

### Requirement: The breakdown reports every state category

The statistics read SHALL report a count for every category in the vocabulary, in the vocabulary's own
order, including the categories nothing is presently in. A category SHALL NOT be omitted when its count
is zero, because a figure that disappears when it empties reads as a missing figure rather than as a
zero.

The breakdown SHALL be derived from the category each workflow declares for the state a task is in, and
SHALL NOT be derived from the state's name.

#### Scenario: All six categories are reported

- **WHEN** the statistics are read for a tenant with one task in one category
- **THEN** six rows are reported, one per category, in the vocabulary's order
- **AND** the five categories holding nothing are reported as zero

#### Scenario: Cancelled work is not counted as completed work

- **GIVEN** one task finished, one task cancelled and one task untouched, all on the shipped workflow
- **WHEN** the statistics are read
- **THEN** the `done` row counts only the finished task
- **AND** the `cancelled` row counts only the cancelled task
- **AND** the `todo` row counts only the untouched task

#### Scenario: Blocked work is reported apart from work merely waiting to start

- **GIVEN** one task blocked and one task untouched, on the shipped workflow
- **WHEN** the statistics are read
- **THEN** the `blocked` row counts only the blocked task
- **AND** the `todo` row counts only the untouched task

### Requirement: Throughput figures are decided by terminal-ness, not by category

The completed count, the per-day series, the lead times, the leaderboard and the ageing list SHALL all be
derived from the completion timestamp, which follows the state's terminal flag. Changing the category a
state belongs to SHALL NOT change any of them.

A cancelled task is therefore counted as completed by those figures and reported in its own row by the
breakdown, and the documentation SHALL state that the two answer different questions.

#### Scenario: Recategorising cancelled work leaves throughput where it was

- **GIVEN** one task finished and one task cancelled inside the window
- **WHEN** the statistics are read
- **THEN** the completed count is two
- **AND** the `done` row of the breakdown is one

#### Scenario: A cancelled task carries a completion timestamp

- **WHEN** a task is moved into a terminal state carrying the category `cancelled`
- **THEN** it is given a completion timestamp
- **AND** it leaves the set of tasks reported as waiting longest
