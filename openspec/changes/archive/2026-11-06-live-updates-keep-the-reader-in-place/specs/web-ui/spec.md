## ADDED Requirements

### Requirement: A pressed button is pending at once without hiding behind its spinner

A submit button, or a button that makes its own request, SHALL be marked pending in the frame it
is pressed, with no transition. Its spinner SHALL take the place of its icon with the label still
readable, or of its label when it has no icon, and its width SHALL NOT change. A second press
while pending SHALL submit nothing, and the mark SHALL clear when the request ends.

#### Scenario: Pressing Add

- **WHEN** the reader presses a submit button whose request is slow
- **THEN** in the same frame it is pending, its spinner overlaps no visible text, its width is unchanged, and pressing it again sends nothing

#### Scenario: The busy pointer waits

- **WHEN** a form's request answers within 300ms
- **THEN** the pointer never changes to progress
