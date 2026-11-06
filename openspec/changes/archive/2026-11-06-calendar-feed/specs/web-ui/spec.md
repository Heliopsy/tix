## ADDED Requirements

### Requirement: Calendar feed settings

The Settings screen SHALL carry a Calendar feed section with an (i) tip. Its first row SHALL state
whether the reader has a feed. Without a feed it SHALL say No feed and offer a filter field and a
Create button; with one it SHALL say Active and show the filter with a Save button, a Rotate button and
a Revoke button.

#### Scenario: An empty section says there is no feed

- **WHEN** a signed-in reader with no feed opens Settings
- **THEN** the Calendar feed section's first row says No feed, above the filter field and Create

### Requirement: Calendar feed actions in Settings answer with the URL or a notice

Revoking SHALL return to the Calendar feed section with a notice saying so. Creating and rotating
SHALL answer a screen showing the URL once with a Copy button and a link to subscribe, readable and
selectable with scripting off.

#### Scenario: Creating a feed in the browser

- **WHEN** a signed-in reader presses Create in the Calendar feed section
- **THEN** the next screen shows the feed URL beside a Copy button, and returning to Settings shows
  the feed as active without the URL

#### Scenario: Revoking returns to the section

- **WHEN** a reader presses Revoke in the Calendar feed section
- **THEN** Settings opens at the Calendar feed section with a notice that the feed was revoked, and the
  section says No feed
