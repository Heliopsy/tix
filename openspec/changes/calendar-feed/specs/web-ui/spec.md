## ADDED Requirements

### Requirement: Calendar feed settings

The Settings screen SHALL carry a Calendar feed section with an (i) tip. Without a feed it SHALL offer
a filter field and a Create button; with one it SHALL show the filter with a Save button, a Rotate
button and a Revoke button. Creating and rotating SHALL answer a screen showing the URL once with a
Copy button and a link to subscribe, readable and selectable with scripting off.

#### Scenario: Creating a feed in the browser

- **WHEN** a signed-in reader presses Create in the Calendar feed section
- **THEN** the next screen shows the feed URL beside a Copy button, and returning to Settings shows
  the feed as active without the URL
