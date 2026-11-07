## ADDED Requirements

### Requirement: Date preferences are kept with the account

The settings screen SHALL save the date format and timezone on the person's account, offer Inherit for each,
and say what is in use and where it came from. The Tenant screen SHALL offer the tenant default the same way,
its Inherit naming the deployment's value.

#### Scenario: Another browser reads the choice

- **GIVEN** a person who chose `us` in one browser
- **WHEN** they open the settings screen in another browser
- **THEN** `us` is selected there and their timestamps read `us`

### Requirement: Legacy date cookies move onto the account

A request carrying the `tix_time_format` or `tix_timezone` cookie SHALL move each valid value onto the
account when the account holds none for that field, and SHALL expire both cookies.

#### Scenario: The account wins over a cookie

- **GIVEN** an account format of `long` and a cookie format of `us` with a cookie zone of `Asia/Tokyo`
- **WHEN** the browser opens any screen
- **THEN** the account keeps `long`, gains `Asia/Tokyo`, and both cookies are gone

### Requirement: Every timestamp carries its full instant

Every timestamp a screen shows as text SHALL be a `<time>` whose `datetime` is RFC 3339, whose text is in the
reader's style, and whose title, accessible label and focus tip carry the instant with seconds, zone name
and offset, plus the relative form when the text is absolute. It SHALL take focus, so a tap or Tab shows it.

#### Scenario: Activity row

- **WHEN** a reader in `dotted`, `Europe/Sofia` focuses "2 hours ago" in the activity feed
- **THEN** a tip reads like `06.11.2026 14:05:32 EET (UTC+02:00)`

### Requirement: A guard holds every screen to the reader's format

A test SHALL render every screen in the screenshot manifest on a seeded database as a reader in `dotted`
and `Asia/Tokyo`, and fail on a timestamp in another layout, a timestamp written as text outside `<time>`,
or a `<time>` missing part of its tooltip.

#### Scenario: A raw field fails the guard

- **WHEN** a template prints `{{.CreatedAt}}` or `{{compact .CreatedAt}}` as text
- **THEN** the guard fails naming the screen and the value
