## ADDED Requirements

### Requirement: The task screen states the star, the parent and the next occurrence

The task screen SHALL state in words whether the reader has starred the task. A subtask's screen
SHALL link the task it belongs to. A repeating task's screen SHALL state its rule and, when the next
occurrence exists, link it with its due date; when it does not exist yet, SHALL state the date
finishing the task now would give it, computed by the same rule completion applies; and when the
rule has run out, SHALL say that this is the last occurrence. When the task has started or finished,
the screen SHALL say when.

#### Scenario: A weekly task not yet finished

- **GIVEN** an unfinished task due on a Monday that repeats `every week`
- **WHEN** its screen is drawn
- **THEN** it states the rule and that finishing now makes the next one due the following Monday

#### Scenario: A finished occurrence

- **GIVEN** a repeating task that has been finished and created its next occurrence
- **WHEN** its screen is drawn
- **THEN** it links the next occurrence with that occurrence's due date

#### Scenario: The last occurrence

- **GIVEN** a task whose rule has one occurrence left
- **WHEN** its screen is drawn
- **THEN** it says this is the last occurrence

#### Scenario: Starred and a subtask

- **GIVEN** a starred subtask
- **WHEN** its screen is drawn
- **THEN** it says the task is starred and links its parent
