## ADDED Requirements

### Requirement: A history row's channel leads to that token's activity

A task's History SHALL show each row's channel with the token's name when the change was made with a token, and SHALL link it to the activity feed narrowed to that channel and token. It SHALL NOT show a token's secret.

#### Scenario: A change made with a token

- **WHEN** a task's History shows a change made through the API with the token ci-agent
- **THEN** its channel reads "via token ci-agent" and links to the activity feed filtered by channel api and token ci-agent
