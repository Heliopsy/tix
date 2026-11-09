## ADDED Requirements

### Requirement: The CLI stars projects

`tix project star KEY...` and `tix project unstar KEY...` SHALL set and clear the caller's own star on
each project, succeeding without change when it is already in that state. `tix project ls --starred`
SHALL list only the caller's starred projects, and `tix project ls` SHALL mark a starred project's key
with `* ` in table output. An unknown project SHALL exit 3.

#### Scenario: Starring and listing

- **WHEN** `tix project star infra` and then `tix project ls --starred` are run
- **THEN** the listing holds INFRA alone, marked with `* `
