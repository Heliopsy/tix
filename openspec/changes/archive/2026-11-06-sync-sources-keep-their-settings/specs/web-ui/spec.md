## ADDED Requirements

### Requirement: The source form carries the settings

The web form that configures a sync source SHALL offer the mapping path, url, project, query, file and page
size, store those filled in, and the Sources table SHALL show each source's stored settings.

#### Scenario: Saving a source with a url

- **WHEN** a jira source is saved from the form with a url
- **THEN** the source stores the url and its row in the Sources table shows it
