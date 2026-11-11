## ADDED Requirements

### Requirement: The CLI reacts to comments

`tix comment react ID REACTION` and `tix comment unreact ID REACTION` SHALL give and take back the caller's
reaction, succeeding without change when it is already in that state, and SHALL print the comment. `tix
comment ls` SHALL show each comment's reactions as emoji and counts. An unknown reaction SHALL exit 2 and an
unknown comment 3.

#### Scenario: Reacting and listing

- **WHEN** `tix comment react ID heart` and then `tix comment ls REF` are run
- **THEN** the listing shows a heart with count 1 on that comment
